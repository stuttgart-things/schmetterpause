package server_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
)

// The mode is one agreement about how the evening is played, so it is asked
// once at the draw. Before this it was hardcoded to best of three up to
// eleven, and the schedule could not say what had been played.
func TestATournamentKeepsTheModeItWasStartedIn(t *testing.T) {
	h, store := kioskHandler(t)
	field := seedField(t, store)

	rec := postTournament(t, h, url.Values{
		"name":          {"Kurze Sätze"},
		"best_of":       {"5"},
		"points_to_win": {"21"},
		"player_id":     {field[0].String(), field[1].String()},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("creating = %d, want 303: %s", rec.Code, rec.Body.String())
	}

	tours, err := store.tournaments.List(t.Context(), 10)
	if err != nil {
		t.Fatalf("List(): %v", err)
	}
	if len(tours) != 1 {
		t.Fatalf("got %d tournaments, want 1", len(tours))
	}
	if got := tours[0].BestOf; got != 5 {
		t.Errorf("BestOf = %d, want 5", got)
	}
	if got := tours[0].PointsToWin; got != 21 {
		t.Errorf("PointsToWin = %d, want 21", got)
	}
}

// A mode nothing may be played under is a draw that refuses every entry, so
// it is caught at the draw rather than twenty-eight times afterwards.
func TestATournamentRefusesAModeThatDoesNotExist(t *testing.T) {
	h, store := kioskHandler(t)
	field := seedField(t, store)

	for _, tc := range []struct{ name, bestOf, points string }{
		{"four sets", "4", "11"},
		{"no sets", "0", "11"},
		{"to fifteen", "3", "15"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := postTournament(t, h, url.Values{
				"name":          {"Kaputt"},
				"best_of":       {tc.bestOf},
				"points_to_win": {tc.points},
				"player_id":     {field[0].String(), field[1].String()},
			})
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("got %d, want 422: %s", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), "Diesen Modus gibt es nicht.") {
				t.Error("the refusal does not say what was wrong")
			}
		})
	}

	if tours, _ := store.tournaments.List(t.Context(), 10); len(tours) != 0 {
		t.Errorf("a refused draw was stored anyway: %d", len(tours))
	}
}

// One box per set the mode allows. Boxes for sets that cannot exist are boxes
// somebody eventually types into, and at best of one there is exactly one.
func TestTheDrawOffersOneBoxPerSet(t *testing.T) {
	for _, bestOf := range []int{1, 3, 5, 7} {
		t.Run(strconv.Itoa(bestOf), func(t *testing.T) {
			h, store := kioskHandler(t)
			cookie := unlock(t, h, store)
			field := seedField(t, store)

			id := seedTournamentMode(t, store, bestOf, 11, field)
			body := drawBody(t, h, cookie, id)

			for i := 1; i <= bestOf; i++ {
				if !strings.Contains(body, `name="set_home_`+strconv.Itoa(i)+`"`) {
					t.Errorf("set %d has no box", i)
				}
			}
			if strings.Contains(body, `name="set_home_`+strconv.Itoa(bestOf+1)+`"`) {
				t.Errorf("there is a box for set %d, which cannot be played", bestOf+1)
			}
			if !strings.Contains(body, `value="`+strconv.Itoa(bestOf)+`"`) {
				t.Error("the form does not carry the tournament's mode")
			}
		})
	}
}

// The form is the one part of this a caller can edit, so the server takes the
// mode from the tournament. Otherwise a result lands in a draw under a mode
// the draw was never played in, and the table says something untrue.
func TestAResultTakesTheModeFromTheTournament(t *testing.T) {
	h, store := kioskHandler(t)
	cookie := unlock(t, h, store)
	field := seedField(t, store)

	id := seedTournamentMode(t, store, 1, 11, field)

	// A form claiming best of three, with three sets typed into it.
	rec := kioskPost(t, h, "/kiosk/tournaments/"+id+"/matches", cookie, url.Values{
		"home_id":          {field[0].String()},
		"away_id":          {field[1].String()},
		"tournament_round": {"1"},
		"best_of":          {"3"},
		"set_home_1":       {"11"},
		"set_away_1":       {"5"},
		"set_home_2":       {"5"},
		"set_away_2":       {"11"},
		"set_home_3":       {"11"},
		"set_away_3":       {"7"},
		"points_to_win":    {"11"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("got %d, want 303: %s", rec.Code, rec.Body.String())
	}
	// Three sets in a single-set tournament is too many, and the redirect
	// carries the complaint rather than booking it.
	if !strings.Contains(rec.Header().Get("Location"), "fehler=") {
		t.Fatal("three sets were accepted into a single-set tournament")
	}

	// The honest version of the same match does land, under best of one.
	rec = kioskPost(t, h, "/kiosk/tournaments/"+id+"/matches", cookie, url.Values{
		"home_id":          {field[0].String()},
		"away_id":          {field[1].String()},
		"tournament_round": {"1"},
		"best_of":          {"3"},
		"set_home_1":       {"11"},
		"set_away_1":       {"5"},
	})
	if loc := rec.Header().Get("Location"); strings.Contains(loc, "fehler=") {
		t.Fatalf("a single set was refused: %s", loc)
	}

	booked := store.matches.all()
	if len(booked) != 1 {
		t.Fatalf("got %d matches, want 1", len(booked))
	}
	if got := booked[0].BestOf; got != 1 {
		t.Errorf("BestOf = %d, want 1 — the form's 3 was believed", got)
	}
}

func postTournament(t *testing.T, h http.Handler, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	return kioskPost(t, h, "/tournaments", nil, form)
}

// seedTournamentMode stores an open tournament in the given mode.
func seedTournamentMode(t *testing.T, store *memStore, bestOf, points int, field []uuid.UUID) string {
	t.Helper()

	created, err := store.tournaments.Create(t.Context(), domain.Tournament{
		Name:        "Modus " + strconv.Itoa(bestOf),
		Format:      domain.TournamentRoundRobin,
		Status:      domain.TournamentOpen,
		CreatedBy:   field[0],
		BestOf:      bestOf,
		PointsToWin: points,
		Players:     field,
	})
	if err != nil {
		t.Fatalf("seeding the tournament: %v", err)
	}
	return created.ID.String()
}

// drawBody fetches the kiosk copy of a draw.
func drawBody(t *testing.T, h http.Handler, cookie *http.Cookie, id string) string {
	t.Helper()

	rec := fragment(t, h, "/kiosk/tournaments/"+id, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET the draw = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}
