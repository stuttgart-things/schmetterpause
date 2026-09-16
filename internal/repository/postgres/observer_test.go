package postgres_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
)

// An observer does not play (docs/adr/0022): out of the ranking and the
// pickers, refused as a side of a match and in a tournament field — and the
// flag itself only goes on somebody with nothing to lose from it.
func TestAnObserverDoesNotPlay(t *testing.T) {
	store, ctx := newStore(t)
	players := store.Players()

	anna := mustPlayer(ctx, t, store, "Anna", domain.DefaultTTR)
	bodo := mustPlayer(ctx, t, store, "Bodo", domain.DefaultTTR)
	timo := mustPlayer(ctx, t, store, "timoboll", domain.DefaultTTR)

	if timo.IsObserver {
		t.Fatal("Create() made an observer, want everybody playing by default")
	}
	if err := players.SetObserver(ctx, timo.ID, true); err != nil {
		t.Fatalf("SetObserver(): %v", err)
	}

	back, err := players.ByID(ctx, timo.ID)
	if err != nil || !back.IsObserver {
		t.Errorf("ByID() = observer %v, %v, want an observer", back.IsObserver, err)
	}

	// List is everybody, because signing in picks from it.
	all, err := players.List(ctx)
	if err != nil {
		t.Fatalf("List(): %v", err)
	}
	if len(all) != 3 {
		t.Errorf("List() has %d players, want all 3", len(all))
	}

	playing, err := players.Playing(ctx)
	if err != nil {
		t.Fatalf("Playing(): %v", err)
	}
	if len(playing) != 2 || containsPlayer(playing, timo.ID) {
		t.Errorf("Playing() = %v, want Anna and Bodo only", names(playing))
	}

	records, err := players.Records(ctx)
	if err != nil {
		t.Fatalf("Records(): %v", err)
	}
	for _, r := range records {
		if r.Player.ID == timo.ID {
			t.Error("Records() includes the observer, want them out of the ranking")
		}
	}

	sets := []domain.MatchSet{{SetNo: 1, HomePoints: 11, AwayPoints: 5}, {SetNo: 2, HomePoints: 11, AwayPoints: 7}}
	for _, m := range []domain.Match{
		{HomeID: timo.ID, AwayID: anna.ID},
		{HomeID: anna.ID, AwayID: timo.ID},
	} {
		m.BestOf, m.PointsToWin, m.ReportedBy, m.Sets = 3, 11, anna.ID, sets
		if _, err := store.Matches().Create(ctx, m); !errors.Is(err, domain.ErrObserver) {
			t.Errorf("Create() with the observer on a side = %v, want domain.ErrObserver", err)
		}
	}

	// Reporting for others is not playing: an observer holding the pen is
	// what ADR-0014 asks of an operator.
	if _, err := store.Matches().Create(ctx, domain.Match{
		HomeID: anna.ID, AwayID: bodo.ID, BestOf: 3, PointsToWin: 11,
		ReportedBy: timo.ID, EnteredVia: domain.EnteredViaScoreboard, Sets: sets,
	}); err != nil {
		t.Errorf("Create() with the observer as reporter: %v, want it stored", err)
	}

	tournaments, err := store.Tournaments().List(ctx, 10)
	if err != nil {
		t.Fatalf("Tournaments().List(): %v", err)
	}
	if _, err := store.Tournaments().Create(ctx, domain.Tournament{
		Name: "Feierabend", CreatedBy: timo.ID, Players: []uuid.UUID{anna.ID, timo.ID},
	}); !errors.Is(err, domain.ErrObserver) {
		t.Errorf("Tournaments().Create() with the observer in the field = %v, want domain.ErrObserver", err)
	}
	after, err := store.Tournaments().List(ctx, 10)
	if err != nil {
		t.Fatalf("Tournaments().List(): %v", err)
	}
	if len(after) != len(tournaments) {
		t.Error("a refused field left a tournament behind")
	}

	// A tournament an observer only created is fine, and so is editing one
	// as long as the field stays without them.
	tour, err := store.Tournaments().Create(ctx, domain.Tournament{
		Name: "Feierabend", CreatedBy: timo.ID, Players: []uuid.UUID{anna.ID, bodo.ID},
	})
	if err != nil {
		t.Fatalf("Tournaments().Create() by the observer: %v", err)
	}
	tour.Players = []uuid.UUID{bodo.ID, timo.ID}
	if _, err := store.Tournaments().Replace(ctx, tour); !errors.Is(err, domain.ErrObserver) {
		t.Errorf("Tournaments().Replace() with the observer in the field = %v, want domain.ErrObserver", err)
	}

	// Letting them play again always works, and they come back as they were.
	if err := players.SetObserver(ctx, timo.ID, false); err != nil {
		t.Fatalf("SetObserver(false): %v", err)
	}
	playing, _ = players.Playing(ctx)
	if !containsPlayer(playing, timo.ID) {
		t.Error("Playing() still leaves out somebody who plays again")
	}

	if err := players.SetObserver(ctx, uuid.New(), true); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("SetObserver() for an unknown player = %v, want domain.ErrNotFound", err)
	}
}

// Only somebody who never took part may become an observer: a history would
// disappear from other people's ranking with them.
func TestTheObserverFlagRefusesAHistory(t *testing.T) {
	store, ctx := newStore(t)
	players := store.Players()

	anna := mustPlayer(ctx, t, store, "Anna", domain.DefaultTTR)
	bodo := mustPlayer(ctx, t, store, "Bodo", domain.DefaultTTR)
	cleo := mustPlayer(ctx, t, store, "Cleo", domain.DefaultTTR)
	dora := mustPlayer(ctx, t, store, "Dora", domain.DefaultTTR)

	// A pending match counts as much as a confirmed one: it may still be.
	if _, err := store.Matches().Create(ctx, domain.Match{
		HomeID: anna.ID, AwayID: bodo.ID, BestOf: 3, PointsToWin: 11, ReportedBy: cleo.ID,
		Status: domain.MatchPending,
		Sets:   []domain.MatchSet{{SetNo: 1, HomePoints: 11, AwayPoints: 5}, {SetNo: 2, HomePoints: 11, AwayPoints: 7}},
	}); err != nil {
		t.Fatalf("Create(): %v", err)
	}
	if _, err := store.Tournaments().Create(ctx, domain.Tournament{
		Name: "Feierabend", CreatedBy: anna.ID, Players: []uuid.UUID{dora.ID, anna.ID},
	}); err != nil {
		t.Fatalf("Tournaments().Create(): %v", err)
	}

	for _, p := range []domain.Player{anna, bodo, dora} {
		if err := players.SetObserver(ctx, p.ID, true); !errors.Is(err, domain.ErrInUse) {
			t.Errorf("SetObserver(%s) = %v, want domain.ErrInUse", p.DisplayName, err)
		}
	}

	// Having reported somebody else's result is not having played.
	if err := players.SetObserver(ctx, cleo.ID, true); err != nil {
		t.Errorf("SetObserver() for somebody who only reported: %v, want it set", err)
	}
}

func containsPlayer(players []domain.Player, id uuid.UUID) bool {
	for _, p := range players {
		if p.ID == id {
			return true
		}
	}
	return false
}

func names(players []domain.Player) []string {
	out := make([]string, 0, len(players))
	for _, p := range players {
		out = append(out, p.DisplayName)
	}
	return out
}
