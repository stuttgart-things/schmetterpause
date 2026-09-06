package seed_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
	"github.com/stuttgart-things/schmetterpause/internal/repository/postgres"
	"github.com/stuttgart-things/schmetterpause/internal/seed"
)

// Like the repository suite, these need a real database and run only when
// SP_TEST_DATABASE_URL points at a throwaway one:
//
//	task test:integration
//
// A fake store would not test the thing that matters here. The fixture's whole
// claim is that it goes through the application's own write path — the
// transaction in scoring.Confirm, the rating update, the history row — and an
// in-memory double is exactly where that claim would hold by construction
// while failing against Postgres.
const testDSNEnv = "SP_TEST_DATABASE_URL"

// reference is the fixed clock the fixture counts its match dates back from.
// Fixed, because a test that reads the wall clock is a test that fails once a
// year at midnight.
var reference = time.Date(2026, 9, 1, 18, 0, 0, 0, time.UTC)

func newStore(t *testing.T) (*postgres.Store, context.Context) {
	t.Helper()

	dsn := os.Getenv(testDSNEnv)
	if dsn == "" {
		t.Skipf("%s not set, skipping integration test", testDSNEnv)
	}
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

	if err := postgres.TruncateAll(ctx, store); err != nil {
		t.Fatalf("TruncateAll(): %v", err)
	}
	return store, ctx
}

func TestSeedFillsAnEmptyDatabase(t *testing.T) {
	store, ctx := newStore(t)

	summary, err := seed.Run(ctx, store, reference)
	if err != nil {
		t.Fatalf("Run(): %v", err)
	}

	if summary.Players == 0 {
		t.Fatal("the fixture created no players")
	}
	if summary.Confirmed == 0 {
		t.Error("the fixture confirmed no match — the ranking would be every player at their starting rating")
	}

	players, err := store.Players().List(ctx)
	if err != nil {
		t.Fatalf("List(): %v", err)
	}
	if len(players) != summary.Players {
		t.Errorf("List() returned %d players, Run() reported %d", len(players), summary.Players)
	}

	// The ranking has to be a ranking. Every player on the same rating is what
	// an empty preview already looks like.
	if players[0].TTR == players[len(players)-1].TTR {
		t.Error("top and bottom of the ranking are on the same rating")
	}
}

// TestTheUnsettledScreensAreNotEmpty is the reason two of the twelve results
// are left unfinished. "Zu bestätigen" and "Wartet auf den Gegner" are
// invisible in a fixture where everything is confirmed — and they are exactly
// the screens somebody reviewing a change to them needs to see.
func TestTheUnsettledScreensAreNotEmpty(t *testing.T) {
	store, ctx := newStore(t)

	summary, err := seed.Run(ctx, store, reference)
	if err != nil {
		t.Fatalf("Run(): %v", err)
	}
	if summary.Pending == 0 {
		t.Error("nothing is waiting for a confirmation")
	}
	if summary.Disputed == 0 {
		t.Error("nothing is contested, so no correction is offered anywhere")
	}

	matches, err := store.Matches().Recent(ctx, 100)
	if err != nil {
		t.Fatalf("Recent(): %v", err)
	}

	var pending, disputed int
	for _, m := range matches {
		switch m.Status {
		case domain.MatchPending:
			pending++
		case domain.MatchDisputed:
			disputed++
		}
	}
	if pending != summary.Pending {
		t.Errorf("%d matches are pending in the database, Run() reported %d", pending, summary.Pending)
	}
	if disputed != summary.Disputed {
		t.Errorf("%d matches are contested in the database, Run() reported %d", disputed, summary.Disputed)
	}

	// Both sides of issue #159 have to have something in them: the opponent
	// is asked to confirm, and the reporter is told it has not landed.
	for _, m := range matches {
		if m.Status != domain.MatchPending {
			continue
		}
		waiting, err := store.Matches().WaitingOnOpponentFor(ctx, m.ReportedBy)
		if err != nil {
			t.Fatalf("WaitingOnOpponentFor(): %v", err)
		}
		if len(waiting) == 0 {
			t.Error("the reporter of the pending match is not shown that it is waiting")
		}

		other := m.AwayID
		if m.ReportedBy == m.AwayID {
			other = m.HomeID
		}
		count, err := store.Matches().PendingCountFor(ctx, other)
		if err != nil {
			t.Fatalf("PendingCountFor(): %v", err)
		}
		if count == 0 {
			t.Error("the opponent is not asked to confirm the pending match")
		}
	}
}

// TestSeedRefusesADatabaseThatIsInUse is the whole safety story. There is no
// flag to pass correctly and no environment to configure: an instance people
// use has players, and that is what makes it refuse.
func TestSeedRefusesADatabaseThatIsInUse(t *testing.T) {
	store, ctx := newStore(t)

	if _, err := store.Players().Create(ctx, "Anna", 1000); err != nil {
		t.Fatalf("Create(): %v", err)
	}

	_, err := seed.Run(ctx, store, reference)
	if !errors.Is(err, seed.ErrNotEmpty) {
		t.Fatalf("Run() = %v, want ErrNotEmpty", err)
	}

	// And it refused before writing anything, rather than partway through.
	count, err := store.Players().Count(ctx)
	if err != nil {
		t.Fatalf("Count(): %v", err)
	}
	if count != 1 {
		t.Errorf("the refused run left %d players behind, want the 1 that was there", count)
	}

	matches, err := store.Matches().Recent(ctx, 10)
	if err != nil {
		t.Fatalf("Recent(): %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("the refused run wrote %d matches", len(matches))
	}
}

// TestSeedIsDeterministic guards the property a screenshot depends on: two
// previews of the same commit are the same preview.
func TestSeedIsDeterministic(t *testing.T) {
	store, ctx := newStore(t)

	first := ratingsAfterSeed(t, ctx, store)

	if err := postgres.TruncateAll(ctx, store); err != nil {
		t.Fatalf("TruncateAll(): %v", err)
	}

	second := ratingsAfterSeed(t, ctx, store)

	if len(first) != len(second) {
		t.Fatalf("two runs produced %d and %d players", len(first), len(second))
	}
	for name, ttr := range first {
		if second[name] != ttr {
			t.Errorf("%s is on %d after the first run and %d after the second", name, ttr, second[name])
		}
	}
}

func ratingsAfterSeed(t *testing.T, ctx context.Context, store *postgres.Store) map[string]int {
	t.Helper()

	if _, err := seed.Run(ctx, store, reference); err != nil {
		t.Fatalf("Run(): %v", err)
	}
	players, err := store.Players().List(ctx)
	if err != nil {
		t.Fatalf("List(): %v", err)
	}

	ratings := make(map[string]int, len(players))
	for _, p := range players {
		ratings[p.DisplayName] = p.TTR
	}
	return ratings
}
