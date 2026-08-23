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

const testKioskToken = "tuesday"

// kioskHandler wires a server with the kiosk unlocked by testKioskToken.
func kioskHandler(t *testing.T) (http.Handler, *memStore) {
	t.Helper()

	store := newMemStore()
	cfg := testConfig()
	cfg.KioskToken = testKioskToken
	cfg.SessionKey = testSessionKey
	h := newHandlerConfig(cfg, store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))
	return h, store
}

// unlock walks the token exchange and returns the kiosk cookie.
func unlock(t *testing.T, h http.Handler) *http.Cookie {
	t.Helper()

	rec := get(t, h, "/kiosk?token="+testKioskToken)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("unlocking = %d, want 303: %s", rec.Code, rec.Body.String())
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == "schmetterpause_kiosk" {
			return c
		}
	}
	t.Fatal("unlocking set no kiosk cookie")
	return nil
}

func kioskPost(t *testing.T, h http.Handler, path string, cookie *http.Cookie, form url.Values) *httptest.ResponseRecorder {
	t.Helper()

	r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if cookie != nil {
		r.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

// TestTheKioskDoesNotExistWithoutAToken: an unlocked kiosk on a laptop in an
// office is worse than none, so an unset token removes the routes rather than
// leaving them open.
func TestTheKioskDoesNotExistWithoutAToken(t *testing.T) {
	h := newHandler(newMemStore()) // testConfig leaves KioskToken empty

	for _, path := range []string{"/kiosk", "/kiosk?token=tuesday"} {
		if rec := get(t, h, path); rec.Code != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", path, rec.Code)
		}
	}
	if rec := kioskPost(t, h, "/kiosk/matches", nil, url.Values{}); rec.Code != http.StatusNotFound {
		t.Errorf("POST /kiosk/matches = %d, want 404", rec.Code)
	}
}

func TestTheKioskWantsTheToken(t *testing.T) {
	h, _ := kioskHandler(t)

	if rec := get(t, h, "/kiosk"); rec.Code != http.StatusForbidden {
		t.Errorf("without a cookie = %d, want 403", rec.Code)
	}
	if rec := get(t, h, "/kiosk?token=wednesday"); rec.Code != http.StatusForbidden {
		t.Errorf("with the wrong token = %d, want 403", rec.Code)
	}
	if rec := kioskPost(t, h, "/kiosk/players", nil, url.Values{"display_name": {"Anna"}}); rec.Code != http.StatusForbidden {
		t.Errorf("posting without a cookie = %d, want 403", rec.Code)
	}

	kiosk := unlock(t, h)

	r := httptest.NewRequest(http.MethodGet, "/kiosk", nil)
	r.AddCookie(kiosk)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	if rec.Code != http.StatusOK {
		t.Fatalf("with the cookie = %d, want 200", rec.Code)
	}
	// The token is swapped for a cookie and does not come back in the page.
	if strings.Contains(rec.Body.String(), testKioskToken) {
		t.Error("the page carries the token where the next borrower can read it")
	}
}

// TestTheKioskLeavesItsOwnSessionAlone is the reason a kiosk needs its own
// route at all: joining normally sets the cookie, so eight players entered
// from one laptop would leave the laptop signed in as the eighth.
func TestTheKioskLeavesItsOwnSessionAlone(t *testing.T) {
	h, store := kioskHandler(t)
	kiosk := unlock(t, h)

	for _, name := range []string{"Anna", "Bodo", "Cara"} {
		rec := kioskPost(t, h, "/kiosk/players", kiosk, url.Values{"display_name": {name}})
		if rec.Code != http.StatusOK {
			t.Fatalf("adding %s = %d: %s", name, rec.Code, rec.Body.String())
		}
		for _, c := range rec.Result().Cookies() {
			if c.Name == auth.SessionCookieName {
				t.Fatalf("adding %s signed the kiosk in as them", name)
			}
		}
	}

	n, err := store.Players().Count(t.Context())
	if err != nil {
		t.Fatalf("Count(): %v", err)
	}
	if n != 3 {
		t.Errorf("%d players exist, want 3", n)
	}
}

func TestTheKioskRefusesATakenName(t *testing.T) {
	h, _ := kioskHandler(t)
	kiosk := unlock(t, h)

	kioskPost(t, h, "/kiosk/players", kiosk, url.Values{"display_name": {"Anna"}})
	rec := kioskPost(t, h, "/kiosk/players", kiosk, url.Values{"display_name": {"anna"}})

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "gibt es schon") {
		t.Errorf("the refusal does not say why: %s", rec.Body.String())
	}
}

// kioskPlayers returns the ids of the seeded players, by name.
func kioskPlayers(t *testing.T, store *memStore, names ...string) []string {
	t.Helper()

	players, err := store.Players().List(t.Context())
	if err != nil {
		t.Fatalf("List(): %v", err)
	}

	ids := make([]string, 0, len(names))
	for _, want := range names {
		found := ""
		for _, p := range players {
			if p.DisplayName == want {
				found = p.ID.String()
			}
		}
		if found == "" {
			t.Fatalf("no player named %q", want)
		}
		ids = append(ids, found)
	}
	return ids
}

func kioskResult(home, away string, bestOf, points int, sets ...string) url.Values {
	form := url.Values{
		"home_id":       {home},
		"away_id":       {away},
		"best_of":       {strconv.Itoa(bestOf)},
		"points_to_win": {strconv.Itoa(points)},
	}
	for i, s := range sets {
		h, a, _ := strings.Cut(s, ":")
		form.Set("set_home_"+strconv.Itoa(i+1), h)
		form.Set("set_away_"+strconv.Itoa(i+1), a)
	}
	return form
}

// TestTheKioskSettlesAtOnce is the Definition of Done: somebody watched the
// match and wrote it down, so there is nobody left to ask and the rating
// moves immediately.
func TestTheKioskSettlesAtOnce(t *testing.T) {
	h, store := kioskHandler(t)
	kiosk := unlock(t, h)

	kioskPost(t, h, "/kiosk/players", kiosk, url.Values{"display_name": {"Anna"}})
	kioskPost(t, h, "/kiosk/players", kiosk, url.Values{"display_name": {"Bodo"}})
	ids := kioskPlayers(t, store, "Anna", "Bodo")

	rec := kioskPost(t, h, "/kiosk/matches", kiosk, kioskResult(ids[0], ids[1], 3, 11, "11:9", "12:10"))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	// Told the way somebody at the table would say it.
	if !strings.Contains(rec.Body.String(), "Anna schlägt Bodo 2:0") {
		t.Errorf("the page does not say what happened: %s", rec.Body.String())
	}

	stored := store.matches.all()
	if len(stored) != 1 {
		t.Fatalf("%d matches stored, want 1", len(stored))
	}
	if stored[0].Status != domain.MatchConfirmed {
		t.Errorf("status = %q, want confirmed — the kiosk asks nobody", stored[0].Status)
	}
	if got := ttrOfPlayer(t, store, "Anna"); got != 1008 {
		t.Errorf("Anna is on %d, want 1008 — the result did not count", got)
	}
	if got := ttrOfPlayer(t, store, "Bodo"); got != 992 {
		t.Errorf("Bodo is on %d, want 992", got)
	}
}

func TestTheKioskRefusesImpossibleInput(t *testing.T) {
	h, store := kioskHandler(t)
	kiosk := unlock(t, h)

	kioskPost(t, h, "/kiosk/players", kiosk, url.Values{"display_name": {"Anna"}})
	kioskPost(t, h, "/kiosk/players", kiosk, url.Values{"display_name": {"Bodo"}})
	ids := kioskPlayers(t, store, "Anna", "Bodo")

	tests := []struct {
		name string
		form url.Values
		says string
	}{
		{"the same player twice", kioskResult(ids[0], ids[0], 3, 11, "11:9", "11:7"), "verschiedene Spieler"},
		{"nobody chosen", kioskResult("", "", 3, 11, "11:9", "11:7"), "beide Spieler"},
		{"one clear point", kioskResult(ids[0], ids[1], 3, 11, "11:10", "11:9"), "zwei Punkte Vorsprung"},
		{"undecided", kioskResult(ids[0], ids[1], 5, 11, "11:9"), "noch nicht entschieden"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := kioskPost(t, h, "/kiosk/matches", kiosk, tc.form)

			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want 422", rec.Code)
			}
			if !strings.Contains(rec.Body.String(), tc.says) {
				t.Errorf("the refusal does not say why: %s", rec.Body.String())
			}
			if n := len(store.matches.all()); n != 0 {
				t.Errorf("%d matches were stored anyway", n)
			}
			if got := ttrOfPlayer(t, store, "Anna"); got != domain.DefaultTTR {
				t.Errorf("Anna moved to %d on a refused entry", got)
			}
		})
	}
}

func TestTheKioskLeadsWithTheThingThatHappensAllEvening(t *testing.T) {
	// Adding a player happens once per person, entering a result happens
	// after every match. The order on the page follows the frequency.
	h, _ := kioskHandler(t)
	cookie := unlock(t, h)

	body := fragment(t, h, "/kiosk", cookie).Body.String()

	enter := strings.Index(body, "Ergebnis eintragen")
	create := strings.Index(body, "Spieler anlegen")

	if enter < 0 || create < 0 {
		t.Fatalf("the kiosk is missing one of its two forms: %s", body)
	}
	if enter > create {
		t.Error("adding a player comes before entering a result")
	}
}

func TestTheKioskCannotPitchAPlayerAgainstThemselves(t *testing.T) {
	// The server refuses it, but only after the whole match has been typed
	// in. The name comes out of the other list instead, so the mistake is
	// unavailable rather than punished.
	h, store := kioskHandler(t)
	cookie := unlock(t, h)

	for _, name := range []string{"Anna", "Bodo"} {
		if _, err := store.Players().Create(t.Context(), name, domain.DefaultTTR); err != nil {
			t.Fatalf("seeding %s: %v", name, err)
		}
	}
	anna := opponentID(t, store, "Anna")

	r := httptest.NewRequest(http.MethodGet,
		"/fragments/sets?sets_prefix=kiosk&home_id="+anna+"&best_of=3", nil)
	r.Header.Set("HX-Trigger", "kiosk-home")
	r.AddCookie(cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)

	body := rec.Body.String()

	if !strings.Contains(body, `id="kiosk-away"`) {
		t.Fatalf("the opposite picker did not come back: %s", body)
	}
	if !strings.Contains(body, `hx-swap-oob="true"`) {
		t.Errorf("the picker is not marked for an out-of-band swap: %s", body)
	}
	if strings.Contains(body, `value="`+anna+`"`) {
		t.Errorf("the opponent list still offers the player who is already home: %s", body)
	}
	if !strings.Contains(body, "Bodo") {
		t.Errorf("the opponent list lost everybody else too: %s", body)
	}
}

func TestAModeChangeLeavesThePickersAlone(t *testing.T) {
	// The two selects already agree with each other then, and replacing a
	// select somebody is not touching is a change they did not ask for.
	h, store := kioskHandler(t)
	cookie := unlock(t, h)
	if _, err := store.Players().Create(t.Context(), "Anna", domain.DefaultTTR); err != nil {
		t.Fatalf("seeding: %v", err)
	}

	r := httptest.NewRequest(http.MethodGet, "/fragments/sets?sets_prefix=kiosk&best_of=3", nil)
	r.Header.Set("HX-Trigger", "kiosk-best-of")
	r.AddCookie(cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)

	if strings.Contains(rec.Body.String(), "hx-swap-oob") {
		t.Errorf("a mode change replaced a picker: %s", rec.Body.String())
	}
}

func TestBothPagesGreetWithTheMascot(t *testing.T) {
	// One drawing, at the top of both pages, and hidden in print.
	h, store := kioskHandler(t)
	cookie := unlock(t, h)

	kiosk := fragment(t, h, "/kiosk", cookie).Body.String()
	if !strings.Contains(kiosk, `class="page-mascot"`) {
		t.Errorf("the kiosk has no mascot: %s", kiosk)
	}
	// Above the heading, not somewhere below the forms.
	if strings.Index(kiosk, "page-mascot") > strings.Index(kiosk, "<h1>Kiosk</h1>") {
		t.Error("the kiosk mascot sits below the heading")
	}

	if _, err := store.Players().Create(t.Context(), "Anna", domain.DefaultTTR); err != nil {
		t.Fatalf("seeding: %v", err)
	}
	start := get(t, h, "/").Body.String()
	if !strings.Contains(start, `class="page-mascot"`) {
		t.Errorf("the start page has no mascot: %s", start)
	}
	if strings.Index(start, "page-mascot") > strings.Index(start, `id="standings"`) {
		t.Error("the start page mascot sits below the ranking")
	}
}
