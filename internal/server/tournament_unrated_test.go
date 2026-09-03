package server_test

import (
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
)

// The Friday afternoon tournament: it happens, and it leaves the ranking
// where it was (docs/adr/0012).
func TestAnUnratedTournamentLeavesTheRatingAlone(t *testing.T) {
	h, store := kioskHandler(t)
	cookie := unlock(t, h)
	field := seedField(t, store)
	id := seedRated(t, store, field, false)

	before := ttrOfPlayer(t, store, "Anna")

	rec := kioskPost(t, h, "/kiosk/tournaments/"+id+"/matches", cookie, url.Values{
		"home_id": {field[0].String()}, "away_id": {field[1].String()},
		"tournament_round": {"1"},
		"set_home_1":       {"11"}, "set_away_1": {"5"},
		"set_home_2": {"11"}, "set_away_2": {"7"},
	})
	if loc := rec.Header().Get("Location"); strings.Contains(loc, "fehler=") {
		t.Fatalf("the entry was refused: %s", loc)
	}

	if after := ttrOfPlayer(t, store, "Anna"); after != before {
		t.Errorf("the winner moved from %d to %d in a tournament that does not count", before, after)
	}
	// It still happened. Unrated is about the rating and nothing else: the
	// match is confirmed, and the tournament table counts it like any other
	// (docs/adr/0012).
	body := listBody(t, h, "/tournaments/"+id)
	if !strings.Contains(body, "<td>Anna</td><td>1</td><td>0</td>") {
		t.Errorf("the table does not count the match that was played: %s", body)
	}
}

// And the same evening when it does count, so the test above cannot pass
// because something else stopped rating altogether.
func TestARatedTournamentStillMovesTheRating(t *testing.T) {
	h, store := kioskHandler(t)
	cookie := unlock(t, h)
	field := seedField(t, store)
	id := seedRated(t, store, field, true)

	before := ttrOfPlayer(t, store, "Anna")

	kioskPost(t, h, "/kiosk/tournaments/"+id+"/matches", cookie, url.Values{
		"home_id": {field[0].String()}, "away_id": {field[1].String()},
		"tournament_round": {"1"},
		"set_home_1":       {"11"}, "set_away_1": {"5"},
		"set_home_2": {"11"}, "set_away_2": {"7"},
	})

	if after := ttrOfPlayer(t, store, "Anna"); after <= before {
		t.Errorf("the winner went from %d to %d in a tournament that counts", before, after)
	}
}

// Nobody should have to open a tournament to find out that it does not count.
func TestAnUnratedTournamentSaysSo(t *testing.T) {
	h, store := kioskHandler(t)
	field := seedField(t, store)
	id := seedRated(t, store, field, false)

	if page := listBody(t, h, "/tournaments/"+id); !strings.Contains(page, "Zählt nicht für die Rangliste") {
		t.Errorf("the tournament page does not say it is unrated: %s", page)
	}
	// The list is where somebody chooses which of two open ones to walk
	// over to.
	if list := listBody(t, h, "/tournaments"); !strings.Contains(list, "ohne Wertung") {
		t.Errorf("the list does not mark the unrated tournament: %s", list)
	}
	// And a rated one says nothing, because that is the ordinary case.
	rated := seedRated(t, store, field, true)
	if page := listBody(t, h, "/tournaments/"+rated); strings.Contains(page, "Zählt nicht") {
		t.Errorf("a tournament that counts claims it does not: %s", page)
	}
}

// The form asks the question the other way round: nothing ticked has to mean
// the default, and the default is that it counts.
func TestTheFormOptsOutRatherThanIn(t *testing.T) {
	h, store := kioskHandler(t)
	seedField(t, store)

	body := listBody(t, h, "/tournaments")
	if !strings.Contains(body, `name="unrated"`) {
		t.Errorf("the form has no way to say a tournament does not count: %s", body)
	}
	// A fresh form counts, so the box is not ticked. The inversion is the
	// whole point: an unticked checkbox sends nothing, and nothing has to
	// land on the default.
	unrated := strings.Index(body, `name="unrated"`)
	if end := strings.Index(body[unrated:], ">"); strings.Contains(body[unrated:unrated+end], "checked") {
		t.Errorf("a fresh form opts out by default: %s", body[unrated:unrated+end])
	}
}

func seedRated(t *testing.T, store *memStore, field []uuid.UUID, rated bool) string {
	t.Helper()

	created, err := store.tournaments.Create(t.Context(), domain.Tournament{
		Name: "Freitag", Format: domain.TournamentRoundRobin,
		Status: domain.TournamentOpen, CreatedBy: field[0],
		BestOf: 3, PointsToWin: 11, Rated: rated, Players: field,
	})
	if err != nil {
		t.Fatalf("seeding the tournament: %v", err)
	}
	return created.ID.String()
}
