package postgres_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
)

func TestTournamentRepository(t *testing.T) {
	store, ctx := newStore(t)
	players := store.Players()
	tournaments := store.Tournaments()

	field := make([]uuid.UUID, 0, 4)
	for _, name := range []string{"Anna", "Bodo", "Cleo", "Dilan"} {
		p, err := players.Create(ctx, name, domain.DefaultTTR)
		if err != nil {
			t.Fatalf("Create(%q): %v", name, err)
		}
		field = append(field, p.ID)
	}

	created, err := tournaments.Create(ctx, domain.Tournament{
		Name:      "Mittwochsrunde",
		CreatedBy: field[0],
		Players:   field,
	})
	if err != nil {
		t.Fatalf("Create(): %v", err)
	}

	// The defaults are the repository's job, so a caller that fills in only
	// what it cares about still gets a valid row rather than a constraint
	// violation.
	if created.Format != domain.TournamentRoundRobin {
		t.Errorf("format = %q, want %q", created.Format, domain.TournamentRoundRobin)
	}
	if created.Status != domain.TournamentOpen || !created.Open() {
		t.Errorf("status = %q, want open", created.Status)
	}
	if created.ClosedAt != nil {
		t.Errorf("ClosedAt = %v on a fresh tournament, want nil", created.ClosedAt)
	}

	// The order is the draw. A round trip that returns the field shuffled
	// would silently produce different pairings than the ones people were
	// shown when the tournament was made.
	loaded, err := tournaments.ByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("ByID(): %v", err)
	}
	if len(loaded.Players) != len(field) {
		t.Fatalf("ByID() returned %d players, want %d", len(loaded.Players), len(field))
	}
	for i, want := range field {
		if loaded.Players[i] != want {
			t.Errorf("player %d = %s, want %s — draw order is not preserved",
				i, loaded.Players[i], want)
		}
	}

	if _, err := tournaments.ByID(ctx, uuid.New()); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("ByID() for an unknown id = %v, want domain.ErrNotFound", err)
	}
}

// A match carries its bracket through the round trip, and a casual match
// carries none. Both halves matter: the second is every row that existed
// before this feature.
func TestMatchesCarryTheirTournament(t *testing.T) {
	store, ctx := newStore(t)

	anna, err := store.Players().Create(ctx, "Anna", domain.DefaultTTR)
	if err != nil {
		t.Fatalf("Create(): %v", err)
	}
	bodo, err := store.Players().Create(ctx, "Bodo", domain.DefaultTTR)
	if err != nil {
		t.Fatalf("Create(): %v", err)
	}

	tour, err := store.Tournaments().Create(ctx, domain.Tournament{
		Name: "Schnellturnier", CreatedBy: anna.ID, Players: []uuid.UUID{anna.ID, bodo.ID},
	})
	if err != nil {
		t.Fatalf("Create(): %v", err)
	}

	inTournament, err := store.Matches().Create(ctx, domain.Match{
		HomeID: anna.ID, AwayID: bodo.ID, BestOf: 3, PointsToWin: 11,
		ReportedBy: anna.ID, TournamentID: &tour.ID,
		Sets: []domain.MatchSet{
			{SetNo: 1, HomePoints: 11, AwayPoints: 5},
			{SetNo: 2, HomePoints: 11, AwayPoints: 7},
		},
	})
	if err != nil {
		t.Fatalf("Create() in tournament: %v", err)
	}
	if inTournament.TournamentID == nil || *inTournament.TournamentID != tour.ID {
		t.Errorf("TournamentID = %v, want %s", inTournament.TournamentID, tour.ID)
	}

	casual, err := store.Matches().Create(ctx, domain.Match{
		HomeID: anna.ID, AwayID: bodo.ID, BestOf: 3, PointsToWin: 11,
		ReportedBy: bodo.ID,
		Sets:       []domain.MatchSet{{SetNo: 1, HomePoints: 11, AwayPoints: 0}},
	})
	if err != nil {
		t.Fatalf("Create() casual: %v", err)
	}
	if casual.TournamentID != nil {
		t.Errorf("a casual match has TournamentID %v, want nil", casual.TournamentID)
	}

	// Reading it back is the half that would catch a column missing from
	// matchColumns, which a returning clause alone would not.
	reread, err := store.Matches().ByID(ctx, inTournament.ID)
	if err != nil {
		t.Fatalf("ByID(): %v", err)
	}
	if reread.TournamentID == nil || *reread.TournamentID != tour.ID {
		t.Errorf("ByID().TournamentID = %v, want %s", reread.TournamentID, tour.ID)
	}

	// The tournament sees its own match and only its own.
	booked, err := store.Tournaments().Matches(ctx, tour.ID)
	if err != nil {
		t.Fatalf("Matches(): %v", err)
	}
	if len(booked) != 1 {
		t.Fatalf("Matches() returned %d, want 1 — the casual match is leaking in", len(booked))
	}
	if booked[0].ID != inTournament.ID {
		t.Errorf("Matches()[0] = %s, want %s", booked[0].ID, inTournament.ID)
	}
	if len(booked[0].Sets) != 2 {
		t.Errorf("Matches()[0] has %d sets, want 2 — sets are not being loaded", len(booked[0].Sets))
	}
}

func TestClosingATournamentIsIdempotent(t *testing.T) {
	store, ctx := newStore(t)

	anna, err := store.Players().Create(ctx, "Anna", domain.DefaultTTR)
	if err != nil {
		t.Fatalf("Create(): %v", err)
	}

	tour, err := store.Tournaments().Create(ctx, domain.Tournament{
		Name: "Feierabend", CreatedBy: anna.ID, Players: []uuid.UUID{anna.ID},
	})
	if err != nil {
		t.Fatalf("Create(): %v", err)
	}

	first := time.Now().Truncate(time.Millisecond)
	if err := store.Tournaments().Close(ctx, tour.ID, first); err != nil {
		t.Fatalf("Close(): %v", err)
	}

	closed, err := store.Tournaments().ByID(ctx, tour.ID)
	if err != nil {
		t.Fatalf("ByID(): %v", err)
	}
	if closed.Open() || closed.Status != domain.TournamentClosed {
		t.Errorf("status = %q, want closed", closed.Status)
	}
	if closed.ClosedAt == nil {
		t.Fatal("ClosedAt is nil on a closed tournament")
	}
	was := *closed.ClosedAt

	// Two people pressing the same button is not a failure, and the second
	// press must not overwrite the honest timestamp of the first.
	if err := store.Tournaments().Close(ctx, tour.ID, first.Add(time.Hour)); err != nil {
		t.Fatalf("Close() twice: %v", err)
	}
	again, err := store.Tournaments().ByID(ctx, tour.ID)
	if err != nil {
		t.Fatalf("ByID(): %v", err)
	}
	if !again.ClosedAt.Equal(was) {
		t.Errorf("ClosedAt moved from %v to %v on a second close", was, *again.ClosedAt)
	}
}

// Open tournaments come first: at the table, the thing still being played is
// what somebody is looking for.
func TestListPutsOpenTournamentsFirst(t *testing.T) {
	store, ctx := newStore(t)

	anna, err := store.Players().Create(ctx, "Anna", domain.DefaultTTR)
	if err != nil {
		t.Fatalf("Create(): %v", err)
	}

	older, err := store.Tournaments().Create(ctx, domain.Tournament{
		Name: "Noch offen", CreatedBy: anna.ID, Players: []uuid.UUID{anna.ID},
	})
	if err != nil {
		t.Fatalf("Create(): %v", err)
	}
	newer, err := store.Tournaments().Create(ctx, domain.Tournament{
		Name: "Schon vorbei", CreatedBy: anna.ID, Players: []uuid.UUID{anna.ID},
	})
	if err != nil {
		t.Fatalf("Create(): %v", err)
	}
	if err := store.Tournaments().Close(ctx, newer.ID, time.Now()); err != nil {
		t.Fatalf("Close(): %v", err)
	}

	list, err := store.Tournaments().List(ctx, 10)
	if err != nil {
		t.Fatalf("List(): %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("List() returned %d, want 2", len(list))
	}
	if list[0].ID != older.ID {
		t.Errorf("List()[0] = %q, want the open one (%q)", list[0].Name, older.Name)
	}
	if len(list[0].Players) != 1 {
		t.Errorf("List() did not load the field: %d players", len(list[0].Players))
	}
}
