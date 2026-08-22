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
