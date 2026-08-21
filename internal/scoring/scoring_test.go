package scoring_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
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
