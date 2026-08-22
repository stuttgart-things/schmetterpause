package server_test

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stuttgart-things/schmetterpause/internal/auth"
	"github.com/stuttgart-things/schmetterpause/internal/config"
	"github.com/stuttgart-things/schmetterpause/internal/domain"
	"github.com/stuttgart-things/schmetterpause/internal/repository"
	"github.com/stuttgart-things/schmetterpause/internal/server"
)

// newHandler wires a server with no recognition at all.
// testSessionKey signs cookies in these tests; its content does not matter.
var testSessionKey = []byte("0123456789abcdef0123456789abcdef")

func newHandler(store repository.Store) http.Handler {
	return newHandlerWith(store, auth.Anonymous{})
}

// newHandlerWith wires a server with the given authenticator.
func newHandlerWith(store repository.Store, a auth.SessionAuthenticator) http.Handler {
	return newHandlerConfig(testConfig(), store, a)
}

// newHandlerConfig wires a server with a configuration of the caller's own.
func newHandlerConfig(cfg config.Config, store repository.Store, a auth.SessionAuthenticator) http.Handler {
	log := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	return server.New(cfg, store, log, a, "test").Handler()
}

// testConfig is the configuration the handler tests run against.
func testConfig() config.Config {
	return config.Config{
		HTTPAddr:         ":0",
		DatabaseURL:      "postgres://test",
		ShutdownTimeout:  time.Second,
		ReadinessTimeout: time.Second,
	}
}

func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestHealthzIgnoresDatabase(t *testing.T) {
	// Liveness must not depend on the database: an outage should not trigger
	// a container restart.
	h := newHandler(&memStore{players: &memPlayers{}, pingErr: errors.New("database gone")})

	rec := get(t, h, "/healthz")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != "ok" {
		t.Errorf("body = %q, want \"ok\"", got)
	}
}

func TestReadyz(t *testing.T) {
	tests := []struct {
		name    string
		pingErr error
		want    int
	}{
		{"database reachable", nil, http.StatusOK},
		{"database gone", errors.New("database gone"), http.StatusServiceUnavailable},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := get(t, newHandler(&memStore{players: &memPlayers{}, pingErr: tc.pingErr}), "/readyz")

			if rec.Code != tc.want {
				t.Errorf("status = %d, want %d", rec.Code, tc.want)
			}
		})
	}
}

func TestIndexRendersLayout(t *testing.T) {
	h := newHandler(newMemStore())

	rec := get(t, h, "/")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"Schmetterpause", "/static/js/htmx.min.js", `hx-get="/fragments/status"`} {
		if !strings.Contains(body, want) {
			t.Errorf("page does not contain %q", want)
		}
	}
}

func TestStatusFragmentShowsPlayerCount(t *testing.T) {
	h := newHandler(storeWithPlayers(t, 7))

	rec := get(t, h, "/fragments/status")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, ">7<") {
		t.Errorf("fragment does not state the player count: %s", body)
	}
	if !strings.Contains(body, "erreichbar") {
		t.Errorf("fragment does not state the database status: %s", body)
	}
}

func TestStatusFragmentWithoutDatabase(t *testing.T) {
	// Without a database the page stays usable and says what is missing.
	h := newHandler(&memStore{players: &memPlayers{}, pingErr: errors.New("database gone")})

	rec := get(t, h, "/fragments/status")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "nicht erreichbar") {
		t.Errorf("fragment does not report the outage: %s", rec.Body.String())
	}
}

func TestStaticAssetsAreEmbedded(t *testing.T) {
	// Catches exactly the fault the verify step later looks for in the image:
	// assets that did not make it into the container. The fonts are in here
	// because a missing one degrades silently — the page still renders, in
	// the wrong typeface, on whichever machine nobody checked.
	h := newHandler(newMemStore())

	for _, asset := range []string{
		"/static/js/htmx.min.js",
		"/static/css/app.css",
		"/static/fonts/space-grotesk-latin.woff2",
		"/static/fonts/jetbrains-mono-latin.woff2",
		"/static/img/mark-32.png",
		"/static/img/mark-180.png",
		"/static/img/mascot.png",
	} {
		rec := get(t, h, asset)

		if rec.Code != http.StatusOK {
			t.Errorf("%s: status = %d, want 200", asset, rec.Code)
			continue
		}
		if rec.Body.Len() == 0 {
			t.Errorf("%s is empty", asset)
		}
	}
}

func TestTheBrowserFindsAnIconWithoutBeingTold(t *testing.T) {
	// Browsers request /favicon.ico on their own, before they have parsed the
	// link elements. Answering it costs nothing and keeps the log readable.
	h := newHandler(newMemStore())

	rec := get(t, h, "/favicon.ico")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "image/png" {
		t.Errorf("content type = %q, want image/png", got)
	}
	// The eight magic bytes every PNG starts with: proves the route serves the
	// image rather than a rendered error page with a 200 on it.
	if got := rec.Body.Bytes(); len(got) < 8 || string(got[1:4]) != "PNG" {
		t.Errorf("body is not a PNG (%d bytes)", len(got))
	}
}

func TestTheMarkStandsBesideTheWordmark(t *testing.T) {
	h := newHandler(newMemStore())

	body := get(t, h, "/").Body.String()

	for _, want := range []string{
		`rel="icon"`,
		`rel="apple-touch-icon"`,
		`class="brand-logo"`,
		// Decorative on purpose: the wordmark beside it already says the name.
		`alt=""`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page does not contain %q", want)
		}
	}
}

// storeWithPlayers returns a store already holding n players.
func storeWithPlayers(t *testing.T, n int) *memStore {
	t.Helper()

	store := newMemStore()
	for i := range n {
		if _, err := store.Players().Create(t.Context(), fmt.Sprintf("Spieler %d", i+1), domain.DefaultTTR); err != nil {
			t.Fatalf("seeding player %d: %v", i+1, err)
		}
	}
	return store
}

// join posts the form and returns the response.
func join(t *testing.T, h http.Handler, name string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()

	form := url.Values{"display_name": {name}}
	r := httptest.NewRequest(http.MethodPost, "/players", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range cookies {
		r.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

func sessionCookie(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()

	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.SessionCookieName {
			return c
		}
	}
	t.Fatal("the response carried no session cookie")
	return nil
}

func TestIndexShowsTheJoinFormWhenNobodyIsRecognised(t *testing.T) {
	rec := get(t, newHandler(newMemStore()), "/")

	body := rec.Body.String()
	for _, want := range []string{`hx-post="/players"`, `name="display_name"`, "Mitspielen"} {
		if !strings.Contains(body, want) {
			t.Errorf("the start page does not contain %q", want)
		}
	}
}

func TestJoinCreatesAPlayerAndStartsASession(t *testing.T) {
	store := newMemStore()
	h := newHandlerWith(store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))

	rec := join(t, h, "Anna")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Anna") {
		t.Errorf("the response does not name the new player: %s", rec.Body.String())
	}
	// The roster is refreshed out of band in the same response, so a
	// successful join needs no second request and no JavaScript.
	if !strings.Contains(rec.Body.String(), `hx-swap-oob="true"`) {
		t.Errorf("the response does not refresh the ranking out of band: %s", rec.Body.String())
	}
	if c := sessionCookie(t, rec); c.Value == "" {
		t.Error("the session cookie is empty")
	}

	n, err := store.Players().Count(t.Context())
	if err != nil {
		t.Fatalf("Count(): %v", err)
	}
	if n != 1 {
		t.Errorf("%d players exist, want 1", n)
	}
}

// TestAReturningBrowserIsRecognised is the Definition of Done of AP2: the
// same browser maps to the same player on a later request, through nothing
// but the cookie it kept.
func TestAReturningBrowserIsRecognised(t *testing.T) {
	store := newMemStore()
	sessions := auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false)
	h := newHandlerWith(store, sessions)

	cookie := sessionCookie(t, join(t, h, "Anna"))

	// A separate server value over the same store and key stands in for a
	// restarted process: nothing is carried over in memory.
	restarted := newHandlerWith(store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(cookie)
	rec := httptest.NewRecorder()
	restarted.ServeHTTP(rec, r)

	body := rec.Body.String()
	// Being recognised is now stated in the top bar rather than in a card of
	// its own, and the name leads to that player's own profile.
	if !strings.Contains(body, `class="whoami-name"`) || !strings.Contains(body, ">Anna<") {
		t.Errorf("the returning browser was not recognised: %s", body)
	}
	if strings.Contains(body, `name="display_name"`) {
		t.Error("the join form is still being shown to a recognised player")
	}
}

// TestTheBadgeCountsWhatIsWaiting: the top bar replaces a card that said
// "nothing to confirm", so the badge has to be the thing that says otherwise.
func TestTheBadgeCountsWhatIsWaiting(t *testing.T) {
	h, store, anna, bodo := twoBrowsers(t)

	// Nobody has entered anything, so there is nothing to announce.
	if body := get(t, h, "/fragments/whoami").Body.String(); strings.Contains(body, "whoami-badge") {
		t.Errorf("a badge turned up with nothing waiting: %s", body)
	}

	reportedByAnna(t, h, store, anna)

	forBodo := fragment(t, h, "/fragments/whoami", bodo).Body.String()
	if !strings.Contains(forBodo, `class="whoami-badge"`) || !strings.Contains(forBodo, ">1<") {
		t.Errorf("Bodo is not told a result waits on him: %s", forBodo)
	}
	// Anna entered it, so it does not wait on her.
	if forAnna := fragment(t, h, "/fragments/whoami", anna).Body.String(); strings.Contains(forAnna, "whoami-badge") {
		t.Errorf("Anna is counted for her own result: %s", forAnna)
	}

	// Confirming empties it again, in the same response.
	rec := post(t, h, "/matches/"+store.matches.all()[0].ID.String()+"/confirm", bodo)
	if !strings.Contains(rec.Body.String(), `id="whoami" hx-swap-oob="innerHTML"`) {
		t.Errorf("the ruling did not refresh the top bar: %s", rec.Body.String())
	}
	if after := fragment(t, h, "/fragments/whoami", bodo).Body.String(); strings.Contains(after, "whoami-badge") {
		t.Errorf("the badge outlived the confirmation: %s", after)
	}
}

// TestJoiningSaysItOnce keeps the onboarding line where it belongs: it
// answers a question somebody has exactly once, so it is not on the page
// afterwards.
func TestJoiningSaysItOnce(t *testing.T) {
	store := newMemStore()
	h := newHandlerWith(store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))

	rec := join(t, h, "Anna")
	body := rec.Body.String()
	if !strings.Contains(body, "erkennt dich wieder") {
		t.Errorf("joining does not explain the cookie: %s", body)
	}
	if !strings.Contains(body, `id="whoami" hx-swap-oob="innerHTML"`) {
		t.Errorf("joining does not put the name in the top bar: %s", body)
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(sessionCookie(t, rec))
	next := httptest.NewRecorder()
	h.ServeHTTP(next, r)

	if strings.Contains(next.Body.String(), "erkennt dich wieder") {
		t.Error("the onboarding line is still on the page on the next visit")
	}
}

// TestARestartWithADifferentKeyForgetsEverybody is the reason SP_SESSION_KEY
// has no default: a key generated per start would look like it works and quietly
// fail the Definition of Done.
func TestARestartWithADifferentKeyForgetsEverybody(t *testing.T) {
	store := newMemStore()
	h := newHandlerWith(store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))

	cookie := sessionCookie(t, join(t, h, "Anna"))

	other := newHandlerWith(store, auth.NewCookieAuthenticator(store.Identities(),
		[]byte("a-completely-different-session-key"), false))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(cookie)
	rec := httptest.NewRecorder()
	other.ServeHTTP(rec, r)

	if !strings.Contains(rec.Body.String(), `name="display_name"`) {
		t.Error("a cookie signed with another key was accepted")
	}
}

func TestJoinRejectsUnusableNames(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"only whitespace", "   "},
		{"too long", strings.Repeat("a", 41)},
		{"contains a newline", "Anna\nBodo"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := newMemStore()
			h := newHandlerWith(store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))

			rec := join(t, h, tc.input)

			if rec.Code != http.StatusUnprocessableEntity {
				t.Errorf("status = %d, want 422", rec.Code)
			}
			n, _ := store.Players().Count(t.Context())
			if n != 0 {
				t.Errorf("%d players were created, want 0", n)
			}
		})
	}
}

// TestJoinRejectsATakenName checks the message as well as the status: "an
// implausible input is rejected and the message says why" is the standard the
// forms in this project are held to.
func TestJoinRejectsATakenName(t *testing.T) {
	store := newMemStore()
	h := newHandlerWith(store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))

	join(t, h, "Anna")
	rec := join(t, h, "anna")

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "gibt es schon") {
		t.Errorf("the message does not say the name is taken: %s", body)
	}
	// What was typed comes back, so nobody retypes it.
	if !strings.Contains(body, `value="anna"`) {
		t.Errorf("the typed name was not returned: %s", body)
	}
}

func TestStandingsFragment(t *testing.T) {
	rec := get(t, newHandler(storeWithPlayers(t, 3)), "/fragments/standings")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	for _, want := range []string{"Spieler 1", "Spieler 2", "Spieler 3"} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("the ranking does not contain %q", want)
		}
	}
}

func TestStandingsMarksTheViewer(t *testing.T) {
	store := newMemStore()
	h := newHandlerWith(store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))

	cookie := sessionCookie(t, join(t, h, "Anna"))

	r := httptest.NewRequest(http.MethodGet, "/fragments/standings", nil)
	r.AddCookie(cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)

	if !strings.Contains(rec.Body.String(), "(du)") {
		t.Errorf("the ranking does not mark the viewer: %s", rec.Body.String())
	}
}
