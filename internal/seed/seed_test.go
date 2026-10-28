package seed_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stuttgart-things/schmetterpause/internal/avatar"
	"github.com/stuttgart-things/schmetterpause/internal/domain"
	"github.com/stuttgart-things/schmetterpause/internal/repository/postgres"
	"github.com/stuttgart-things/schmetterpause/internal/seed"
	"github.com/stuttgart-things/schmetterpause/internal/tournament"
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

// TestTheTournamentScreensAreNotEmpty is the reason the fixture opens a
// bracket. The notice at the top of the start page, the tournament list and
// the draw are all empty without one — and a table of zeroes says as little
// about a table as an empty ranking does about a ranking.
func TestTheTournamentScreensAreNotEmpty(t *testing.T) {
	store, ctx := newStore(t)

	summary, err := seed.Run(ctx, store, reference)
	if err != nil {
		t.Fatalf("Run(): %v", err)
	}
	if summary.Tournaments == 0 {
		t.Fatal("the fixture opened no tournament")
	}
	if summary.TournamentPlayed == 0 {
		t.Error("the tournament has no results, so its table is all zeroes")
	}

	tours, err := store.Tournaments().List(ctx, 10)
	if err != nil {
		t.Fatalf("List(): %v", err)
	}
	if len(tours) != summary.Tournaments {
		t.Fatalf("%d tournaments in the database, Run() reported %d", len(tours), summary.Tournaments)
	}

	tour := tours[0]
	if !tour.Open() {
		t.Error("the tournament is closed, so nothing is 'running' anywhere")
	}

	booked, err := store.Tournaments().Matches(ctx, tour.ID)
	if err != nil {
		t.Fatalf("Matches(): %v", err)
	}
	if len(booked) != summary.TournamentPlayed {
		t.Errorf("%d matches are booked to it, Run() reported %d", len(booked), summary.TournamentPlayed)
	}

	// Not everything: a draw with every pairing played offers no entry form,
	// which is half of what the schedule is for.
	if len(booked) >= tournament.Matches(len(tour.Players), tour.Format.Legs(), tour.WithFinal) {
		t.Error("every pairing is played, so the schedule offers nothing to enter")
	}

	for _, m := range booked {
		switch {
		case m.Status != domain.MatchConfirmed:
			t.Errorf("a tournament match is %q — the kiosk settles at once", m.Status)
		case m.EnteredVia != domain.EnteredViaKiosk:
			t.Errorf("entered_via = %q, want kiosk: somebody stood at the table", m.EnteredVia)
		case m.TournamentRound == nil:
			t.Error("a tournament match knows no round, so the schedule cannot place it")
		}
		// And whoever wrote it down is not playing in it, which is the rule
		// the kiosk enforces and a fixture must not quietly break.
		if m.ReportedBy == m.HomeID || m.ReportedBy == m.AwayID {
			t.Error("the result was credited to somebody playing in it")
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

	first := ratingsAfterSeed(ctx, t, store)

	if err := postgres.TruncateAll(ctx, store); err != nil {
		t.Fatalf("TruncateAll(): %v", err)
	}

	second := ratingsAfterSeed(ctx, t, store)

	if len(first) != len(second) {
		t.Fatalf("two runs produced %d and %d players", len(first), len(second))
	}
	for name, ttr := range first {
		if second[name] != ttr {
			t.Errorf("%s is on %d after the first run and %d after the second", name, ttr, second[name])
		}
	}
}

func ratingsAfterSeed(ctx context.Context, t *testing.T, store *postgres.Store) map[string]int {
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

// TestThePongFixtureWritesTheEightBotsAndTheirObserver is the contract with
// zaehlwerk-pong: its PONG_PLAYER_SKILL gives a strength by display name, so
// the names are checked one by one, and Pong has to be an observer, because
// that is how GET /api/operators offers it as the operator (docs/adr/0027).
func TestThePongFixtureWritesTheEightBotsAndTheirObserver(t *testing.T) {
	store, ctx := newStore(t)

	summary, err := seed.RunFixture(ctx, store, seed.FixturePong, reference)
	if err != nil {
		t.Fatalf("RunFixture(pong): %v", err)
	}
	if summary.Fixture != seed.FixturePong {
		t.Errorf("Summary.Fixture = %q, want %q", summary.Fixture, seed.FixturePong)
	}
	if summary.Players != 9 || summary.Observers != 1 {
		t.Errorf("Summary reports %d players and %d observers, want 9 and 1", summary.Players, summary.Observers)
	}

	playing, err := store.Players().Playing(ctx)
	if err != nil {
		t.Fatalf("Playing(): %v", err)
	}
	want := []string{"Ada", "Bert", "Cleo", "Dino", "Emil", "Frida", "Gero", "Hanna"}
	got := make(map[string]int, len(playing))
	for _, p := range playing {
		got[p.DisplayName]++
		// Everybody on the same rating, so whatever order the ranking ends up
		// in is the rating's doing.
		if p.TTR != domain.DefaultTTR {
			t.Errorf("%s starts on %d, want %d", p.DisplayName, p.TTR, domain.DefaultTTR)
		}
	}
	if len(playing) != len(want) {
		t.Errorf("%d players play, want exactly the %d bots", len(playing), len(want))
	}
	for _, name := range want {
		if got[name] != 1 {
			t.Errorf("the bot %s is there %d times, want once", name, got[name])
		}
	}

	everybody, err := store.Players().List(ctx)
	if err != nil {
		t.Fatalf("List(): %v", err)
	}
	var observers []string
	for _, p := range everybody {
		if p.IsObserver {
			observers = append(observers, p.DisplayName)
		}
	}
	if len(observers) != 1 || observers[0] != "Pong" {
		t.Errorf("the observers are %q, want exactly [Pong]", observers)
	}

	// No results and no tournament: the league fills itself.
	matches, err := store.Matches().Recent(ctx, 10)
	if err != nil {
		t.Fatalf("Recent(): %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("the pong fixture wrote %d matches, want none", len(matches))
	}
	tours, err := store.Tournaments().List(ctx, 10)
	if err != nil {
		t.Fatalf("Tournaments().List(): %v", err)
	}
	if len(tours) != 0 {
		t.Errorf("the pong fixture opened %d tournaments, want none", len(tours))
	}
}

// TestThePongFixtureRefusesADatabaseThatIsInUse is the same safety story as
// the demo's: a sandbox starts empty, and an office database is not one.
func TestThePongFixtureRefusesADatabaseThatIsInUse(t *testing.T) {
	store, ctx := newStore(t)

	if _, err := store.Players().Create(ctx, "Anna", 1000); err != nil {
		t.Fatalf("Create(): %v", err)
	}

	_, err := seed.RunFixture(ctx, store, seed.FixturePong, reference)
	if !errors.Is(err, seed.ErrNotEmpty) {
		t.Fatalf("RunFixture(pong) = %v, want ErrNotEmpty", err)
	}

	count, err := store.Players().Count(ctx)
	if err != nil {
		t.Fatalf("Count(): %v", err)
	}
	if count != 1 {
		t.Errorf("the refused run left %d players behind, want the 1 that was there", count)
	}
}

// TestThePongFixtureIsDeterministic: two sandboxes seeded from the same
// commit hold the same accounts on the same ratings with the same flags.
func TestThePongFixtureIsDeterministic(t *testing.T) {
	store, ctx := newStore(t)

	names := func() []string {
		t.Helper()
		if _, err := seed.RunFixture(ctx, store, seed.FixturePong, reference); err != nil {
			t.Fatalf("RunFixture(pong): %v", err)
		}
		players, err := store.Players().List(ctx)
		if err != nil {
			t.Fatalf("List(): %v", err)
		}
		out := make([]string, 0, len(players))
		for _, p := range players {
			out = append(out, fmt.Sprintf("%s/%d/%t", p.DisplayName, p.TTR, p.IsObserver))
		}
		return out
	}

	first := names()
	if err := postgres.TruncateAll(ctx, store); err != nil {
		t.Fatalf("TruncateAll(): %v", err)
	}
	second := names()

	if fmt.Sprint(first) != fmt.Sprint(second) {
		t.Errorf("two runs wrote %v and %v", first, second)
	}
}

// TestSeededPlayersWearTheirFigures: a preview should show what figures look
// like once people have chosen (docs/adr/0029), and show the same ones every
// time. Every player of the demo field and every bot gets the next figure
// from the fixture's own seeded sequence; Pong, the observer, gets none.
func TestSeededPlayersWearTheirFigures(t *testing.T) {
	for _, fixture := range []seed.Fixture{seed.FixtureDemo, seed.FixturePong} {
		t.Run(string(fixture), func(t *testing.T) {
			store, ctx := newStore(t)
			if _, err := seed.RunFixture(ctx, store, fixture, reference); err != nil {
				t.Fatalf("RunFixture(): %v", err)
			}
			players, err := store.Players().List(ctx)
			if err != nil {
				t.Fatal(err)
			}

			worn := map[avatar.Avatar]int{}
			dressed := 0
			for _, p := range players {
				a, err := store.Avatars().Get(ctx, p.ID)
				if p.IsObserver {
					if !errors.Is(err, domain.ErrNotFound) {
						t.Errorf("observer %s wears a figure: %+v, %v", p.DisplayName, a, err)
					}
					continue
				}
				if err != nil {
					t.Fatalf("%s has no figure: %v", p.DisplayName, err)
				}
				if err := avatar.Validate(a); err != nil {
					t.Errorf("%s wears an invalid figure: %v", p.DisplayName, err)
				}
				worn[a]++
				dressed++
			}

			// The same figures as the fixture's sequence rolls, in whatever
			// order List returns the players.
			figures := seed.Figures(fixture)
			for range dressed {
				worn[avatar.Random(figures)]--
			}
			for a, n := range worn {
				if n != 0 {
					t.Errorf("figure %+v worn %+d times more than the sequence rolls it", a, n)
				}
			}
		})
	}
}

// TestDressGivesTheUndressedAFigureAndLeavesTheRestAlone: the repair for a
// sandbox seeded before figures existed (docs/adr/0029). A player who never
// chose gets the figure their id rolls; a figure somebody chose stays, and so
// does an observer's default. Run again, it dresses nobody.
func TestDressGivesTheUndressedAFigureAndLeavesTheRestAlone(t *testing.T) {
	store, ctx := newStore(t)

	bare, err := store.Players().Create(ctx, "Bare", domain.DefaultTTR)
	if err != nil {
		t.Fatal(err)
	}
	chosen, err := store.Players().Create(ctx, "Chosen", domain.DefaultTTR)
	if err != nil {
		t.Fatal(err)
	}
	own := avatar.Avatar{Fur: avatar.Furs[1].Key, Headdress: avatar.Headdresses[1].Key,
		Handle: avatar.Handles[1].Key, Blade: avatar.Blades[1].Key, Bling: avatar.Blings[1].Key}
	if err := store.Avatars().Put(ctx, chosen.ID, own); err != nil {
		t.Fatal(err)
	}
	observer, err := store.Players().Create(ctx, "Pong", domain.DefaultTTR)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Players().SetObserver(ctx, observer.ID, true); err != nil {
		t.Fatal(err)
	}

	dressed, err := seed.Dress(ctx, store)
	if err != nil {
		t.Fatalf("Dress(): %v", err)
	}
	if dressed != 1 {
		t.Errorf("Dress() dressed %d players, want 1", dressed)
	}

	got, err := store.Avatars().Get(ctx, bare.ID)
	if err != nil {
		t.Fatalf("Bare has no figure: %v", err)
	}
	if want := avatar.Random(seed.FiguresFor(bare.ID)); got != want {
		t.Errorf("Bare wears %+v, want the figure their id rolls, %+v", got, want)
	}
	if got, err := store.Avatars().Get(ctx, chosen.ID); err != nil || got != own {
		t.Errorf("Chosen wears %+v (%v), want the figure they chose, %+v", got, err, own)
	}
	if _, err := store.Avatars().Get(ctx, observer.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("the observer was dressed: %v", err)
	}

	again, err := seed.Dress(ctx, store)
	if err != nil {
		t.Fatalf("Dress() again: %v", err)
	}
	if again != 0 {
		t.Errorf("a second Dress() dressed %d players, want 0", again)
	}
}
