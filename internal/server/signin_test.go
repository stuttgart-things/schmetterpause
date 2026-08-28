package server_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stuttgart-things/schmetterpause/internal/auth"
	"github.com/stuttgart-things/schmetterpause/internal/credential"
	"github.com/stuttgart-things/schmetterpause/internal/domain"
)

func signIn(t *testing.T, h http.Handler, playerID, secret string) *httptest.ResponseRecorder {
	t.Helper()

	form := url.Values{"player_id": {playerID}, "secret": {secret}}
	r := httptest.NewRequest(http.MethodPost, "/signin", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

// recognisedAs returns the name the start page greets this browser with, or
// the empty string for a browser it does not recognise.
func recognisedAs(t *testing.T, h http.Handler, cookies ...*http.Cookie) string {
	t.Helper()

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range cookies {
		r.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)

	body := rec.Body.String()
	_, after, found := strings.Cut(body, "<h1>Hallo, ")
	if !found {
		return ""
	}
	name, _, _ := strings.Cut(after, "<")
	return name
}

// This is issue #70, start to finish: a player joins, their browser forgets
// the cookie, and they get back to their own player rather than being told
// their name is taken.
func TestLosingTheCookieIsNoLongerADeadEnd(t *testing.T) {
	store := newMemStore()
	h := newHandlerWith(store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))

	joined := join(t, h, "Anna")
	code := codeInPage.FindStringSubmatch(joined.Body.String())
	if code == nil {
		t.Fatal("joining showed no recovery code")
	}

	players, err := store.Players().List(t.Context())
	if err != nil || len(players) != 1 {
		t.Fatalf("List() = %v, %v", players, err)
	}
	anna := players[0]

	// The browser has nothing left. Before this change that was the end of it.
	if got := recognisedAs(t, h); got != "" {
		t.Fatalf("a browser with no cookie is recognised as %q", got)
	}

	rec := signIn(t, h, anna.ID.String(), code[1])
	if rec.Code != http.StatusOK {
		t.Fatalf("signIn() = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	if got := recognisedAs(t, h, sessionCookie(t, rec)); got != "Anna" {
		t.Errorf("after signing in the page greets %q, want %q", got, "Anna")
	}
}

// What was printed is not what gets typed back. Every one of these is the
// same code as far as the person holding it is concerned.
func TestSigningInAcceptsTheCodeAsItGetsTyped(t *testing.T) {
	store := newMemStore()
	h := newHandlerWith(store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))

	code := codeInPage.FindStringSubmatch(join(t, h, "Anna").Body.String())
	if code == nil {
		t.Fatal("joining showed no recovery code")
	}
	printed := code[1]

	players, _ := store.Players().List(t.Context())
	anna := players[0]

	typed := map[string]string{
		"as printed":       printed,
		"lower case":       strings.ToLower(printed),
		"without hyphens":  strings.ReplaceAll(printed, "-", ""),
		"spaces instead":   strings.ReplaceAll(printed, "-", " "),
		"with a stray tab": "\t" + printed + " ",
	}

	for name, secret := range typed {
		t.Run(name, func(t *testing.T) {
			rec := signIn(t, h, anna.ID.String(), secret)
			if rec.Code != http.StatusOK {
				t.Errorf("signIn(%q) = %d, want %d", secret, rec.Code, http.StatusOK)
			}
		})
	}
}

func TestSigningInRefusesAWrongSecret(t *testing.T) {
	store := newMemStore()
	h := newHandlerWith(store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))

	join(t, h, "Anna")
	players, _ := store.Players().List(t.Context())
	anna := players[0]

	for _, secret := range []string{"XXXX-XXXX-XXXX-XXXX", "123456", "x"} {
		rec := signIn(t, h, anna.ID.String(), secret)

		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("signIn(%q) = %d, want %d", secret, rec.Code, http.StatusUnprocessableEntity)
		}
		for _, c := range rec.Result().Cookies() {
			if c.Name == auth.SessionCookieName && c.Value != "" {
				t.Errorf("signIn(%q) handed out a session cookie", secret)
			}
		}
	}
}

// The wording is the same whatever went wrong. Anything else answers a
// question about somebody else's account — here, whether they have a PIN.
func TestARefusalSaysNothingAboutTheAccount(t *testing.T) {
	store := newMemStore()
	h := newHandlerWith(store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))

	join(t, h, "Anna")
	players, _ := store.Players().List(t.Context())
	anna := players[0]

	// Anna has a recovery code and no PIN. Bodo, created at the kiosk, has
	// neither. A stranger asking must not be able to tell the two apart.
	bodo, err := store.Players().Create(t.Context(), "Bodo", domain.DefaultTTR)
	if err != nil {
		t.Fatalf("Create(): %v", err)
	}

	withCode := signIn(t, h, anna.ID.String(), "123456").Body.String()
	withNothing := signIn(t, h, bodo.ID.String(), "123456").Body.String()

	const refusal = "Das passt nicht."
	if !strings.Contains(withCode, refusal) || !strings.Contains(withNothing, refusal) {
		t.Fatalf("the refusal wording changed:\n%s\n---\n%s", withCode, withNothing)
	}
	if strings.Contains(withNothing, "PIN oder Code geht es nicht") {
		t.Error("a player without credentials is answered differently")
	}
}

func TestSigningInNeedsBothHalves(t *testing.T) {
	store := newMemStore()
	h := newHandlerWith(store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))

	code := codeInPage.FindStringSubmatch(join(t, h, "Anna").Body.String())
	players, _ := store.Players().List(t.Context())
	anna := players[0]

	tests := []struct{ name, playerID, secret string }{
		{"no name", "", code[1]},
		{"a name that is not a uuid", "Anna", code[1]},
		{"no secret", anna.ID.String(), ""},
		{"a name nobody has", "3f9d3b1e-0000-4000-8000-000000000000", code[1]},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := signIn(t, h, tc.playerID, tc.secret)
			if rec.Code != http.StatusUnprocessableEntity {
				t.Errorf("signIn() = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
			}
		})
	}
}

// Signing in on a new phone must not sign the browser at home out. A player
// holds several identities by design (ADR-0003), and a way back that costs
// the device you still have is a way back people avoid using.
func TestSigningInLeavesTheOtherBrowserAlone(t *testing.T) {
	store := newMemStore()
	h := newHandlerWith(store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))

	joined := join(t, h, "Anna")
	first := sessionCookie(t, joined)
	code := codeInPage.FindStringSubmatch(joined.Body.String())

	players, _ := store.Players().List(t.Context())
	anna := players[0]

	second := sessionCookie(t, signIn(t, h, anna.ID.String(), code[1]))

	if first.Value == second.Value {
		t.Error("signing in reused the first browser's subject, so the two devices are one session")
	}
	for i, c := range []*http.Cookie{first, second} {
		if got := recognisedAs(t, h, c); got != "Anna" {
			t.Errorf("browser %d is recognised as %q, want %q", i+1, got, "Anna")
		}
	}
}

// The picker is the first half of the form, so it has to hold everybody —
// including players the kiosk created, who have no credential yet but will
// once somebody issues them one.
func TestTheSignInFormListsEverybodyByName(t *testing.T) {
	store := newMemStore()
	h := newHandlerWith(store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))

	for _, name := range []string{"Zoe", "anna", "Bodo"} {
		if _, err := store.Players().Create(t.Context(), name, domain.DefaultTTR); err != nil {
			t.Fatalf("Create(%q): %v", name, err)
		}
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/fragments/signin", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /fragments/signin = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	order := []string{"anna", "Bodo", "Zoe"}
	at := -1
	for _, name := range order {
		i := strings.Index(body, ">"+name+"<")
		if i < 0 {
			t.Fatalf("%q is not in the picker: %s", name, body)
		}
		if i < at {
			t.Errorf("%q comes out of order; the picker is not sorted by name", name)
		}
		at = i
	}
}

// The start page has to offer the way back, or nobody finds it.
func TestTheStartPageOffersSigningIn(t *testing.T) {
	store := newMemStore()
	h := newHandlerWith(store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if !strings.Contains(rec.Body.String(), `hx-get="/fragments/signin"`) {
		t.Errorf("a stranger is not offered a way to sign in: %s", rec.Body.String())
	}
}

// A PIN is the other kind the same field takes. Nothing sets one yet — that
// is the next step — so this puts one in through the repository directly and
// checks the form does not care which kind it got.
func TestSigningInAlsoTakesAPIN(t *testing.T) {
	store := newMemStore()
	h := newHandlerWith(store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))

	join(t, h, "Anna")
	players, _ := store.Players().List(t.Context())
	anna := players[0]

	if err := store.Credentials().Put(
		t.Context(), anna.ID, domain.CredentialPIN, credential.Hash("246813")); err != nil {
		t.Fatalf("Put(): %v", err)
	}

	rec := signIn(t, h, anna.ID.String(), "246813")
	if rec.Code != http.StatusOK {
		t.Fatalf("signIn() with a PIN = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := recognisedAs(t, h, sessionCookie(t, rec)); got != "Anna" {
		t.Errorf("after signing in with a PIN the page greets %q, want %q", got, "Anna")
	}
}
