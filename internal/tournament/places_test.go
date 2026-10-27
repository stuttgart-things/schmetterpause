package tournament_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
	"github.com/stuttgart-things/schmetterpause/internal/tournament"
)

// playOut confirms every open group place and returns the matches booked so
// far. Whoever stands earlier in the field wins, so the table is a strict
// order and the final is the first two.
func playOut(f tournament.Field, booked []domain.Match) []domain.Match {
	for _, p := range tournament.OpenPlaces(f, booked) {
		if p.Final {
			continue
		}
		home, away := 11, 5
		if slices.Index(f.Players, p.Pairing.Away) < slices.Index(f.Players, p.Pairing.Home) {
			home, away = 5, 11
		}
		booked = append(booked, confirmedIn(p.Pairing.Home, p.Pairing.Away, p.Round, home, away))
	}
	return booked
}

func TestEveryPlaceOfAnUntouchedDrawIsOpenInDrawOrder(t *testing.T) {
	f := tournament.Field{Players: ids(4), Legs: 2}

	var want []tournament.Place
	for _, round := range tournament.Draw(f.Players, f.Legs) {
		for _, p := range round.Pairings {
			want = append(want, tournament.Place{Round: round.No, Pairing: p})
		}
	}

	got := tournament.OpenPlaces(f, nil)
	if len(got) != len(want) {
		t.Fatalf("got %d open places, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("place %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
}

// A pending result holds its place. Otherwise a second result would be
// entered for it while the first still waits for its confirmation.
func TestAPendingResultFillsItsPlace(t *testing.T) {
	f := tournament.Field{Players: ids(4), Legs: 1}
	first := tournament.OpenPlaces(f, nil)[0]

	m := confirmedIn(first.Pairing.Home, first.Pairing.Away, first.Round, 11, 5)
	m.Status = domain.MatchPending

	for _, p := range tournament.OpenPlaces(f, []domain.Match{m}) {
		if p == first {
			t.Fatal("a place with a pending result is still offered")
		}
	}
	err := tournament.CheckPlace(f, []domain.Match{m}, first.Pairing.Away, first.Pairing.Home, first.Round)
	if !errors.Is(err, tournament.ErrPlaceFilled) {
		t.Fatalf("got %v, want ErrPlaceFilled, whichever way round the pair is entered", err)
	}
}

// The return leg is the same pair in another round, and still open after the
// first leg is played.
func TestTheFirstLegFillsOnlyItsOwnRound(t *testing.T) {
	f := tournament.Field{Players: ids(2), Legs: 2}
	places := tournament.OpenPlaces(f, nil)
	if len(places) != 2 {
		t.Fatalf("two players over two legs: got %d places, want 2", len(places))
	}

	booked := []domain.Match{confirmedIn(places[0].Pairing.Home, places[0].Pairing.Away, places[0].Round, 11, 5)}
	open := tournament.OpenPlaces(f, booked)
	if len(open) != 1 || open[0] != places[1] {
		t.Fatalf("after the first leg: got %+v, want only %+v", open, places[1])
	}
}

// The final is offered only once every group match is confirmed, not as soon
// as somebody leads.
func TestTheFinalIsOpenOnlyOnceTheGroupIsPlayedOut(t *testing.T) {
	f := tournament.Field{Players: ids(3), Legs: 1, WithFinal: true}
	group := tournament.GroupRounds(3, 1)
	places := tournament.OpenPlaces(f, nil)

	booked := []domain.Match{confirmedIn(places[0].Pairing.Home, places[0].Pairing.Away, places[0].Round, 11, 5)}
	for _, p := range tournament.OpenPlaces(f, booked) {
		if p.Final {
			t.Fatal("the final is offered after one group match")
		}
	}
	if err := tournament.CheckPlace(f, booked, places[1].Pairing.Home, places[1].Pairing.Away, group+1); !errors.Is(err, tournament.ErrFinalUndecided) {
		t.Fatalf("got %v, want ErrFinalUndecided", err)
	}

	booked = playOut(f, booked)
	open := tournament.OpenPlaces(f, booked)
	if len(open) != 1 || !open[0].Final || open[0].Round != group+1 {
		t.Fatalf("after the group: got %+v, want only the final in round %d", open, group+1)
	}

	final := open[0].Pairing
	if err := tournament.CheckPlace(f, booked, final.Away, final.Home, group+1); err != nil {
		t.Fatalf("the finalists the other way round: got %v, want nil", err)
	}

	var third uuid.UUID
	for _, id := range f.Players {
		if id != final.Home && id != final.Away {
			third = id
		}
	}
	if err := tournament.CheckPlace(f, booked, final.Home, third, group+1); !errors.Is(err, tournament.ErrNotTheFinalists) {
		t.Fatalf("got %v, want ErrNotTheFinalists", err)
	}

	booked = append(booked, confirmedIn(final.Home, final.Away, group+1, 11, 9))
	if open := tournament.OpenPlaces(f, booked); len(open) != 0 {
		t.Fatalf("after the final: got %+v, want nothing open", open)
	}
	if err := tournament.CheckPlace(f, booked, final.Home, final.Away, group+1); !errors.Is(err, tournament.ErrPlaceFilled) {
		t.Fatalf("got %v, want ErrPlaceFilled", err)
	}
}

// A pending group result keeps the final closed: the table counts confirmed
// matches only, so it could still change who plays it.
func TestAPendingGroupResultKeepsTheFinalUndecided(t *testing.T) {
	f := tournament.Field{Players: ids(3), Legs: 1, WithFinal: true}
	booked := playOut(f, nil)
	booked[len(booked)-1].Status = domain.MatchPending

	for _, p := range tournament.OpenPlaces(f, booked) {
		if p.Final {
			t.Fatal("the final is offered while a group result is pending")
		}
	}
}

// Three players beating each other in a circle share first place, and a final
// between two of them would be chosen, not decided.
func TestATiedTopHasNoFinal(t *testing.T) {
	players := ids(3)
	f := tournament.Field{Players: players, Legs: 1, WithFinal: true}

	var booked []domain.Match
	for _, p := range tournament.OpenPlaces(f, nil) {
		winner, loser := p.Pairing.Home, p.Pairing.Away
		// a beats b, b beats c, c beats a, by the same margin.
		if (winner == players[1] && loser == players[0]) ||
			(winner == players[2] && loser == players[1]) ||
			(winner == players[0] && loser == players[2]) {
			winner, loser = loser, winner
		}
		booked = append(booked, confirmedIn(winner, loser, p.Round, 11, 5))
	}

	if open := tournament.OpenPlaces(f, booked); len(open) != 0 {
		t.Fatalf("got %+v, want no final for a tied top", open)
	}
}

func TestAPairTheDrawDoesNotHaveInThatRoundIsRefused(t *testing.T) {
	f := tournament.Field{Players: ids(4), Legs: 1}
	first := tournament.OpenPlaces(f, nil)[0]

	err := tournament.CheckPlace(f, nil, first.Pairing.Home, first.Pairing.Away, first.Round+1)
	if !errors.Is(err, tournament.ErrNotInDraw) {
		t.Fatalf("got %v, want ErrNotInDraw", err)
	}
	if err := tournament.CheckPlace(f, nil, first.Pairing.Home, first.Pairing.Away, 99); !errors.Is(err, tournament.ErrNotInDraw) {
		t.Fatalf("a round past the draw: got %v, want ErrNotInDraw", err)
	}
}

// Results from before rounds existed name their place by the pair, which is
// exact for the single round robins they all are.
func TestAResultFromBeforeRoundsFillsItsPairsPlace(t *testing.T) {
	f := tournament.Field{Players: ids(4), Legs: 1}
	first := tournament.OpenPlaces(f, nil)[0]
	booked := []domain.Match{confirmedIn(first.Pairing.Home, first.Pairing.Away, 0, 11, 5)}

	for _, p := range tournament.OpenPlaces(f, booked) {
		if p == first {
			t.Fatal("a pair with a result from before rounds is still offered")
		}
	}
}
