package server_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/auth"
	"github.com/stuttgart-things/schmetterpause/internal/domain"
	"github.com/stuttgart-things/schmetterpause/internal/scoring"
)

const testScoreboardToken = "zaehlwerk"

// scoreboardHandler wires a server with the Zählwerk's surface unlocked.
func scoreboardHandler(t *testing.T) (http.Handler, *memStore) {
	t.Helper()

	store := newMemStore()
	cfg := testConfig()
	cfg.ScoreboardToken = testScoreboardToken
	cfg.SessionKey = testSessionKey
	h := newHandlerConfig(cfg, store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))
	return h, store
}

func apiPost(t *testing.T, h http.Handler, token string, body any) *httptest.ResponseRecorder {
	t.Helper()

	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshalling the body: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/results", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// threePlayers returns home, away and an operator who is not playing.
func threePlayers(t *testing.T, store *memStore) (domain.Player, domain.Player, domain.Player) {
	t.Helper()

	var out []domain.Player
	for _, name := range []string{"Anna", "Bea", "Cem"} {
		p, err := store.Players().Create(t.Context(), name, domain.DefaultTTR)
		if err != nil {
			t.Fatalf("creating %s: %v", name, err)
		}
		out = append(out, p)
	}
	return out[0], out[1], out[2]
}

func goodResult(home, away, operator domain.Player) map[string]any {
	return map[string]any{
		"home_id":       home.ID,
		"away_id":       away.ID,
		"operator_id":   operator.ID,
		"sets":          [][2]int{{11, 9}, {11, 7}},
		"best_of":       3,
		"points_to_win": 11,
	}
}

// TestScoreboardRoutesDoNotExistWithoutTheToken is the property the whole
// surface rests on: unset means absent, not unlocked. Same posture as /kiosk
// without SP_KIOSK_TOKEN, and docs/adr/0015 says so in as many words.
func TestScoreboardRoutesDoNotExistWithoutTheToken(t *testing.T) {
	store := newMemStore()
	cfg := testConfig()
	cfg.SessionKey = testSessionKey
	h := newHandlerConfig(cfg, store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))

	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/api/players"},
		{http.MethodPost, "/api/results"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		req.Header.Set("Authorization", "Bearer anything")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("%s %s without a token: got %d, want 404 — an unset token must mean the route does not exist",
				tc.method, tc.path, rec.Code)
		}
	}
}

func TestScoreboardRefusesAWrongOrMissingToken(t *testing.T) {
	h, store := scoreboardHandler(t)
	home, away, operator := threePlayers(t, store)

	for _, token := range []string{"", "wrong"} {
		req := httptest.NewRequest(http.MethodGet, "/api/players", nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("GET /api/players with token %q: got %d, want 401", token, rec.Code)
		}

		if rec := apiPost(t, h, token, goodResult(home, away, operator)); rec.Code != http.StatusUnauthorized {
			t.Errorf("POST /api/results with token %q: got %d, want 401", token, rec.Code)
		}
	}
}

func TestScoreboardListsPlayers(t *testing.T) {
	h, store := scoreboardHandler(t)
	home, _, _ := threePlayers(t, store)

	req := httptest.NewRequest(http.MethodGet, "/api/players", nil)
	req.Header.Set("Authorization", "Bearer "+testScoreboardToken)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/players: got %d, want 200 (%s)", rec.Code, rec.Body)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Errorf("Content-Type: got %q", got)
	}

	var players []struct {
		ID          uuid.UUID `json:"id"`
		DisplayName string    `json:"display_name"`
		TTR         int       `json:"ttr"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &players); err != nil {
		t.Fatalf("decoding the list: %v", err)
	}
	if len(players) != 3 {
		t.Fatalf("got %d players, want 3", len(players))
	}
	for _, p := range players {
		if p.ID == uuid.Nil || p.DisplayName == "" || p.TTR == 0 {
			t.Errorf("incomplete player in the list: %+v", p)
		}
	}
	// The id the other side sends back has to be one it was given here.
	var found bool
	for _, p := range players {
		if p.ID == home.ID {
			found = true
		}
	}
	if !found {
		t.Error("the list does not contain a player the store holds")
	}
}

// TestScoreboardResultStaysPending is the decision docs/adr/0015 took against
// the kiosk's: a counted result waits for a human rather than entering the
// ranking on its own.
func TestScoreboardResultStaysPending(t *testing.T) {
	h, store := scoreboardHandler(t)
	home, away, operator := threePlayers(t, store)

	rec := apiPost(t, h, testScoreboardToken, goodResult(home, away, operator))
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /api/results: got %d, want 201 (%s)", rec.Code, rec.Body)
	}

	var created struct {
		MatchID uuid.UUID `json:"match_id"`
		Status  string    `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decoding the response: %v", err)
	}
	if created.Status != string(domain.MatchPending) {
		t.Errorf("status: got %q, want %q", created.Status, domain.MatchPending)
	}

	stored, err := store.Matches().ByID(t.Context(), created.MatchID)
	if err != nil {
		t.Fatalf("reading the stored match: %v", err)
	}
	if stored.Status != domain.MatchPending {
		t.Errorf("stored status: got %q, want pending", stored.Status)
	}
	if stored.ReportedBy != operator.ID {
		t.Errorf("reported_by: got %s, want the operator %s", stored.ReportedBy, operator.ID)
	}
	if stored.EnteredVia != domain.EnteredViaScoreboard {
		t.Errorf("entered_via: got %q, want %q", stored.EnteredVia, domain.EnteredViaScoreboard)
	}
	if stored.TournamentID != nil {
		t.Errorf("tournament_id: got %v, want nil — adr/0015 postpones tournaments", stored.TournamentID)
	}

	// The rating has not moved, which is the whole point of pending.
	after, err := store.Players().ByID(t.Context(), home.ID)
	if err != nil {
		t.Fatalf("reading the player: %v", err)
	}
	if after.TTR != home.TTR {
		t.Errorf("TTR moved before anybody confirmed: %d -> %d", home.TTR, after.TTR)
	}
}

// TestEitherPlayerCanConfirmAScoreboardResult is the consequence of the
// reporter being a third party, and the only path in this application where
// both participants may rule on a match. scoring.load asks for a participant
// who is not the reporter; with the operator reporting, that is two people
// rather than one. Nothing here changes scoring — this pins that it is true.
func TestEitherPlayerCanConfirmAScoreboardResult(t *testing.T) {
	for _, confirmer := range []string{"home", "away"} {
		t.Run(confirmer, func(t *testing.T) {
			h, store := scoreboardHandler(t)
			home, away, operator := threePlayers(t, store)

			rec := apiPost(t, h, testScoreboardToken, goodResult(home, away, operator))
			if rec.Code != http.StatusCreated {
				t.Fatalf("POST /api/results: got %d (%s)", rec.Code, rec.Body)
			}
			var created struct {
				MatchID uuid.UUID `json:"match_id"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
				t.Fatalf("decoding: %v", err)
			}

			by := home.ID
			if confirmer == "away" {
				by = away.ID
			}
			if _, err := scoring.Confirm(t.Context(), store, created.MatchID, by, time.Now()); err != nil {
				t.Fatalf("%s confirming a scoreboard result: %v", confirmer, err)
			}

			stored, err := store.Matches().ByID(t.Context(), created.MatchID)
			if err != nil {
				t.Fatalf("reading the match: %v", err)
			}
			if stored.Status != domain.MatchConfirmed {
				t.Errorf("after %s confirmed: status %q, want confirmed", confirmer, stored.Status)
			}
		})
	}
}

// TestScoreboardRequiresAnOperatorWhoIsNotPlaying carries docs/adr/0014's rule
// onto this path unchanged. A machine has no identity, so it names the person
// who watched — and that person may not be one of the two.
func TestScoreboardRequiresAnOperatorWhoIsNotPlaying(t *testing.T) {
	h, store := scoreboardHandler(t)
	home, away, operator := threePlayers(t, store)

	t.Run("missing", func(t *testing.T) {
		body := goodResult(home, away, operator)
		delete(body, "operator_id")
		if rec := apiPost(t, h, testScoreboardToken, body); rec.Code != http.StatusBadRequest {
			t.Errorf("without an operator: got %d, want 400 (%s)", rec.Code, rec.Body)
		}
	})

	for _, playing := range []struct {
		name string
		id   uuid.UUID
	}{{"home", home.ID}, {"away", away.ID}} {
		t.Run("operator is the "+playing.name+" player", func(t *testing.T) {
			body := goodResult(home, away, operator)
			body["operator_id"] = playing.id
			rec := apiPost(t, h, testScoreboardToken, body)
			if rec.Code != http.StatusUnprocessableEntity {
				t.Errorf("operator playing: got %d, want 422 (%s)", rec.Code, rec.Body)
			}
		})
	}
}

func TestScoreboardRefusesBadBodies(t *testing.T) {
	h, store := scoreboardHandler(t)
	home, away, operator := threePlayers(t, store)

	cases := []struct {
		name   string
		mutate func(map[string]any)
		want   int
	}{
		{"same player twice", func(b map[string]any) { b["away_id"] = home.ID }, http.StatusBadRequest},
		{"no sets", func(b map[string]any) { b["sets"] = [][2]int{} }, http.StatusBadRequest},
		{"no mode", func(b map[string]any) { delete(b, "best_of") }, http.StatusBadRequest},
		{"unknown player", func(b map[string]any) { b["away_id"] = uuid.New() }, http.StatusUnprocessableEntity},
		{"unknown operator", func(b map[string]any) { b["operator_id"] = uuid.New() }, http.StatusUnprocessableEntity},
		// A set nobody could have played. The arithmetic is internal/match's,
		// and this surface must not have a second opinion about it.
		{"impossible set", func(b map[string]any) { b["sets"] = [][2]int{{11, 9}, {4, 3}} }, http.StatusBadRequest},
		// A misspelt field would otherwise be dropped in silence, and the
		// result stored would not be the one that was played.
		{"unknown field", func(b map[string]any) { b["set"] = [][2]int{{11, 0}} }, http.StatusBadRequest},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := goodResult(home, away, operator)
			tc.mutate(body)
			rec := apiPost(t, h, testScoreboardToken, body)
			if rec.Code != tc.want {
				t.Errorf("got %d, want %d (%s)", rec.Code, tc.want, rec.Body)
			}
			var e struct {
				Error string `json:"error"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil || e.Error == "" {
				t.Errorf("a refusal must be JSON with a reason, got %q", rec.Body)
			}
		})
	}
}
