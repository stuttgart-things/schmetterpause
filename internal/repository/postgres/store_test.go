package postgres_test

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
)

// These tests need a real database and run only when SP_TEST_DATABASE_URL
// is set:
//
//	task test:integration
//
// They empty every table. The variable must point at a throwaway database,
// never at one holding real data.
const testDSNEnv = "SP_TEST_DATABASE_URL"

func newStore(t *testing.T) (*postgres.Store, context.Context) {
	t.Helper()

	dsn := os.Getenv(testDSNEnv)
	if dsn == "" {
		t.Skipf("%s not set, skipping integration test", testDSNEnv)
	}

	// Before Migrate, not only before the truncate: a wrong DSN must not
	// reach a live database with a schema change either (issue #163).
	if err := postgres.RequireTestDatabase(dsn); err != nil {
		t.Fatal(err)
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

	truncate(ctx, t, store)
	return store, ctx
}

// truncate empties every table, bypassing the repository layer. The only
// place in the project where test code issues SQL.
func truncate(ctx context.Context, t *testing.T, store *postgres.Store) {
	t.Helper()

	err := store.InTx(ctx, func(repository.Store) error { return nil })
	if err != nil {
		t.Fatalf("database not usable: %v", err)
	}
	if err := postgres.TruncateAll(ctx, store); err != nil {
		t.Fatalf("truncate tables: %v", err)
	}
}

func mustPlayer(ctx context.Context, t *testing.T, store *postgres.Store, name string, ttr int) domain.Player {
	t.Helper()

	p, err := store.Players().Create(ctx, name, ttr)
	if err != nil {
		t.Fatalf("create player %q: %v", name, err)
	}
	return p
}

func TestPlayerRepository(t *testing.T) {
	store, ctx := newStore(t)
	players := store.Players()

	anna := mustPlayer(ctx, t, store, "Anna", 1100)
	bodo := mustPlayer(ctx, t, store, "Bodo", 900)

	got, err := players.ByID(ctx, anna.ID)
	if err != nil {
		t.Fatalf("ByID(): %v", err)
	}
	if got.DisplayName != "Anna" || got.TTR != 1100 {
		t.Errorf("ByID() = %+v, want Anna/1100", got)
	}

	n, err := players.Count(ctx)
	if err != nil {
		t.Fatalf("Count(): %v", err)
	}
	if n != 2 {
		t.Errorf("Count() = %d, want 2", n)
	}

	// The list is the ranking order: highest rating first.
	list, err := players.List(ctx)
	if err != nil {
		t.Fatalf("List(): %v", err)
	}
	if len(list) != 2 || list[0].ID != anna.ID || list[1].ID != bodo.ID {
		t.Errorf("List() returned the wrong order: %+v", list)
	}

	if err := players.UpdateTTR(ctx, bodo.ID, 1250); err != nil {
		t.Fatalf("UpdateTTR(): %v", err)
	}
	list, err = players.List(ctx)
	if err != nil {
		t.Fatalf("List(): %v", err)
	}
	if list[0].ID != bodo.ID {
		t.Errorf("Bodo is not first after the rating update: %+v", list)
	}

	// The unique index is case- and whitespace-insensitive, and the driver's
	// SQLSTATE has to arrive as domain.ErrConflict — otherwise a taken name
	// reaches the player as a 500 instead of a message saying it is taken.
	if _, err := players.Create(ctx, "  ANNA  ", domain.DefaultTTR); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("Create() with a taken name = %v, want domain.ErrConflict", err)
	}

	if _, err := players.ByID(ctx, uuid.New()); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("ByID() for an unknown player = %v, want domain.ErrNotFound", err)
	}
	if err := players.UpdateTTR(ctx, uuid.New(), 1000); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("UpdateTTR() for an unknown player = %v, want domain.ErrNotFound", err)
	}
}

// TestPlayerRecords covers the aggregate behind the ranking. Two things it
// has to get right and neither is visible from the Go side: only confirmed
// matches count, and the winner comes from the set scores rather than from
// the rating change — a strong favourite who wins can move by zero points.
func TestPlayerRecords(t *testing.T) {
	store, ctx := newStore(t)

	anna := mustPlayer(ctx, t, store, "Anna", 1100)
	bodo := mustPlayer(ctx, t, store, "Bodo", 1000)
	mustPlayer(ctx, t, store, "Cleo", 900)

	// Anna beats Bodo, confirmed.
	confirmed, err := store.Matches().Create(ctx, domain.Match{
		HomeID: anna.ID, AwayID: bodo.ID, BestOf: 3, PointsToWin: 11,
		Status: domain.MatchPending, ReportedBy: bodo.ID,
		Sets: []domain.MatchSet{{SetNo: 1, HomePoints: 11, AwayPoints: 9}, {SetNo: 2, HomePoints: 11, AwayPoints: 7}},
	})
	if err != nil {
		t.Fatalf("create the confirmed match: %v", err)
	}
	now := time.Now()
	if err := store.Matches().SetStatus(ctx, confirmed.ID, domain.MatchConfirmed, &now); err != nil {
		t.Fatalf("SetStatus(): %v", err)
	}

	// One pending and one disputed, neither of which may count.
	pending, err := store.Matches().Create(ctx, domain.Match{
		HomeID: anna.ID, AwayID: bodo.ID, BestOf: 3, PointsToWin: 11,
		Status: domain.MatchPending, ReportedBy: anna.ID,
		Sets: []domain.MatchSet{{SetNo: 1, HomePoints: 11, AwayPoints: 0}, {SetNo: 2, HomePoints: 11, AwayPoints: 0}},
	})
	if err != nil {
		t.Fatalf("create the pending match: %v", err)
	}
	_ = pending

	disputed, err := store.Matches().Create(ctx, domain.Match{
		HomeID: bodo.ID, AwayID: anna.ID, BestOf: 3, PointsToWin: 11,
		Status: domain.MatchPending, ReportedBy: anna.ID,
		Sets: []domain.MatchSet{{SetNo: 1, HomePoints: 11, AwayPoints: 0}, {SetNo: 2, HomePoints: 11, AwayPoints: 0}},
	})
	if err != nil {
		t.Fatalf("create the disputed match: %v", err)
	}
	if err := store.Matches().SetStatus(ctx, disputed.ID, domain.MatchDisputed, nil); err != nil {
		t.Fatalf("SetStatus(): %v", err)
	}

	records, err := store.Players().Records(ctx)
	if err != nil {
		t.Fatalf("Records(): %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("%d records, want 3", len(records))
	}

	// Ordered by rating, best first — the same order List uses.
	wantOrder := []string{"Anna", "Bodo", "Cleo"}
	for i, want := range wantOrder {
		if got := records[i].Player.DisplayName; got != want {
			t.Errorf("record %d is %s, want %s", i, got, want)
		}
	}

	want := map[string][3]int{
		// played, won, lost
		"Anna": {1, 1, 0},
		"Bodo": {1, 0, 1},
		// A player with no matches must come back as a row of zeroes, not
		// vanish from the ranking.
		"Cleo": {0, 0, 0},
	}
	for _, record := range records {
		got := [3]int{record.Played, record.Won, record.Lost}
		if got != want[record.Player.DisplayName] {
			t.Errorf("%s: played/won/lost = %v, want %v",
				record.Player.DisplayName, got, want[record.Player.DisplayName])
		}
	}
}

func TestIdentityRepository(t *testing.T) {
	store, ctx := newStore(t)
	ids := store.Identities()

	anna := mustPlayer(ctx, t, store, "Anna", domain.DefaultTTR)
	bodo := mustPlayer(ctx, t, store, "Bodo", domain.DefaultTTR)

	if err := ids.Link(ctx, domain.ProviderLocal, "cookie-anna", anna.ID); err != nil {
		t.Fatalf("Link(): %v", err)
	}

	got, err := ids.PlayerBy(ctx, domain.ProviderLocal, "cookie-anna")
	if err != nil {
		t.Fatalf("PlayerBy(): %v", err)
	}
	if got.ID != anna.ID {
		t.Errorf("PlayerBy() = %s, want %s", got.ID, anna.ID)
	}

	// Linking the same player again is a no-op.
	if err := ids.Link(ctx, domain.ProviderLocal, "cookie-anna", anna.ID); err != nil {
		t.Errorf("repeated Link() = %v, want nil", err)
	}

	// Linking a different player stays a conflict: merging two players is a
	// separate, deliberate operation per ADR-0003.
	if err := ids.Link(ctx, domain.ProviderLocal, "cookie-anna", bodo.ID); err == nil {
		t.Error("Link() to a different player returned no error")
	}

	// A player can hold several identities.
	if err := ids.Link(ctx, domain.ProviderPasskey, "credential-1", anna.ID); err != nil {
		t.Fatalf("link second identity: %v", err)
	}
	list, err := ids.ForPlayer(ctx, anna.ID)
	if err != nil {
		t.Fatalf("ForPlayer(): %v", err)
	}
	if len(list) != 2 {
		t.Errorf("ForPlayer() = %d identities, want 2", len(list))
	}

	if _, err := ids.PlayerBy(ctx, domain.ProviderGitLab, "unbekannt"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("PlayerBy() for an unknown identity = %v, want domain.ErrNotFound", err)
	}
}

func TestMatchRepository(t *testing.T) {
	store, ctx := newStore(t)
	matches := store.Matches()

	anna := mustPlayer(ctx, t, store, "Anna", domain.DefaultTTR)
	bodo := mustPlayer(ctx, t, store, "Bodo", domain.DefaultTTR)

	created, err := matches.Create(ctx, domain.Match{
		HomeID:      anna.ID,
		AwayID:      bodo.ID,
		BestOf:      5,
		PointsToWin: 11,
		ReportedBy:  anna.ID,
		Sets: []domain.MatchSet{
			{SetNo: 1, HomePoints: 11, AwayPoints: 7},
			{SetNo: 2, HomePoints: 9, AwayPoints: 11},
			{SetNo: 3, HomePoints: 11, AwayPoints: 13},
			{SetNo: 4, HomePoints: 11, AwayPoints: 8},
			{SetNo: 5, HomePoints: 12, AwayPoints: 10},
		},
	})
	if err != nil {
		t.Fatalf("Create(): %v", err)
	}
	if created.Status != domain.MatchPending {
		t.Errorf("status = %q, want pending", created.Status)
	}

	got, err := matches.ByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("ByID(): %v", err)
	}
	if len(got.Sets) != 5 {
		t.Fatalf("ByID() returned %d sets, want 5", len(got.Sets))
	}
	if got.Sets[2].AwayPoints != 13 {
		t.Errorf("third set = %+v, want 11:13", got.Sets[2])
	}

	// Confirmation is the opponent's job: Anna recorded it, so the match
	// waits on Bodo and not on Anna.
	pendingForBodo, err := matches.PendingFor(ctx, bodo.ID)
	if err != nil {
		t.Fatalf("PendingFor(bodo): %v", err)
	}
	if len(pendingForBodo) != 1 {
		t.Errorf("PendingFor(bodo) = %d, want 1", len(pendingForBodo))
	}
	pendingForAnna, err := matches.PendingFor(ctx, anna.ID)
	if err != nil {
		t.Fatalf("PendingFor(anna): %v", err)
	}
	if len(pendingForAnna) != 0 {
		t.Errorf("PendingFor(anna) = %d, want 0", len(pendingForAnna))
	}

	now := time.Now()
	if err := matches.SetStatus(ctx, created.ID, domain.MatchConfirmed, &now); err != nil {
		t.Fatalf("SetStatus(): %v", err)
	}
	got, err = matches.ByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("ByID() after confirmation: %v", err)
	}
	if got.Status != domain.MatchConfirmed || got.ConfirmedAt == nil {
		t.Errorf("after confirmation: status=%q, confirmedAt=%v", got.Status, got.ConfirmedAt)
	}

	recent, err := matches.RecentFor(ctx, anna.ID, 10)
	if err != nil {
		t.Fatalf("RecentFor(): %v", err)
	}
	if len(recent) != 1 || len(recent[0].Sets) != 5 {
		t.Errorf("RecentFor() = %d matches with %d sets", len(recent), len(recent[0].Sets))
	}

	if _, err := matches.ByID(ctx, uuid.New()); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("ByID() for an unknown match = %v, want domain.ErrNotFound", err)
	}
}

// TestReplaceResult covers the guard the caller cannot provide: the status is
// part of the update's condition, so a match that stopped being contested
// between the check and the write is not rewritten anyway.
func TestReplaceResult(t *testing.T) {
	store, ctx := newStore(t)
	matches := store.Matches()

	anna := mustPlayer(ctx, t, store, "Anna", domain.DefaultTTR)
	bodo := mustPlayer(ctx, t, store, "Bodo", domain.DefaultTTR)

	created, err := matches.Create(ctx, domain.Match{
		HomeID: anna.ID, AwayID: bodo.ID,
		BestOf: 5, PointsToWin: 11,
		Status: domain.MatchPending, ReportedBy: anna.ID,
		Sets: []domain.MatchSet{
			{SetNo: 1, HomePoints: 11, AwayPoints: 9},
			{SetNo: 2, HomePoints: 11, AwayPoints: 7},
			{SetNo: 3, HomePoints: 11, AwayPoints: 5},
		},
	})
	if err != nil {
		t.Fatalf("Create(): %v", err)
	}

	corrected := domain.Match{
		BestOf: 3, PointsToWin: 21, ReportedBy: bodo.ID,
		Sets: []domain.MatchSet{
			{SetNo: 1, HomePoints: 19, AwayPoints: 21},
			{SetNo: 2, HomePoints: 21, AwayPoints: 23},
		},
	}

	// Pending is not contested, so there is nothing to correct yet.
	if err := matches.ReplaceResult(ctx, created.ID, corrected); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("ReplaceResult() on a pending match = %v, want domain.ErrConflict", err)
	}

	if err := matches.SetStatus(ctx, created.ID, domain.MatchDisputed, nil); err != nil {
		t.Fatalf("SetStatus(disputed): %v", err)
	}
	if err := matches.ReplaceResult(ctx, created.ID, corrected); err != nil {
		t.Fatalf("ReplaceResult(): %v", err)
	}

	got, err := matches.ByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("ByID(): %v", err)
	}
	switch {
	case got.Status != domain.MatchPending:
		t.Errorf("status = %q, want pending", got.Status)
	case got.ReportedBy != bodo.ID:
		t.Error("the reporter did not move to whoever corrected it")
	case got.BestOf != 3 || got.PointsToWin != 21:
		t.Errorf("mode = best of %d to %d, want best of 3 to 21", got.BestOf, got.PointsToWin)
	case len(got.Sets) != 2:
		t.Errorf("%d sets stored, want 2 — the old ones outlived the correction", len(got.Sets))
	case got.Sets[1].HomePoints != 21 || got.Sets[1].AwayPoints != 23:
		t.Errorf("set 2 = %d:%d, want 21:23", got.Sets[1].HomePoints, got.Sets[1].AwayPoints)
	}

	// It is back to pending, so a second correction has nothing to act on.
	if err := matches.ReplaceResult(ctx, created.ID, corrected); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("a repeated ReplaceResult() = %v, want domain.ErrConflict", err)
	}
	if err := matches.ReplaceResult(ctx, uuid.New(), corrected); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("ReplaceResult() on an unknown match = %v, want domain.ErrNotFound", err)
	}
}

// TestPendingForIncludesContestedMatches: a contested match waits on either
// side to correct it, so it has to be reachable from both. Without this it
// would live only in the answer to the dispute and vanish on a reload.
func TestPendingForIncludesContestedMatches(t *testing.T) {
	store, ctx := newStore(t)
	matches := store.Matches()

	anna := mustPlayer(ctx, t, store, "Anna", domain.DefaultTTR)
	bodo := mustPlayer(ctx, t, store, "Bodo", domain.DefaultTTR)

	created, err := matches.Create(ctx, domain.Match{
		HomeID: anna.ID, AwayID: bodo.ID,
		BestOf: 3, PointsToWin: 11,
		Status: domain.MatchPending, ReportedBy: anna.ID,
		Sets: []domain.MatchSet{{SetNo: 1, HomePoints: 11, AwayPoints: 9}},
	})
	if err != nil {
		t.Fatalf("Create(): %v", err)
	}
	if err := matches.SetStatus(ctx, created.ID, domain.MatchDisputed, nil); err != nil {
		t.Fatalf("SetStatus(disputed): %v", err)
	}

	for name, id := range map[string]uuid.UUID{"Anna": anna.ID, "Bodo": bodo.ID} {
		waiting, err := matches.PendingFor(ctx, id)
		if err != nil {
			t.Fatalf("PendingFor(%s): %v", name, err)
		}
		if len(waiting) != 1 {
			t.Errorf("PendingFor(%s) = %d, want the contested match", name, len(waiting))
		}

		// The badge in the top bar counts with a separate statement, so the
		// two have to agree or it will quietly say the wrong number.
		n, err := matches.PendingCountFor(ctx, id)
		if err != nil {
			t.Fatalf("PendingCountFor(%s): %v", name, err)
		}
		if n != len(waiting) {
			t.Errorf("PendingCountFor(%s) = %d, PendingFor() = %d", name, n, len(waiting))
		}
	}

	// Nobody else is involved, so nothing waits on them.
	cara := mustPlayer(ctx, t, store, "Cara", domain.DefaultTTR)
	if n, err := matches.PendingCountFor(ctx, cara.ID); err != nil || n != 0 {
		t.Errorf("PendingCountFor(Cara) = %d, %v, want 0", n, err)
	}
}

func TestTTRHistoryRepository(t *testing.T) {
	store, ctx := newStore(t)

	anna := mustPlayer(ctx, t, store, "Anna", domain.DefaultTTR)
	bodo := mustPlayer(ctx, t, store, "Bodo", domain.DefaultTTR)

	match, err := store.Matches().Create(ctx, domain.Match{
		HomeID: anna.ID, AwayID: bodo.ID, BestOf: 3, PointsToWin: 11, ReportedBy: anna.ID,
		Sets: []domain.MatchSet{{SetNo: 1, HomePoints: 11, AwayPoints: 5}, {SetNo: 2, HomePoints: 11, AwayPoints: 9}},
	})
	if err != nil {
		t.Fatalf("create match: %v", err)
	}

	changes := []domain.TTRChange{
		{PlayerID: anna.ID, MatchID: match.ID, TTRBefore: 1000, TTRAfter: 1008},
		{PlayerID: bodo.ID, MatchID: match.ID, TTRBefore: 1000, TTRAfter: 992},
	}
	if err := store.TTRHistory().Append(ctx, changes); err != nil {
		t.Fatalf("Append(): %v", err)
	}

	got, err := store.TTRHistory().ForPlayer(ctx, anna.ID, 10)
	if err != nil {
		t.Fatalf("ForPlayer(): %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("ForPlayer() = %d entries, want 1", len(got))
	}
	if got[0].Delta() != 8 {
		t.Errorf("Delta() = %d, want 8", got[0].Delta())
	}

	// One entry per player and match only, otherwise a rating could be
	// settled twice by accident.
	if err := store.TTRHistory().Append(ctx, changes[:1]); err == nil {
		t.Error("a duplicate entry for the same match was accepted")
	}
}

func TestInTxRollback(t *testing.T) {
	store, ctx := newStore(t)

	sentinel := errors.New("abort")

	err := store.InTx(ctx, func(tx repository.Store) error {
		if _, err := tx.Players().Create(ctx, "will be discarded", domain.DefaultTTR); err != nil {
			return err
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("InTx() = %v, want %v", err, sentinel)
	}

	n, err := store.Players().Count(ctx)
	if err != nil {
		t.Fatalf("Count(): %v", err)
	}
	if n != 0 {
		t.Errorf("%d players exist after rollback, want 0", n)
	}
}

func TestInTxCommit(t *testing.T) {
	store, ctx := newStore(t)

	err := store.InTx(ctx, func(tx repository.Store) error {
		_, err := tx.Players().Create(ctx, "stays", domain.DefaultTTR)
		return err
	})
	if err != nil {
		t.Fatalf("InTx(): %v", err)
	}

	n, err := store.Players().Count(ctx)
	if err != nil {
		t.Fatalf("Count(): %v", err)
	}
	if n != 1 {
		t.Errorf("%d players exist after commit, want 1", n)
	}
}

// TestRecentAndForMatches covers the two reads the match list is built from:
// everybody's matches rather than one player's, and what each was worth in
// one query rather than one per row.
func TestRecentAndForMatches(t *testing.T) {
	store, ctx := newStore(t)
	matches, history := store.Matches(), store.TTRHistory()

	anna := mustPlayer(ctx, t, store, "Anna", domain.DefaultTTR)
	bodo := mustPlayer(ctx, t, store, "Bodo", domain.DefaultTTR)
	cara := mustPlayer(ctx, t, store, "Cara", domain.DefaultTTR)

	older, err := matches.Create(ctx, domain.Match{
		HomeID: anna.ID, AwayID: bodo.ID, BestOf: 3, PointsToWin: 11,
		ReportedBy: anna.ID, PlayedAt: time.Now().Add(-2 * time.Hour),
		Sets: []domain.MatchSet{
			{SetNo: 1, HomePoints: 11, AwayPoints: 7},
			{SetNo: 2, HomePoints: 11, AwayPoints: 9},
		},
	})
	if err != nil {
		t.Fatalf("Create(older): %v", err)
	}
	newer, err := matches.Create(ctx, domain.Match{
		HomeID: cara.ID, AwayID: anna.ID, BestOf: 3, PointsToWin: 11,
		ReportedBy: cara.ID, PlayedAt: time.Now(),
		Sets: []domain.MatchSet{
			{SetNo: 1, HomePoints: 5, AwayPoints: 11},
			{SetNo: 2, HomePoints: 8, AwayPoints: 11},
		},
	})
	if err != nil {
		t.Fatalf("Create(newer): %v", err)
	}

	recent, err := matches.Recent(ctx, 10)
	if err != nil {
		t.Fatalf("Recent(): %v", err)
	}
	if len(recent) != 2 {
		t.Fatalf("Recent() = %d matches, want 2", len(recent))
	}
	// Newest first, and nobody's matches are left out — neither of these two
	// involves the same pair.
	if recent[0].ID != newer.ID || recent[1].ID != older.ID {
		t.Errorf("Recent() is not newest first: %s then %s", recent[0].ID, recent[1].ID)
	}
	// The sets come with them, or the list would have to fetch each match
	// again to say what the result was.
	if len(recent[0].Sets) != 2 || recent[0].Sets[0].AwayPoints != 11 {
		t.Errorf("Recent() lost the sets: %+v", recent[0].Sets)
	}

	if limited, err := matches.Recent(ctx, 1); err != nil || len(limited) != 1 {
		t.Errorf("Recent(1) = %d matches, err %v", len(limited), err)
	}

	// Only the older one has been settled, which is the case the list has to
	// tell apart: a match with no history is not a match worth zero points.
	if err := history.Append(ctx, []domain.TTRChange{
		{PlayerID: anna.ID, MatchID: older.ID, TTRBefore: 1000, TTRAfter: 1008},
		{PlayerID: bodo.ID, MatchID: older.ID, TTRBefore: 1000, TTRAfter: 992},
	}); err != nil {
		t.Fatalf("Append(): %v", err)
	}

	changes, err := history.ForMatches(ctx, []uuid.UUID{older.ID, newer.ID})
	if err != nil {
		t.Fatalf("ForMatches(): %v", err)
	}
	if len(changes) != 2 {
		t.Fatalf("ForMatches() = %d entries, want 2", len(changes))
	}
	for _, c := range changes {
		if c.MatchID != older.ID {
			t.Errorf("ForMatches() returned an entry for the unsettled match: %+v", c)
		}
	}

	// An empty request must not become "where match_id = any(null)", which
	// matches nothing but takes a round trip to find out.
	if got, err := history.ForMatches(ctx, nil); err != nil || got != nil {
		t.Errorf("ForMatches(nil) = %v, %v; want nil, nil", got, err)
	}
}
