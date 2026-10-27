package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
)

// Tournaments through the Zählwerk (docs/adr/0028): it reads the open places
// of a draw and reports a result into one of them, checked the way the kiosk
// checks it.

// apiTournamentOf stores an open tournament between the given players, best of
// three to eleven, and returns it.
func apiTournamentOf(t *testing.T, store *memStore, name string, withFinal bool,
	createdAt time.Time, players ...domain.Player,
) domain.Tournament {
	t.Helper()

	field := make([]uuid.UUID, 0, len(players))
	for _, p := range players {
		field = append(field, p.ID)
	}
	tour, err := store.tournaments.Create(t.Context(), domain.Tournament{
		Name:        name,
		Format:      domain.TournamentRoundRobin,
		CreatedBy:   field[0],
		CreatedAt:   createdAt,
		BestOf:      3,
		PointsToWin: 11,
		WithFinal:   withFinal,
		Rated:       true,
		SideA:       "Fenster",
		SideB:       "Tür",
		Players:     field,
	})
	if err != nil {
		t.Fatalf("creating the tournament: %v", err)
	}
	return tour
}

func apiGet(t *testing.T, h http.Handler, path string, out any) int {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer "+testScoreboardToken)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code == http.StatusOK && out != nil {
		if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
			t.Fatalf("decoding %s: %v: %s", path, err, rec.Body.String())
		}
	}
	return rec.Code
}

type placeOnTheWire struct {
	Round    int       `json:"round"`
	Final    bool      `json:"final"`
	HomeID   uuid.UUID `json:"home_id"`
	AwayID   uuid.UUID `json:"away_id"`
	HomeName string    `json:"home_name"`
	AwayName string    `json:"away_name"`
}

func openPlaces(t *testing.T, h http.Handler, tour domain.Tournament) []placeOnTheWire {
	t.Helper()

	var places []placeOnTheWire
	if code := apiGet(t, h, "/api/tournaments/"+tour.ID.String()+"/pairings", &places); code != http.StatusOK {
		t.Fatalf("GET pairings = %d, want 200", code)
	}
	return places
}

// placeResult is a result for a place, home winning 2:0, counted by operator.
func placeResult(tour domain.Tournament, p placeOnTheWire, operator domain.Player) map[string]any {
	return map[string]any{
		"home_id":       p.HomeID,
		"away_id":       p.AwayID,
		"operator_id":   operator.ID,
		"sets":          [][2]int{{11, 9}, {11, 7}},
		"best_of":       3,
		"points_to_win": 11,
		"tournament_id": tour.ID,
		"round":         p.Round,
	}
}

func fourPlayers(t *testing.T, store *memStore) (players []domain.Player, operator domain.Player) {
	t.Helper()

	for _, name := range []string{"Ada", "Bert", "Cleo", "Dino", "Pong"} {
		p, err := store.Players().Create(t.Context(), name, domain.DefaultTTR)
		if err != nil {
			t.Fatalf("creating %s: %v", name, err)
		}
		players = append(players, p)
	}
	return players[:4], players[4]
}

func TestTheZaehlwerkSeesTheOpenTournamentsOldestFirstWithTheirMode(t *testing.T) {
	h, store := scoreboardHandler(t)
	players, _ := fourPlayers(t, store)

	older := apiTournamentOf(t, store, "Montag", true, time.Now().Add(-time.Hour), players...)
	newer := apiTournamentOf(t, store, "Dienstag", false, time.Now(), players[:2]...)
	closed := apiTournamentOf(t, store, "Vorbei", false, time.Now().Add(-2*time.Hour), players[:2]...)
	if err := store.Tournaments().Close(t.Context(), closed.ID, time.Now()); err != nil {
		t.Fatalf("closing: %v", err)
	}

	var got []struct {
		ID          uuid.UUID `json:"id"`
		Name        string    `json:"name"`
		BestOf      int       `json:"best_of"`
		PointsToWin int       `json:"points_to_win"`
		SideA       string    `json:"side_a"`
		SideB       string    `json:"side_b"`
		Rated       bool      `json:"rated"`
		Matches     int       `json:"matches"`
		Played      int       `json:"played"`
	}
	if code := apiGet(t, h, "/api/tournaments", &got); code != http.StatusOK {
		t.Fatalf("GET /api/tournaments = %d, want 200", code)
	}

	if len(got) != 2 || got[0].ID != older.ID || got[1].ID != newer.ID {
		t.Fatalf("got %+v, want Montag then Dienstag and nothing closed", got)
	}
	m := got[0]
	if m.Name != "Montag" || m.BestOf != 3 || m.PointsToWin != 11 || m.SideA != "Fenster" ||
		m.SideB != "Tür" || !m.Rated || m.Matches != 7 || m.Played != 0 {
		t.Errorf("Montag reads %+v, want best of 3 to 11, Fenster/Tür, rated, 7 matches, none played", m)
	}
}

func TestThePairingsAreTheOpenPlacesInDrawOrderWithNames(t *testing.T) {
	h, store := scoreboardHandler(t)
	players, operator := fourPlayers(t, store)
	tour := apiTournamentOf(t, store, "Montag", false, time.Now(), players...)

	places := openPlaces(t, h, tour)
	if len(places) != 6 {
		t.Fatalf("four players: got %d places, want 6", len(places))
	}
	for i, p := range places {
		if p.HomeName == "" || p.AwayName == "" || p.Final {
			t.Errorf("place %d reads %+v, want two names and no final", i, p)
		}
		if i > 0 && p.Round < places[i-1].Round {
			t.Errorf("place %d is in round %d after round %d", i, p.Round, places[i-1].Round)
		}
	}

	rec := apiPost(t, h, testScoreboardToken, placeResult(tour, places[0], operator))
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST = %d, want 201: %s", rec.Code, rec.Body.String())
	}
	after := openPlaces(t, h, tour)
	if len(after) != 5 || after[0] != places[1] {
		t.Fatalf("after one result: got %+v, want the other five in order", after)
	}
}

func TestAResultForAPlaceIsStoredInItAndASecondOneIsAConflict(t *testing.T) {
	h, store := scoreboardHandler(t)
	players, operator := fourPlayers(t, store)
	tour := apiTournamentOf(t, store, "Montag", false, time.Now(), players...)
	place := openPlaces(t, h, tour)[0]

	rec := apiPost(t, h, testScoreboardToken, placeResult(tour, place, operator))
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST = %d, want 201: %s", rec.Code, rec.Body.String())
	}
	stored, err := store.Matches().ByID(t.Context(), createdMatch(t, rec.Body.Bytes()))
	if err != nil {
		t.Fatalf("reading the match: %v", err)
	}
	if stored.TournamentID == nil || *stored.TournamentID != tour.ID ||
		stored.TournamentRound == nil || *stored.TournamentRound != place.Round {
		t.Errorf("stored in tournament %v round %v, want %s round %d",
			stored.TournamentID, stored.TournamentRound, tour.ID, place.Round)
	}
	if stored.Status != domain.MatchPending || stored.EnteredVia != domain.EnteredViaScoreboard {
		t.Errorf("stored %s via %s, want pending via scoreboard", stored.Status, stored.EnteredVia)
	}

	// The retry of a result that arrived, or the same place entered at the
	// kiosk meanwhile.
	again := apiPost(t, h, testScoreboardToken, placeResult(tour, place, operator))
	if again.Code != http.StatusConflict {
		t.Fatalf("the same place again = %d, want 409: %s", again.Code, again.Body.String())
	}
}

func TestAPlaceTheDrawDoesNotOfferIsRefused(t *testing.T) {
	h, store := scoreboardHandler(t)
	players, operator := fourPlayers(t, store)
	tour := apiTournamentOf(t, store, "Montag", true, time.Now(), players...)
	place := openPlaces(t, h, tour)[0]

	for name, tc := range map[string]struct {
		change func(map[string]any)
		want   int
	}{
		"a tournament without a round": {func(b map[string]any) { delete(b, "round") }, http.StatusBadRequest},
		"a round without a tournament": {func(b map[string]any) { delete(b, "tournament_id") }, http.StatusBadRequest},
		"round zero":                   {func(b map[string]any) { b["round"] = 0 }, http.StatusBadRequest},
		"an unknown tournament":        {func(b map[string]any) { b["tournament_id"] = uuid.New() }, http.StatusUnprocessableEntity},
		"another round":                {func(b map[string]any) { b["round"] = place.Round + 1 }, http.StatusUnprocessableEntity},
		"the final before the group":   {func(b map[string]any) { b["round"] = 4 }, http.StatusUnprocessableEntity},
		"another mode": {func(b map[string]any) {
			b["best_of"], b["sets"] = 1, [][2]int{{11, 9}}
		}, http.StatusUnprocessableEntity},
		"somebody outside the field": {func(b map[string]any) { b["away_id"] = operator.ID; b["operator_id"] = players[2].ID },
			http.StatusUnprocessableEntity},
	} {
		body := placeResult(tour, place, operator)
		tc.change(body)
		rec := apiPost(t, h, testScoreboardToken, body)
		if rec.Code != tc.want {
			t.Errorf("%s: got %d, want %d: %s", name, rec.Code, tc.want, rec.Body.String())
		}
	}

	if got := len(openPlaces(t, h, tour)); got != 6 {
		t.Errorf("after the refusals %d places are open, want all 6", got)
	}
}

func TestAClosedTournamentTakesNoResultAndOffersNoPlaces(t *testing.T) {
	h, store := scoreboardHandler(t)
	players, operator := fourPlayers(t, store)
	tour := apiTournamentOf(t, store, "Montag", false, time.Now(), players...)
	place := openPlaces(t, h, tour)[0]
	if err := store.Tournaments().Close(t.Context(), tour.ID, time.Now()); err != nil {
		t.Fatalf("closing: %v", err)
	}

	if rec := apiPost(t, h, testScoreboardToken, placeResult(tour, place, operator)); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("POST into a closed tournament = %d, want 422", rec.Code)
	}
	if places := openPlaces(t, h, tour); len(places) != 0 {
		t.Errorf("a closed tournament offers %+v, want nothing", places)
	}
	if code := apiGet(t, h, "/api/tournaments/"+uuid.NewString()+"/pairings", nil); code != http.StatusNotFound {
		t.Errorf("pairings of an unknown tournament = %d, want 404", code)
	}
}

// The sandbox's case: nobody confirms, so every result counts at once, and the
// final appears as soon as the last group match is reported.
func TestUnderImmediateSettlementATournamentIsPlayedToItsFinalThroughTheAPI(t *testing.T) {
	h, store := immediateScoreboardHandler(t)
	players, operator := fourPlayers(t, store)
	tour := apiTournamentOf(t, store, "Montag", true, time.Now(), players...)
	// Whoever stands earlier in the field wins, so the table has a clear
	// top two and the final is Ada against Bert.
	rank := map[uuid.UUID]int{}
	for i, p := range players {
		rank[p.ID] = i
	}

	for range 6 {
		places := openPlaces(t, h, tour)
		if len(places) == 0 || places[0].Final {
			t.Fatalf("the group is not played out, got %+v", places)
		}
		body := placeResult(tour, places[0], operator)
		if rank[places[0].AwayID] < rank[places[0].HomeID] {
			body["sets"] = [][2]int{{9, 11}, {7, 11}}
		}
		if rec := apiPost(t, h, testScoreboardToken, body); rec.Code != http.StatusCreated {
			t.Fatalf("POST = %d, want 201: %s", rec.Code, rec.Body.String())
		}
	}

	places := openPlaces(t, h, tour)
	if len(places) != 1 || !places[0].Final || places[0].Round != 4 ||
		places[0].HomeID != players[0].ID || places[0].AwayID != players[1].ID {
		t.Fatalf("after the group: got %+v, want only Ada against Bert in round 4", places)
	}
	if rec := apiPost(t, h, testScoreboardToken, placeResult(tour, places[0], operator)); rec.Code != http.StatusCreated {
		t.Fatalf("POST the final = %d, want 201: %s", rec.Code, rec.Body.String())
	}
	if places := openPlaces(t, h, tour); len(places) != 0 {
		t.Fatalf("after the final: got %+v, want nothing open", places)
	}

	var listed []struct {
		Played int `json:"played"`
	}
	apiGet(t, h, "/api/tournaments", &listed)
	if len(listed) != 1 || listed[0].Played != 7 {
		t.Errorf("the list reads %+v, want 7 played", listed)
	}
}

func TestAnUnratedTournamentMovesNoRatingEvenWhenItCountsAtOnce(t *testing.T) {
	h, store := immediateScoreboardHandler(t)
	players, operator := fourPlayers(t, store)
	tour, err := store.tournaments.Create(t.Context(), domain.Tournament{
		Name: "Freitag", Format: domain.TournamentRoundRobin, CreatedBy: players[0].ID,
		CreatedAt: time.Now(), BestOf: 3, PointsToWin: 11, Rated: false,
		Players: []uuid.UUID{players[0].ID, players[1].ID, players[2].ID, players[3].ID},
	})
	if err != nil {
		t.Fatalf("creating the tournament: %v", err)
	}
	place := openPlaces(t, h, tour)[0]

	rec := apiPost(t, h, testScoreboardToken, placeResult(tour, place, operator))
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST = %d, want 201: %s", rec.Code, rec.Body.String())
	}
	settledAnswer(t, rec.Body.Bytes())
	for _, id := range []uuid.UUID{place.HomeID, place.AwayID} {
		p, err := store.Players().ByID(t.Context(), id)
		if err != nil {
			t.Fatalf("reading a player: %v", err)
		}
		if p.TTR != domain.DefaultTTR {
			t.Errorf("%s moved to %d in an unrated tournament", p.DisplayName, p.TTR)
		}
	}
}

// Two check-ins prove a tournament result as well as an operator does: a
// tournament evening at the table is what check-in is for.
func TestACheckedInResultFillsAPlaceToo(t *testing.T) {
	h, store := scoreboardHandler(t)
	players, operator := fourPlayers(t, store)
	tour := apiTournamentOf(t, store, "Montag", false, time.Now(), players...)
	place := openPlaces(t, h, tour)[0]

	home := checkedIn(t, store, place.HomeID, domain.HalfLeft)
	away := checkedIn(t, store, place.AwayID, domain.HalfRight)
	body := placeResult(tour, place, operator)
	delete(body, "operator_id")
	body["home_checkin_id"], body["away_checkin_id"] = home.ID, away.ID

	rec := apiPost(t, h, testScoreboardToken, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST = %d, want 201: %s", rec.Code, rec.Body.String())
	}
	stored, err := store.Matches().ByID(t.Context(), createdMatch(t, rec.Body.Bytes()))
	if err != nil {
		t.Fatalf("reading the match: %v", err)
	}
	if stored.TournamentRound == nil || *stored.TournamentRound != place.Round || stored.EnteredVia != domain.EnteredViaCheckin {
		t.Errorf("stored round %v via %s, want round %d via checkin", stored.TournamentRound, stored.EnteredVia, place.Round)
	}

	again := checkedInResult(players[0], players[1], uuid.Nil, uuid.Nil)
	h2 := checkedIn(t, store, place.HomeID, domain.HalfLeft)
	a2 := checkedIn(t, store, place.AwayID, domain.HalfRight)
	again["home_id"], again["away_id"] = place.HomeID, place.AwayID
	again["home_checkin_id"], again["away_checkin_id"] = h2.ID, a2.ID
	again["tournament_id"], again["round"] = tour.ID, place.Round
	if rec := apiPost(t, h, testScoreboardToken, again); rec.Code != http.StatusConflict {
		t.Fatalf("the same place with fresh check-ins = %d, want 409: %s", rec.Code, rec.Body.String())
	}
}
