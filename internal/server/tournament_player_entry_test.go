package server_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
)

// A tournament used to need a laptop: only the unlocked machine could enter a
// result. Since ADR-0010 a player enters their own from their own device and
// the opponent confirms, exactly like a break-time match.
func TestAPlayerEntersTheirOwnTournamentResult(t *testing.T) {
	h, store, cookie := twoPlayers(t)
	anna, bodo := playerIDs(t, store, "Anna", "Bodo")
	id := seedTournamentMode(t, store, 3, 11, []uuid.UUID{anna, bodo})

	// The draw offers Anna a form to the ordinary entry endpoint.
	body := signedInDraw(t, h, cookie, id)
	if !strings.Contains(body, `action="/matches"`) {
		t.Fatal("the draw offers no form for the reader's own pairing")
	}
	if !strings.Contains(body, `name="tournament_id" value="`+id+`"`) {
		t.Error("the form does not say which tournament the result belongs to")
	}

	rec := postAs(t, h, "/matches", cookie, url.Values{
		"tournament_id": {id},
		"home_id":       {anna.String()},
		"away_id":       {bodo.String()},
		"best_of":       {"3"},
		"set_home_1":    {"11"}, "set_away_1": {"7"},
		"set_home_2": {"11"}, "set_away_2": {"9"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("entering = %d, want 303: %s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); loc != "/tournaments/"+id {
		t.Errorf("Location = %q, want the draw", loc)
	}

	booked := store.matches.all()
	if len(booked) != 1 {
		t.Fatalf("got %d matches, want 1", len(booked))
	}
	m := booked[0]
	switch {
	case m.TournamentID == nil || m.TournamentID.String() != id:
		t.Error("the result did not land in the tournament")
	case m.Status != domain.MatchPending:
		t.Errorf("status = %q, want pending — a player's result waits on the opponent", m.Status)
	case m.EnteredVia != domain.EnteredViaPlayer:
		t.Errorf("entered_via = %q, want player", m.EnteredVia)
	}
}

// Entering a result for two other people is what the machine at the table is
// for: it settles at once because somebody is standing there, which is exactly
// what nobody can check about a phone across the room.
func TestAPlayerCannotEnterSomebodyElsesPairing(t *testing.T) {
	h, store, cookie := twoPlayers(t)
	anna, bodo := playerIDs(t, store, "Anna", "Bodo")
	cesar, err := store.Players().Create(t.Context(), "Cesar", domain.DefaultTTR)
	if err != nil {
		t.Fatalf("creating Cesar: %v", err)
	}
	id := seedTournamentMode(t, store, 3, 11, []uuid.UUID{anna, bodo, cesar.ID})

	rec := postAs(t, h, "/matches", cookie, url.Values{
		"tournament_id": {id},
		"home_id":       {bodo.String()},
		"away_id":       {cesar.ID.String()},
		"best_of":       {"3"},
		"set_home_1":    {"11"}, "set_away_1": {"7"},
		"set_home_2": {"11"}, "set_away_2": {"9"},
	})
	if !strings.Contains(rec.Header().Get("Location"), "fehler=") {
		t.Error("a player booked a match they did not play")
	}
	if n := len(store.matches.all()); n != 0 {
		t.Errorf("got %d matches, want 0", n)
	}
}

// The pair still has to be in the field, and the tournament still has to be
// open. The player path uses the same check as the kiosk so the two cannot
// drift apart.
func TestThePlayerPathRefusesWhatTheKioskRefuses(t *testing.T) {
	h, store, cookie := twoPlayers(t)
	anna, bodo := playerIDs(t, store, "Anna", "Bodo")
	outsider, err := store.Players().Create(t.Context(), "Dora", domain.DefaultTTR)
	if err != nil {
		t.Fatalf("creating Dora: %v", err)
	}

	open := seedTournamentMode(t, store, 3, 11, []uuid.UUID{anna, bodo})
	result := url.Values{
		"best_of":    {"3"},
		"set_home_1": {"11"}, "set_away_1": {"7"},
		"set_home_2": {"11"}, "set_away_2": {"9"},
	}

	t.Run("an outsider in the pairing", func(t *testing.T) {
		form := cloneValues(result)
		form.Set("tournament_id", open)
		form.Set("home_id", anna.String())
		form.Set("away_id", outsider.ID.String())
		rec := postAs(t, h, "/matches", cookie, form)
		if !strings.Contains(rec.Header().Get("Location"), "fehler=") {
			t.Error("somebody outside the field was booked into the draw")
		}
	})

	t.Run("a tournament that is over", func(t *testing.T) {
		closed := seedTournamentClosed(t, store, []uuid.UUID{anna, bodo})
		form := cloneValues(result)
		form.Set("tournament_id", closed)
		form.Set("home_id", anna.String())
		form.Set("away_id", bodo.String())
		rec := postAs(t, h, "/matches", cookie, form)
		if !strings.Contains(rec.Header().Get("Location"), "fehler=") {
			t.Error("a result landed in a closed tournament")
		}
	})

	if n := len(store.matches.all()); n != 0 {
		t.Errorf("got %d matches, want 0", n)
	}
}

// The mode is the tournament's. A form claiming otherwise is the one part of
// this a caller can edit.
func TestThePlayerPathTakesTheModeFromTheTournament(t *testing.T) {
	h, store, cookie := twoPlayers(t)
	anna, bodo := playerIDs(t, store, "Anna", "Bodo")
	id := seedTournamentMode(t, store, 1, 21, []uuid.UUID{anna, bodo})

	rec := postAs(t, h, "/matches", cookie, url.Values{
		"tournament_id": {id},
		"home_id":       {anna.String()},
		"away_id":       {bodo.String()},
		"best_of":       {"3"},
		"points_to_win": {"11"},
		"set_home_1":    {"21"}, "set_away_1": {"18"},
	})
	if loc := rec.Header().Get("Location"); strings.Contains(loc, "fehler=") {
		t.Fatalf("a single set to 21 was refused: %s", loc)
	}

	booked := store.matches.all()
	if len(booked) != 1 {
		t.Fatalf("got %d matches, want 1", len(booked))
	}
	if booked[0].BestOf != 1 || booked[0].PointsToWin != 21 {
		t.Errorf("mode = best of %d to %d, want 1 to 21 — the form was believed",
			booked[0].BestOf, booked[0].PointsToWin)
	}
}

// Somebody who is not in the field reads the draw and is offered nothing.
func TestTheDrawOffersNothingToSomebodyNotPlaying(t *testing.T) {
	h, store, cookie := twoPlayers(t)
	_, bodo := playerIDs(t, store, "Anna", "Bodo")
	cesar, err := store.Players().Create(t.Context(), "Cesar", domain.DefaultTTR)
	if err != nil {
		t.Fatalf("creating Cesar: %v", err)
	}
	id := seedTournamentMode(t, store, 3, 11, []uuid.UUID{bodo, cesar.ID})

	body := signedInDraw(t, h, cookie, id)
	if strings.Contains(body, `action="/matches"`) {
		t.Error("the draw offers a form to somebody who is not in the field")
	}
}

func playerIDs(t *testing.T, store *memStore, names ...string) (uuid.UUID, uuid.UUID) {
	t.Helper()

	ids := kioskPlayers(t, store, names...)
	first, err := uuid.Parse(ids[0])
	if err != nil {
		t.Fatalf("parsing %s: %v", ids[0], err)
	}
	second, err := uuid.Parse(ids[1])
	if err != nil {
		t.Fatalf("parsing %s: %v", ids[1], err)
	}
	return first, second
}

func seedTournamentClosed(t *testing.T, store *memStore, field []uuid.UUID) string {
	t.Helper()

	closed := timeNow()
	created, err := store.tournaments.Create(t.Context(), domain.Tournament{
		Name: "Vorbei", Format: domain.TournamentRoundRobin,
		Status: domain.TournamentClosed, ClosedAt: &closed,
		CreatedBy: field[0], BestOf: 3, PointsToWin: 11, Players: field,
	})
	if err != nil {
		t.Fatalf("seeding the closed tournament: %v", err)
	}
	return created.ID.String()
}

func signedInDraw(t *testing.T, h http.Handler, cookie *http.Cookie, id string) string {
	t.Helper()

	rec := fragment(t, h, "/tournaments/"+id, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET the draw = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

func postAs(t *testing.T, h http.Handler, path string, cookie *http.Cookie, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	return kioskPost(t, h, path, cookie, form)
}

func cloneValues(v url.Values) url.Values {
	out := url.Values{}
	for k, vs := range v {
		out[k] = append([]string(nil), vs...)
	}
	return out
}
