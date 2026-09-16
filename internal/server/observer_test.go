package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// observerSetup is the office the flag is for (docs/adr/0022): Timo holds the
// admin flag from the bootstrap and flags himself, Anna and Bodo play.
func observerSetup(t *testing.T) (http.Handler, *memStore, *http.Cookie, *http.Cookie, string) {
	t.Helper()

	srv, store := adminHandler(t, "Timo")
	h := srv.Handler()

	timo := sessionCookie(t, join(t, h, "Timo"))
	anna := sessionCookie(t, join(t, h, "Anna"))
	join(t, h, "Bodo")
	srv.GrantBootstrapAdmin(t.Context())

	timoID := opponentID(t, store, "Timo")
	rec := postForm(t, h, "/admin/players/"+timoID+"/observer", url.Values{"observer": {"on"}}, timo)
	if rec.Code != http.StatusOK {
		t.Fatalf("flagging himself: status %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Timo ist jetzt Beobachter") {
		t.Errorf("the page does not say what happened: %s", rec.Body.String())
	}
	return h, store, timo, anna, timoID
}

// An admin who never plays flags their own account, which is the one thing
// the removal next to it refuses on your own row.
func TestAnAdminFlagsThemselvesAsObserver(t *testing.T) {
	h, store, timo, _, timoID := observerSetup(t)

	records, err := store.Players().Records(t.Context())
	if err != nil {
		t.Fatalf("Records(): %v", err)
	}
	for _, r := range records {
		if r.Player.DisplayName == "Timo" {
			t.Error("the observer is still in the ranking")
		}
	}
	if body := getWith(t, h, "/standings", nil).Body.String(); strings.Contains(body, "Timo") {
		t.Error("the ranking page still names the observer")
	}

	page := getWith(t, h, "/admin", timo).Body.String()
	for _, want := range []string{"Beobachter", "Spielt wieder mit"} {
		if !strings.Contains(page, want) {
			t.Errorf("the admin page does not show %q", want)
		}
	}

	// Still signs in and keeps a page of their own, since that is where the
	// PIN lives — but nothing on it about playing.
	profile := getWith(t, h, "/players/"+timoID, timo)
	if profile.Code != http.StatusOK {
		t.Fatalf("the observer's own page: status %d", profile.Code)
	}
	body := profile.Body.String()
	if !strings.Contains(body, "pin-card") {
		t.Error("the observer's own page has no PIN section")
	}
	if strings.Contains(body, "stat-value") || strings.Contains(body, "Letzte Matches") {
		t.Error("the observer's page still shows a rating or a match list")
	}

	if home := getWith(t, h, "/", timo).Body.String(); strings.Contains(home, "Ergebnis eintragen") {
		t.Error("the start page offers an observer result entry")
	}

	// And back: they play again at the rating they never moved.
	rec := postForm(t, h, "/admin/players/"+timoID+"/observer", url.Values{"observer": {"off"}}, timo)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Timo spielt wieder mit") {
		t.Fatalf("unflagging: status %d: %s", rec.Code, rec.Body.String())
	}
	playing, _ := store.Players().Playing(t.Context())
	found := false
	for _, p := range playing {
		found = found || p.DisplayName == "Timo"
	}
	if !found {
		t.Error("Timo does not play again after unflagging")
	}
}

// Nobody picks an observer as an opponent, and a form that names one anyway
// is refused in words rather than stored.
func TestAnObserverCannotBeAnOpponent(t *testing.T) {
	h, store, _, anna, timoID := observerSetup(t)

	if home := getWith(t, h, "/", anna).Body.String(); strings.Contains(home, timoID) {
		t.Error("the opponent picker offers the observer")
	}

	rec := recordMatch(t, h, anna, timoID, 3, 11, "11:9", "11:7")
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("a match against the observer: status %d, want 422", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Ein Beobachter spielt nicht mit.") {
		t.Errorf("the refusal does not say why: %s", rec.Body.String())
	}
	if n := len(store.matches.all()); n != 0 {
		t.Errorf("%d matches stored, want none", n)
	}
}

// The flag only goes on somebody with no history, and only an admin sets it.
func TestTheObserverFlagIsRefusedForAPlayerAndForANonAdmin(t *testing.T) {
	h, store, anna, bodo := twoPlayersAndAnAdmin(t)
	countedMatch(t, h, store, anna, bodo)
	bodoID := opponentID(t, store, "Bodo")

	rec := postForm(t, h, "/admin/players/"+bodoID+"/observer", url.Values{"observer": {"on"}}, anna)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("flagging somebody who played: status %d, want 422", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "hat schon gespielt") {
		t.Errorf("the refusal does not say why: %s", rec.Body.String())
	}

	annaID := opponentID(t, store, "Anna")
	if got := postForm(t, h, "/admin/players/"+annaID+"/observer",
		url.Values{"observer": {"on"}}, bodo).Code; got != http.StatusForbidden {
		t.Errorf("a plain player gets %d, want 403", got)
	}
	if p, _ := store.Players().ByDisplayName(t.Context(), "Anna"); p.IsObserver {
		t.Error("a refused caller set the flag anyway")
	}
}

// The Zählwerk is not offered an observer and cannot name one as a side, but
// an observer holding the pen is exactly an operator (docs/adr/0014).
func TestTheScoreboardAndAnObserver(t *testing.T) {
	h, store := scoreboardHandler(t)
	home, away, operator := threePlayers(t, store)
	if err := store.Players().SetObserver(t.Context(), operator.ID, true); err != nil {
		t.Fatalf("SetObserver(): %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/players", nil)
	req.Header.Set("Authorization", "Bearer "+testScoreboardToken)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var players []struct {
		ID uuid.UUID `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &players); err != nil {
		t.Fatalf("decoding the list: %v", err)
	}
	if len(players) != 2 {
		t.Errorf("GET /api/players lists %d, want the 2 who play", len(players))
	}
	for _, p := range players {
		if p.ID == operator.ID {
			t.Error("GET /api/players lists the observer")
		}
	}

	if rec := apiPost(t, h, testScoreboardToken, goodResult(operator, away, home)); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("the observer as a side: status %d, want 422 (%s)", rec.Code, rec.Body)
	}
	if rec := apiPost(t, h, testScoreboardToken, goodResult(home, away, operator)); rec.Code != http.StatusCreated {
		t.Errorf("the observer as operator: status %d, want 201 (%s)", rec.Code, rec.Body)
	}
}
