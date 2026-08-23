package scoring_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
	"github.com/stuttgart-things/schmetterpause/internal/match"
	"github.com/stuttgart-things/schmetterpause/internal/repository"
	"github.com/stuttgart-things/schmetterpause/internal/repository/postgres"
	"github.com/stuttgart-things/schmetterpause/internal/scoring"
)

// These tests run against a real database, because what this package has to
// get right is transactional: two ratings, two history rows and a status
// change either all land or none do, and the schema's own constraints are
// part of the contract. A fake store would test the arrangement of the calls
// and none of that.
//
// They run only when SP_TEST_DATABASE_URL is set, and they empty every table:
//
//	task test:integration
const testDSNEnv = "SP_TEST_DATABASE_URL"

func newStore(t *testing.T) (*postgres.Store, context.Context) {
	t.Helper()

	dsn := os.Getenv(testDSNEnv)
	if dsn == "" {
		t.Skipf("%s not set, skipping integration test", testDSNEnv)
	}

	ctx := t.Context()
	if err := postgres.Migrate(ctx, dsn); err != nil {
		t.Fatalf("Migrate(): %v", err)
	}

	store, err := postgres.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("Open(): %v", err)
	}
	t.Cleanup(store.Close)

	if err := postgres.TruncateAll(ctx, store); err != nil {
		t.Fatalf("truncate tables: %v", err)
	}
	return store, ctx
}

// pendingMatch seeds two players and a match Anna reported, so Bodo is the
// one who has to rule on it.
func pendingMatch(ctx context.Context, t *testing.T, store *postgres.Store) (anna, bodo domain.Player, m domain.Match) {
	t.Helper()

	var err error
	if anna, err = store.Players().Create(ctx, "Anna", domain.DefaultTTR); err != nil {
		t.Fatalf("create Anna: %v", err)
	}
	if bodo, err = store.Players().Create(ctx, "Bodo", domain.DefaultTTR); err != nil {
		t.Fatalf("create Bodo: %v", err)
	}

	m, err = store.Matches().Create(ctx, domain.Match{
		HomeID: anna.ID, AwayID: bodo.ID,
		BestOf: 3, PointsToWin: 11,
		Status: domain.MatchPending, ReportedBy: anna.ID,
		Sets: []domain.MatchSet{
			{SetNo: 1, HomePoints: 11, AwayPoints: 9},
			{SetNo: 2, HomePoints: 12, AwayPoints: 10},
		},
	})
	if err != nil {
		t.Fatalf("create the match: %v", err)
	}
	return anna, bodo, m
}

func ttrOf(ctx context.Context, t *testing.T, store repository.Store, id uuid.UUID) int {
	t.Helper()

	p, err := store.Players().ByID(ctx, id)
	if err != nil {
		t.Fatalf("ByID(): %v", err)
	}
	return p.TTR
}

// TestAPendingMatchDoesNotCount is the Definition of Done of AP5. Recording a
// result must move nothing until the opponent agrees it happened.
func TestAPendingMatchDoesNotCount(t *testing.T) {
	store, ctx := newStore(t)
	anna, bodo, _ := pendingMatch(ctx, t, store)

	if got := ttrOf(ctx, t, store, anna.ID); got != domain.DefaultTTR {
		t.Errorf("Anna is on %d, want the starting %d", got, domain.DefaultTTR)
	}
	if got := ttrOf(ctx, t, store, bodo.ID); got != domain.DefaultTTR {
		t.Errorf("Bodo is on %d, want the starting %d", got, domain.DefaultTTR)
	}

	for _, p := range []domain.Player{anna, bodo} {
		history, err := store.TTRHistory().ForPlayer(ctx, p.ID, 10)
		if err != nil {
			t.Fatalf("ForPlayer(): %v", err)
		}
		if len(history) != 0 {
			t.Errorf("%s already has %d history entries, want 0", p.DisplayName, len(history))
		}
	}
}

func TestConfirmSettlesTheMatch(t *testing.T) {
	store, ctx := newStore(t)
	anna, bodo, m := pendingMatch(ctx, t, store)

	at := time.Now()
	settlement, err := scoring.Confirm(ctx, store, m.ID, bodo.ID, at)
	if err != nil {
		t.Fatalf("Confirm(): %v", err)
	}

	// Equal ratings, Anna won: 16 * (1 - 0.5) = +8.
	if settlement.HomeChange.Delta() != 8 || settlement.AwayChange.Delta() != -8 {
		t.Errorf("changes = %+d / %+d, want +8 / -8",
			settlement.HomeChange.Delta(), settlement.AwayChange.Delta())
	}

	if got := ttrOf(ctx, t, store, anna.ID); got != 1008 {
		t.Errorf("Anna is on %d, want 1008", got)
	}
	if got := ttrOf(ctx, t, store, bodo.ID); got != 992 {
		t.Errorf("Bodo is on %d, want 992", got)
	}

	stored, err := store.Matches().ByID(ctx, m.ID)
	if err != nil {
		t.Fatalf("ByID(): %v", err)
	}
	if stored.Status != domain.MatchConfirmed {
		t.Errorf("status = %q, want confirmed", stored.Status)
	}
	// The schema ties the two together (matches_confirmed_at_matches_status),
	// so a confirmed match without a timestamp cannot exist — checking it
	// here proves the write went through the intended path.
	if stored.ConfirmedAt == nil {
		t.Error("ConfirmedAt is nil on a confirmed match")
	}

	for _, p := range []domain.Player{anna, bodo} {
		history, err := store.TTRHistory().ForPlayer(ctx, p.ID, 10)
		if err != nil {
			t.Fatalf("ForPlayer(): %v", err)
		}
		if len(history) != 1 {
			t.Fatalf("%s has %d history entries, want 1", p.DisplayName, len(history))
		}
		if history[0].TTRBefore != domain.DefaultTTR {
			t.Errorf("%s: TTRBefore = %d, want %d", p.DisplayName, history[0].TTRBefore, domain.DefaultTTR)
		}
	}
}

func TestConfirmRatesTheUnderdogsWinHigher(t *testing.T) {
	store, ctx := newStore(t)
	anna, bodo, m := pendingMatch(ctx, t, store)

	// Bodo is the clear favourite, and loses.
	if err := store.Players().UpdateTTR(ctx, bodo.ID, 1150); err != nil {
		t.Fatalf("UpdateTTR(): %v", err)
	}

	settlement, err := scoring.Confirm(ctx, store, m.ID, bodo.ID, time.Now())
	if err != nil {
		t.Fatalf("Confirm(): %v", err)
	}

	// E = 0.090909 for Anna, so 16 * (1 - 0.090909) = 14.5455, rounded to 15.
	if settlement.HomeChange.Delta() != 15 {
		t.Errorf("Anna gained %+d, want +15", settlement.HomeChange.Delta())
	}
	if got := ttrOf(ctx, t, store, anna.ID); got != 1015 {
		t.Errorf("Anna is on %d, want 1015", got)
	}
	if got := ttrOf(ctx, t, store, bodo.ID); got != 1135 {
		t.Errorf("Bodo is on %d, want 1135", got)
	}
}

// TestAWinWorthNothingIsStillAWin guards a trap the rating system sets: a
// strong favourite who wins gains zero points after rounding. Anything that
// reads "did they win?" off the rating change reports a loss here.
func TestAWinWorthNothingIsStillAWin(t *testing.T) {
	store, ctx := newStore(t)
	anna, bodo, m := pendingMatch(ctx, t, store)

	// 300 points ahead: E = 0.990099, so 16 * 0.009901 = 0.16, rounded to 0.
	if err := store.Players().UpdateTTR(ctx, anna.ID, 1300); err != nil {
		t.Fatalf("UpdateTTR(): %v", err)
	}

	settlement, err := scoring.Confirm(ctx, store, m.ID, bodo.ID, time.Now())
	if err != nil {
		t.Fatalf("Confirm(): %v", err)
	}

	if settlement.HomeChange.Delta() != 0 {
		t.Fatalf("Anna moved by %+d, want 0 — the premise of this test is gone",
			settlement.HomeChange.Delta())
	}
	if !settlement.HomeWon {
		t.Error("HomeWon = false although Anna took the match 2:0")
	}
	if settlement.HomeSets != 2 || settlement.AwaySets != 0 {
		t.Errorf("sets = %d:%d, want 2:0", settlement.HomeSets, settlement.AwaySets)
	}

	// The history still records the match, even though nothing moved: a
	// missing row would look like the match was never settled.
	history, err := store.TTRHistory().ForPlayer(ctx, anna.ID, 10)
	if err != nil {
		t.Fatalf("ForPlayer(): %v", err)
	}
	if len(history) != 1 {
		t.Errorf("%d history entries, want 1", len(history))
	}
}

func TestConfirmTwice(t *testing.T) {
	store, ctx := newStore(t)
	_, bodo, m := pendingMatch(ctx, t, store)

	if _, err := scoring.Confirm(ctx, store, m.ID, bodo.ID, time.Now()); err != nil {
		t.Fatalf("Confirm(): %v", err)
	}

	_, err := scoring.Confirm(ctx, store, m.ID, bodo.ID, time.Now())
	if !errors.Is(err, scoring.ErrNotPending) {
		t.Fatalf("second Confirm() = %v, want scoring.ErrNotPending", err)
	}

	// The rating must not have moved twice. The unique index on the history
	// would have refused the second write anyway; this checks that nothing
	// slipped through before it.
	if got := ttrOf(ctx, t, store, bodo.ID); got != 992 {
		t.Errorf("Bodo is on %d after a repeated confirmation, want 992", got)
	}
}

// TestTheReporterCannotConfirm is the point of the whole work package. A
// result confirmed by whoever entered it is not confirmed at all, and this is
// the actual defence against a joke result — see the threat-model note in
// docs/adr/0004.
func TestTheReporterCannotConfirm(t *testing.T) {
	store, ctx := newStore(t)
	anna, _, m := pendingMatch(ctx, t, store)

	_, err := scoring.Confirm(ctx, store, m.ID, anna.ID, time.Now())

	if !errors.Is(err, scoring.ErrNotYours) {
		t.Fatalf("Confirm() by the reporter = %v, want scoring.ErrNotYours", err)
	}
	if got := ttrOf(ctx, t, store, anna.ID); got != domain.DefaultTTR {
		t.Errorf("Anna is on %d, want the starting %d", got, domain.DefaultTTR)
	}
}

func TestAByStanderCannotConfirm(t *testing.T) {
	store, ctx := newStore(t)
	_, _, m := pendingMatch(ctx, t, store)

	cleo, err := store.Players().Create(ctx, "Cleo", domain.DefaultTTR)
	if err != nil {
		t.Fatalf("create Cleo: %v", err)
	}

	if _, err := scoring.Confirm(ctx, store, m.ID, cleo.ID, time.Now()); !errors.Is(err, scoring.ErrNotYours) {
		t.Fatalf("Confirm() by a bystander = %v, want scoring.ErrNotYours", err)
	}
}

func TestDisputeBlocksScoring(t *testing.T) {
	store, ctx := newStore(t)
	anna, bodo, m := pendingMatch(ctx, t, store)

	if err := scoring.Dispute(ctx, store, m.ID, bodo.ID); err != nil {
		t.Fatalf("Dispute(): %v", err)
	}

	stored, err := store.Matches().ByID(ctx, m.ID)
	if err != nil {
		t.Fatalf("ByID(): %v", err)
	}
	if stored.Status != domain.MatchDisputed {
		t.Errorf("status = %q, want disputed", stored.Status)
	}
	if stored.ConfirmedAt != nil {
		t.Error("ConfirmedAt is set on a disputed match")
	}

	if got := ttrOf(ctx, t, store, anna.ID); got != domain.DefaultTTR {
		t.Errorf("Anna is on %d, want the starting %d", got, domain.DefaultTTR)
	}

	// And it stays blocked: resolving a dispute is a manual step in the MVP
	// (issue #18), not something a second click can undo.
	if _, err := scoring.Confirm(ctx, store, m.ID, bodo.ID, time.Now()); !errors.Is(err, scoring.ErrNotPending) {
		t.Errorf("Confirm() after a dispute = %v, want scoring.ErrNotPending", err)
	}
}

func TestDisputeRefusesTheReporter(t *testing.T) {
	store, ctx := newStore(t)
	anna, _, m := pendingMatch(ctx, t, store)

	if err := scoring.Dispute(ctx, store, m.ID, anna.ID); !errors.Is(err, scoring.ErrNotYours) {
		t.Fatalf("Dispute() by the reporter = %v, want scoring.ErrNotYours", err)
	}
}

func TestConfirmAnUnknownMatch(t *testing.T) {
	store, ctx := newStore(t)
	_, bodo, _ := pendingMatch(ctx, t, store)

	if _, err := scoring.Confirm(ctx, store, uuid.New(), bodo.ID, time.Now()); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Confirm() for an unknown match = %v, want domain.ErrNotFound", err)
	}
}

// disputedMatch is pendingMatch with Bodo having said it is wrong.
func disputedMatch(ctx context.Context, t *testing.T, store *postgres.Store) (anna, bodo domain.Player, m domain.Match) {
	t.Helper()

	anna, bodo, m = pendingMatch(ctx, t, store)
	if err := scoring.Dispute(ctx, store, m.ID, bodo.ID); err != nil {
		t.Fatalf("Dispute(): %v", err)
	}
	return anna, bodo, m
}

// TestCorrectHandsTheMatchBack is the Definition of Done of issue #18: a
// contested result reaches a rating without anybody opening psql.
func TestCorrectHandsTheMatchBack(t *testing.T) {
	store, ctx := newStore(t)
	anna, bodo, m := disputedMatch(ctx, t, store)

	// Bodo says he actually won it, in three sets rather than two.
	corrected := match.Result{
		Mode: match.Mode{BestOf: 3, PointsToWin: 11},
		Sets: []match.Set{{Home: 11, Away: 9}, {Home: 8, Away: 11}, {Home: 7, Away: 11}},
	}

	correction, err := scoring.Correct(ctx, store, m.ID, bodo.ID, corrected)
	if err != nil {
		t.Fatalf("Correct(): %v", err)
	}
	if correction.Opponent.ID != anna.ID {
		t.Errorf("the correction points at %s, want Anna", correction.Opponent.DisplayName)
	}
	if correction.HomeWon {
		t.Error("the corrected result still reads as a win for the home player")
	}

	// Read back from the database rather than from the return value: the
	// point of this package is what ends up stored.
	stored, err := store.Matches().ByID(ctx, m.ID)
	if err != nil {
		t.Fatalf("ByID(): %v", err)
	}
	if stored.Status != domain.MatchPending {
		t.Fatalf("status = %q, want pending", stored.Status)
	}
	if stored.ReportedBy != bodo.ID {
		t.Error("whoever corrected it is not the reporter, so the wrong player would confirm")
	}
	if len(stored.Sets) != 3 {
		t.Fatalf("%d sets stored, want 3 — the old ones were not replaced", len(stored.Sets))
	}
	if s := stored.Sets[2]; s.SetNo != 3 || s.HomePoints != 7 || s.AwayPoints != 11 {
		t.Errorf("set 3 stored as %d:%d (no %d), want 7:11 (no 3)", s.HomePoints, s.AwayPoints, s.SetNo)
	}

	// Nothing has been scored yet: a correction is a claim like any other.
	if got := ttrOf(ctx, t, store, bodo.ID); got != domain.DefaultTTR {
		t.Errorf("Bodo is on %d before the confirmation, want %d", got, domain.DefaultTTR)
	}

	if _, err := scoring.Confirm(ctx, store, m.ID, anna.ID, time.Now()); err != nil {
		t.Fatalf("Confirm() after the correction: %v", err)
	}
	if got := ttrOf(ctx, t, store, bodo.ID); got != 1008 {
		t.Errorf("Bodo is on %d after winning the corrected match, want 1008", got)
	}
}

// TestCorrectRejectsAnImpossibleResult keeps a correction under exactly the
// rules a fresh entry is under, and — because it runs in a transaction —
// leaves nothing behind when it refuses.
func TestCorrectRejectsAnImpossibleResult(t *testing.T) {
	store, ctx := newStore(t)
	_, bodo, m := disputedMatch(ctx, t, store)

	// 11:10 is one clear point, not two.
	_, err := scoring.Correct(ctx, store, m.ID, bodo.ID, match.Result{
		Mode: match.Mode{BestOf: 3, PointsToWin: 11},
		Sets: []match.Set{{Home: 11, Away: 10}, {Home: 11, Away: 9}},
	})

	var rejection *match.Rejection
	if !errors.As(err, &rejection) {
		t.Fatalf("Correct() = %v, want a rejection", err)
	}

	stored, err := store.Matches().ByID(ctx, m.ID)
	if err != nil {
		t.Fatalf("ByID(): %v", err)
	}
	if stored.Status != domain.MatchDisputed {
		t.Errorf("status = %q, want the match left disputed", stored.Status)
	}
	if len(stored.Sets) != 2 || stored.Sets[0].HomePoints != 11 || stored.Sets[0].AwayPoints != 9 {
		t.Error("the refused correction changed the stored sets anyway")
	}
}

func TestCorrectOnlyTouchesAContestedMatch(t *testing.T) {
	store, ctx := newStore(t)
	anna, bodo, m := pendingMatch(ctx, t, store)

	valid := match.Result{
		Mode: match.Mode{BestOf: 3, PointsToWin: 11},
		Sets: []match.Set{{Home: 11, Away: 9}, {Home: 11, Away: 7}},
	}

	if _, err := scoring.Correct(ctx, store, m.ID, bodo.ID, valid); !errors.Is(err, scoring.ErrNotDisputed) {
		t.Errorf("correcting a pending match = %v, want ErrNotDisputed", err)
	}

	if _, err := scoring.Confirm(ctx, store, m.ID, bodo.ID, time.Now()); err != nil {
		t.Fatalf("Confirm(): %v", err)
	}
	if _, err := scoring.Correct(ctx, store, m.ID, anna.ID, valid); !errors.Is(err, scoring.ErrNotDisputed) {
		t.Errorf("correcting a confirmed match = %v, want ErrNotDisputed", err)
	}
	if got := ttrOf(ctx, t, store, anna.ID); got != 1008 {
		t.Errorf("Anna is on %d, so a settled result was rewritten", got)
	}
}

func TestABystanderCannotCorrect(t *testing.T) {
	store, ctx := newStore(t)
	_, _, m := disputedMatch(ctx, t, store)

	cara, err := store.Players().Create(ctx, "Cara", domain.DefaultTTR)
	if err != nil {
		t.Fatalf("create Cara: %v", err)
	}

	_, err = scoring.Correct(ctx, store, m.ID, cara.ID, match.Result{
		Mode: match.Mode{BestOf: 3, PointsToWin: 11},
		Sets: []match.Set{{Home: 11, Away: 9}, {Home: 11, Away: 7}},
	})
	if !errors.Is(err, scoring.ErrNotYours) {
		t.Errorf("Correct() by a bystander = %v, want ErrNotYours", err)
	}
}

// TestRecordSettlesWithoutAsking is what the kiosk rests on: somebody watched
// the match and wrote it down, so the result counts on the spot.
func TestRecordSettlesWithoutAsking(t *testing.T) {
	store, ctx := newStore(t)

	anna, err := store.Players().Create(ctx, "Anna", domain.DefaultTTR)
	if err != nil {
		t.Fatalf("create Anna: %v", err)
	}
	bodo, err := store.Players().Create(ctx, "Bodo", domain.DefaultTTR)
	if err != nil {
		t.Fatalf("create Bodo: %v", err)
	}

	settlement, err := scoring.Record(ctx, store, anna.ID, bodo.ID, match.Result{
		Mode: match.Mode{BestOf: 3, PointsToWin: 11},
		Sets: []match.Set{{Home: 11, Away: 9}, {Home: 12, Away: 10}},
	}, time.Now())
	if err != nil {
		t.Fatalf("Record(): %v", err)
	}
	if !settlement.HomeWon || settlement.HomeSets != 2 {
		t.Errorf("settlement reads %d:%d, home won = %v",
			settlement.HomeSets, settlement.AwaySets, settlement.HomeWon)
	}

	stored, err := store.Matches().ByID(ctx, settlement.Match.ID)
	if err != nil {
		t.Fatalf("ByID(): %v", err)
	}
	if stored.Status != domain.MatchConfirmed || stored.ConfirmedAt == nil {
		t.Errorf("status = %q, confirmedAt = %v — a recorded match waits on nobody",
			stored.Status, stored.ConfirmedAt)
	}
	if len(stored.Sets) != 2 {
		t.Errorf("%d sets stored, want 2", len(stored.Sets))
	}

	if got := ttrOf(ctx, t, store, anna.ID); got != 1008 {
		t.Errorf("Anna is on %d, want 1008", got)
	}
	if got := ttrOf(ctx, t, store, bodo.ID); got != 992 {
		t.Errorf("Bodo is on %d, want 992", got)
	}

	// The history is what a profile draws from, so it has to be there too.
	history, err := store.TTRHistory().ForPlayer(ctx, anna.ID, 10)
	if err != nil {
		t.Fatalf("ForPlayer(): %v", err)
	}
	if len(history) != 1 || history[0].TTRAfter != 1008 {
		t.Errorf("history = %+v, want one entry ending at 1008", history)
	}

	// Nothing waits on either of them.
	for name, id := range map[string]uuid.UUID{"Anna": anna.ID, "Bodo": bodo.ID} {
		n, err := store.Matches().PendingCountFor(ctx, id)
		if err != nil {
			t.Fatalf("PendingCountFor(%s): %v", name, err)
		}
		if n != 0 {
			t.Errorf("%d results wait on %s after a recorded match, want 0", n, name)
		}
	}
}

// TestRecordLeavesNothingBehindWhenItRefuses: create and settle share one
// transaction, so an impossible result must not leave a match row.
func TestRecordLeavesNothingBehindWhenItRefuses(t *testing.T) {
	store, ctx := newStore(t)

	anna, err := store.Players().Create(ctx, "Anna", domain.DefaultTTR)
	if err != nil {
		t.Fatalf("create Anna: %v", err)
	}
	bodo, err := store.Players().Create(ctx, "Bodo", domain.DefaultTTR)
	if err != nil {
		t.Fatalf("create Bodo: %v", err)
	}

	// 11:10 is one clear point, not two.
	_, err = scoring.Record(ctx, store, anna.ID, bodo.ID, match.Result{
		Mode: match.Mode{BestOf: 3, PointsToWin: 11},
		Sets: []match.Set{{Home: 11, Away: 10}, {Home: 11, Away: 9}},
	}, time.Now())

	var rejection *match.Rejection
	if !errors.As(err, &rejection) {
		t.Fatalf("Record() = %v, want a rejection", err)
	}

	recent, err := store.Matches().RecentFor(ctx, anna.ID, 10)
	if err != nil {
		t.Fatalf("RecentFor(): %v", err)
	}
	if len(recent) != 0 {
		t.Errorf("%d matches survived a refused recording", len(recent))
	}
	if got := ttrOf(ctx, t, store, anna.ID); got != domain.DefaultTTR {
		t.Errorf("Anna moved to %d on a refused recording", got)
	}
}

func TestRecordRefusesAPlayerAgainstThemselves(t *testing.T) {
	store, ctx := newStore(t)

	anna, err := store.Players().Create(ctx, "Anna", domain.DefaultTTR)
	if err != nil {
		t.Fatalf("create Anna: %v", err)
	}

	_, err = scoring.Record(ctx, store, anna.ID, anna.ID, match.Result{
		Mode: match.Mode{BestOf: 3, PointsToWin: 11},
		Sets: []match.Set{{Home: 11, Away: 9}, {Home: 11, Away: 7}},
	}, time.Now())

	if !errors.Is(err, scoring.ErrSamePlayer) {
		t.Errorf("Record() against themselves = %v, want ErrSamePlayer", err)
	}
}

// TestUndoPutsBothRatingsBack is the answer to a kiosk typo: a result there
// counts at once, so there is nothing to dispute and nothing to correct.
func TestUndoPutsBothRatingsBack(t *testing.T) {
	store, ctx := newStore(t)
	anna, bodo, m := pendingMatch(ctx, t, store)

	settled, err := scoring.Confirm(ctx, store, m.ID, bodo.ID, time.Now())
	if err != nil {
		t.Fatalf("Confirm(): %v", err)
	}
	if ttrOf(ctx, t, store, anna.ID) == domain.DefaultTTR {
		t.Fatal("the rating did not move, so there is nothing to take back")
	}

	undone, err := scoring.Undo(ctx, store, settled.Match.ID, time.Now())
	if err != nil {
		t.Fatalf("Undo(): %v", err)
	}

	if got := ttrOf(ctx, t, store, anna.ID); got != domain.DefaultTTR {
		t.Errorf("Anna is on %d, want %d", got, domain.DefaultTTR)
	}
	if got := ttrOf(ctx, t, store, bodo.ID); got != domain.DefaultTTR {
		t.Errorf("Bodo is on %d, want %d", got, domain.DefaultTTR)
	}
	if undone.HomeSets != 2 || undone.AwaySets != 0 {
		t.Errorf("undone says %d:%d, want 2:0", undone.HomeSets, undone.AwaySets)
	}

	// The match and everything the schema hangs off it are gone.
	if _, err := store.Matches().ByID(ctx, settled.Match.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("the match is still there: %v", err)
	}
	history, err := store.TTRHistory().ForPlayer(ctx, anna.ID, 10)
	if err != nil {
		t.Fatalf("ForPlayer(): %v", err)
	}
	if len(history) != 0 {
		t.Errorf("the rating history survived the undo: %+v", history)
	}
}

func TestUndoRefusesOnceSomethingElseHasCounted(t *testing.T) {
	// Putting the ratings back means writing ttr_before straight back, and
	// that is right only while nothing has counted since. A later match
	// would be undone along with it, silently.
	store, ctx := newStore(t)
	anna, bodo, first := pendingMatch(ctx, t, store)

	if _, err := scoring.Confirm(ctx, store, first.ID, bodo.ID, time.Now()); err != nil {
		t.Fatalf("Confirm(): %v", err)
	}
	if _, err := scoring.Record(ctx, store, bodo.ID, anna.ID, match.Result{
		Mode: match.Mode{BestOf: 3, PointsToWin: 11},
		Sets: []match.Set{{Home: 11, Away: 4}, {Home: 11, Away: 6}},
	}, time.Now()); err != nil {
		t.Fatalf("Record(): %v", err)
	}

	before := ttrOf(ctx, t, store, anna.ID)

	_, err := scoring.Undo(ctx, store, first.ID, time.Now())
	if !errors.Is(err, scoring.ErrNotLast) {
		t.Fatalf("Undo() = %v, want ErrNotLast", err)
	}
	if got := ttrOf(ctx, t, store, anna.ID); got != before {
		t.Errorf("the refused undo moved the rating anyway: %d, want %d", got, before)
	}
}

func TestUndoRefusesAnOldResult(t *testing.T) {
	store, ctx := newStore(t)
	_, bodo, m := pendingMatch(ctx, t, store)

	if _, err := scoring.Confirm(ctx, store, m.ID, bodo.ID, time.Now()); err != nil {
		t.Fatalf("Confirm(): %v", err)
	}

	// Asked for an hour later: the undo is for a typo somebody is still
	// looking at, not for editing the evening afterwards.
	_, err := scoring.Undo(ctx, store, m.ID, time.Now().Add(time.Hour))
	if !errors.Is(err, scoring.ErrTooLate) {
		t.Errorf("Undo() = %v, want ErrTooLate", err)
	}
}
