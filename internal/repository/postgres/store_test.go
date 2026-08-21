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

	if _, err := players.ByID(ctx, uuid.New()); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("ByID() for an unknown player = %v, want domain.ErrNotFound", err)
	}
	if err := players.UpdateTTR(ctx, uuid.New(), 1000); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("UpdateTTR() for an unknown player = %v, want domain.ErrNotFound", err)
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
