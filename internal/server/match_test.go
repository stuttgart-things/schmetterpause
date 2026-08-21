package server_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/stuttgart-things/schmetterpause/internal/auth"
	"github.com/stuttgart-things/schmetterpause/internal/domain"
)

// twoPlayers signs Anna in and adds Bodo as an opponent, returning Anna's
// cookie and the handler.
func twoPlayers(t *testing.T) (http.Handler, *memStore, *http.Cookie) {
	t.Helper()

	store := newMemStore()
	h := newHandlerWith(store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))

	cookie := sessionCookie(t, join(t, h, "Anna"))
	if _, err := store.Players().Create(t.Context(), "Bodo", domain.DefaultTTR); err != nil {
		t.Fatalf("creating the opponent: %v", err)
	}
	return h, store, cookie
}

func opponentID(t *testing.T, store *memStore, name string) string {
	t.Helper()

	players, err := store.Players().List(t.Context())
	if err != nil {
		t.Fatalf("List(): %v", err)
	}
	for _, p := range players {
		if p.DisplayName == name {
			return p.ID.String()
		}
	}
	t.Fatalf("no player named %q", name)
	return ""
}

// recordMatch posts a result. sets are given as "home:away" strings in order.
func recordMatch(t *testing.T, h http.Handler, cookie *http.Cookie, opponent string, bestOf, points int, sets ...string) *httptest.ResponseRecorder {
	t.Helper()

	form := url.Values{
		"opponent_id":   {opponent},
		"best_of":       {strconv.Itoa(bestOf)},
		"points_to_win": {strconv.Itoa(points)},
	}
	for i, s := range sets {
		home, away, _ := strings.Cut(s, ":")
		form.Set("set_home_"+strconv.Itoa(i+1), home)
		form.Set("set_away_"+strconv.Itoa(i+1), away)
	}

	r := httptest.NewRequest(http.MethodPost, "/matches", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if cookie != nil {
		r.AddCookie(cookie)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

func TestTheFormIsHiddenFromStrangers(t *testing.T) {
	rec := get(t, newHandler(newMemStore()), "/")

	if strings.Contains(rec.Body.String(), `hx-post="/matches"`) {
		t.Error("result entry is offered to somebody who is not recognised")
	}
}

func TestTheFormAppearsOnceRecognised(t *testing.T) {
	h, _, cookie := twoPlayers(t)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)

	body := rec.Body.String()
	if !strings.Contains(body, `hx-post="/matches"`) {
		t.Fatalf("the start page offers no result entry: %s", body)
	}
	if !strings.Contains(body, "Bodo") {
		t.Error("the opponent picker does not list Bodo")
	}
}

// TestThePickerLeavesYouOut covers matches_players_differ from the friendly
// side: the schema refuses a match against yourself, so the form never offers
// it rather than explaining it after the fact.
func TestThePickerLeavesYouOut(t *testing.T) {
	h, store, cookie := twoPlayers(t)
	anna := opponentID(t, store, "Anna")

	r := httptest.NewRequest(http.MethodGet, "/fragments/match", nil)
	r.AddCookie(cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)

	if strings.Contains(rec.Body.String(), `<option value="`+anna+`"`) {
		t.Errorf("the picker offers the player themselves: %s", rec.Body.String())
	}
}

func TestRecordingAMatchStoresItAsPending(t *testing.T) {
	h, store, cookie := twoPlayers(t)

	rec := recordMatch(t, h, cookie, opponentID(t, store, "Bodo"), 5, 11, "11:9", "9:11", "11:7", "12:10")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Gewonnen") || !strings.Contains(body, "3:1") {
		t.Errorf("the confirmation does not state the outcome: %s", body)
	}
	// An unconfirmed match changes no rating, and saying so beats letting
	// the ranking look wrong later.
	if !strings.Contains(body, "bestätigen") {
		t.Errorf("the confirmation does not mention that Bodo must confirm: %s", body)
	}

	stored := store.matches.all()
	if len(stored) != 1 {
		t.Fatalf("%d matches were stored, want 1", len(stored))
	}
	m := stored[0]
	if m.Status != domain.MatchPending {
		t.Errorf("status = %q, want pending", m.Status)
	}
	if m.ReportedBy != m.HomeID {
		t.Errorf("ReportedBy = %s, want the reporting player %s", m.ReportedBy, m.HomeID)
	}
	if len(m.Sets) != 4 {
		t.Fatalf("%d sets were stored, want 4", len(m.Sets))
	}
	for i, s := range m.Sets {
		if s.SetNo != i+1 {
			t.Errorf("set %d is numbered %d", i+1, s.SetNo)
		}
	}
	if m.BestOf != 5 || m.PointsToWin != 11 {
		t.Errorf("mode = best of %d to %d, want 5 to 11", m.BestOf, m.PointsToWin)
	}
}

func TestRecordingAMatchToTwentyOne(t *testing.T) {
	h, store, cookie := twoPlayers(t)

	rec := recordMatch(t, h, cookie, opponentID(t, store, "Bodo"), 3, 21, "21:19", "23:21")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if got := store.matches.all()[0].PointsToWin; got != 21 {
		t.Errorf("PointsToWin = %d, want 21", got)
	}
}

// TestARejectedResultSaysWhy is the Definition of Done of AP4. Each case
// checks for wording specific to the rule that was broken, so a generic
// "ungültige Eingabe" would fail every one of them.
func TestARejectedResultSaysWhy(t *testing.T) {
	tests := []struct {
		name     string
		sets     []string
		wantText string
	}{
		{"won by a single point", []string{"11:10", "11:0"}, "zwei Punkte Vorsprung"},
		{"set not finished", []string{"10:8", "11:0"}, "noch nicht zu Ende"},
		{"ran past the decision", []string{"13:10", "11:0"}, "sobald jemand zwei Punkte vorn"},
		{"level", []string{"11:11", "11:0"}, "gewinnt immer jemand"},
		{"unfinished match", []string{"11:0", "0:11"}, "noch nicht entschieden"},
		{"a set too many", []string{"11:0", "11:0", "11:0"}, "schon entschieden"},
		{"nothing entered", nil, "mindestens einen Satz"},
		{"half a set", []string{"11:"}, "halb ausgefüllt"},
		{"a gap", []string{"11:0", ":", "11:0"}, "hinter einem leeren Satz"},
		{"not a number", []string{"elf:9"}, "keine Zahl"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h, store, cookie := twoPlayers(t)

			rec := recordMatch(t, h, cookie, opponentID(t, store, "Bodo"), 3, 11, tc.sets...)

			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want 422", rec.Code)
			}
			if !strings.Contains(rec.Body.String(), tc.wantText) {
				t.Errorf("the message does not contain %q: %s", tc.wantText, rec.Body.String())
			}
			if n := len(store.matches.all()); n != 0 {
				t.Errorf("%d matches were stored despite the rejection, want 0", n)
			}
		})
	}
}

// TestARejectionHandsTheInputBack matters more than it looks: retyping a
// four-set result because one box was wrong is exactly the friction the
// Definition of Done of the MVP is measuring.
func TestARejectionHandsTheInputBack(t *testing.T) {
	h, store, cookie := twoPlayers(t)
	bodo := opponentID(t, store, "Bodo")

	rec := recordMatch(t, h, cookie, bodo, 7, 21, "21:19", "11:10")

	body := rec.Body.String()
	for _, want := range []string{`value="21"`, `value="19"`, `value="11"`, `value="10"`} {
		if !strings.Contains(body, want) {
			t.Errorf("the form does not hand back %s: %s", want, body)
		}
	}
	if !strings.Contains(body, `<option value="`+bodo+`" selected>`) {
		t.Errorf("the chosen opponent was not kept: %s", body)
	}
	if !strings.Contains(body, `<option value="7" selected>`) {
		t.Errorf("the chosen mode was not kept: %s", body)
	}
}

func TestRecordingWithoutAnOpponent(t *testing.T) {
	h, _, cookie := twoPlayers(t)

	rec := recordMatch(t, h, cookie, "", 3, 11, "11:0", "11:0")

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Wähle einen Gegner") {
		t.Errorf("the message does not ask for an opponent: %s", rec.Body.String())
	}
}

func TestRecordingAgainstSomebodyWhoDoesNotExist(t *testing.T) {
	h, _, cookie := twoPlayers(t)

	rec := recordMatch(t, h, cookie, "11111111-2222-3333-4444-555555555555", 3, 11, "11:0", "11:0")

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Diesen Gegner gibt es nicht") {
		t.Errorf("the message does not say the opponent is unknown: %s", rec.Body.String())
	}
}

func TestStrangersCannotRecordMatches(t *testing.T) {
	h, store, _ := twoPlayers(t)

	rec := recordMatch(t, h, nil, opponentID(t, store, "Bodo"), 3, 11, "11:0", "11:0")

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
	if n := len(store.matches.all()); n != 0 {
		t.Errorf("%d matches were stored by an unrecognised browser, want 0", n)
	}
}

func TestTheMatchFormFragment(t *testing.T) {
	h, _, cookie := twoPlayers(t)

	r := httptest.NewRequest(http.MethodGet, "/fragments/match", nil)
	r.AddCookie(cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `hx-post="/matches"`) {
		t.Errorf("the fragment is not the entry form: %s", rec.Body.String())
	}
}
