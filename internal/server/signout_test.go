package server_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stuttgart-things/schmetterpause/internal/auth"
	"github.com/stuttgart-things/schmetterpause/internal/domain"
)

// The case this exists for: somebody signs in on a colleague's phone to enter
// a result, and without it the phone stays them until the owner signs in
// again.
func TestSigningOutMakesTheBrowserAStrangerAgain(t *testing.T) {
	store := newMemStore()
	h := newHandlerWith(store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))

	cookie := sessionCookie(t, join(t, h, "Anna"))
	if got := recognisedAs(t, h, cookie); got != "Anna" {
		t.Fatalf("before signing out the page greets %q, want %q", got, "Anna")
	}

	rec := postForm(t, h, "/signout", nil, cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("signing out = %d, want %d", rec.Code, http.StatusSeeOther)
	}

	// The response clears the cookie, so the browser stops sending one.
	var cleared bool
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.SessionCookieName && c.Value == "" && c.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Error("the response does not clear the session cookie")
	}

	// And the old cookie no longer resolves, because the row behind it is
	// gone. Clearing only the browser would leave a credential in the table
	// that anybody replaying the cookie could still use.
	if got := recognisedAs(t, h, cookie); got != "" {
		t.Errorf("the old cookie still recognises %q", got)
	}
}

// Only this browser. A player holds several identities by design (ADR-0003),
// so signing out on a borrowed phone must leave the one at home signed in.
func TestSigningOutLeavesOtherDevicesAlone(t *testing.T) {
	store := newMemStore()
	h := newHandlerWith(store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))

	joined := join(t, h, "Anna")
	home := sessionCookie(t, joined)
	code := codeInPage.FindStringSubmatch(joined.Body.String())

	players, _ := store.Players().List(t.Context())
	anna := players[0]

	borrowed := sessionCookie(t, signIn(t, h, anna.ID.String(), code[1]))

	postForm(t, h, "/signout", nil, borrowed)

	if got := recognisedAs(t, h, borrowed); got != "" {
		t.Errorf("the borrowed phone still recognises %q", got)
	}
	if got := recognisedAs(t, h, home); got != "Anna" {
		t.Errorf("the phone at home was signed out too: %q", got)
	}
}

// The way back has to work, or this is issue #70 rebuilt by a button.
func TestSigningOutIsNotADeadEnd(t *testing.T) {
	store := newMemStore()
	h := newHandlerWith(store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))

	joined := join(t, h, "Anna")
	cookie := sessionCookie(t, joined)
	code := codeInPage.FindStringSubmatch(joined.Body.String())

	players, _ := store.Players().List(t.Context())
	anna := players[0]

	postForm(t, h, "/signout", nil, cookie)

	back := signIn(t, h, anna.ID.String(), code[1])
	if back.Code != http.StatusOK {
		t.Fatalf("signing back in = %d, want %d: %s", back.Code, http.StatusOK, back.Body.String())
	}
	if got := recognisedAs(t, h, sessionCookie(t, back)); got != "Anna" {
		t.Errorf("after signing back in the page greets %q, want %q", got, "Anna")
	}
}

// GET must not do it. Chat programs follow links to build previews, which is
// the same reason ADR-0006 makes the recovery code a code and not a link.
func TestSigningOutNeedsAPost(t *testing.T) {
	store := newMemStore()
	h := newHandlerWith(store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))

	cookie := sessionCookie(t, join(t, h, "Anna"))

	r := httptest.NewRequest(http.MethodGet, "/signout", nil)
	r.AddCookie(cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)

	if rec.Code == http.StatusSeeOther {
		t.Fatal("GET /signout signed somebody out")
	}
	if got := recognisedAs(t, h, cookie); got != "Anna" {
		t.Errorf("GET /signout ended the session: %q", got)
	}
}

func TestSigningOutNeedsASession(t *testing.T) {
	store := newMemStore()
	h := newHandlerWith(store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))

	if got := postForm(t, h, "/signout", nil).Code; got != http.StatusUnauthorized {
		t.Errorf("signing out without a session = %d, want %d", got, http.StatusUnauthorized)
	}
}

// The button names the price rather than asking "are you sure", and somebody
// without a PIN is told what it costs them — they are the one person who
// should think twice.
func TestTheSignOutCardNamesThePrice(t *testing.T) {
	store := newMemStore()
	h := newHandlerWith(store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))

	cookie := sessionCookie(t, join(t, h, "Anna"))
	players, _ := store.Players().List(t.Context())
	anna := players[0]

	page := func() string {
		r := httptest.NewRequest(http.MethodGet, "/players/"+anna.ID.String(), nil)
		r.AddCookie(cookie)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		return rec.Body.String()
	}

	withoutPIN := page()
	if !strings.Contains(withoutPIN, `action="/signout"`) {
		t.Fatalf("the profile does not offer signing out: %s", withoutPIN)
	}
	if !strings.Contains(withoutPIN, "Du hast keine PIN") {
		t.Error("somebody without a PIN is not warned")
	}

	postForm(t, h, "/credentials/pin", url.Values{"pin": {"246813"}}, cookie)

	withPIN := page()
	if strings.Contains(withPIN, "Du hast keine PIN") {
		t.Error("the warning survives setting a PIN")
	}
	if !strings.Contains(withPIN, "deine PIN oder deinen") {
		t.Errorf("somebody with a PIN is not told the way back: %s", withPIN)
	}
}

// Somebody else's profile offers nothing of the sort.
func TestSigningOutIsOnlyOnYourOwnProfile(t *testing.T) {
	store := newMemStore()
	h := newHandlerWith(store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))

	join(t, h, "Anna")
	bodoCookie := sessionCookie(t, join(t, h, "Bodo"))

	players, _ := store.Players().List(t.Context())
	var anna domain.Player
	for _, p := range players {
		if p.DisplayName == "Anna" {
			anna = p
		}
	}

	r := httptest.NewRequest(http.MethodGet, "/players/"+anna.ID.String(), nil)
	r.AddCookie(bodoCookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)

	if strings.Contains(rec.Body.String(), `action="/signout"`) {
		t.Error("bodo is offered a sign-out on anna's page")
	}
}
