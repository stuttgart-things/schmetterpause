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

// postForm is the form-encoded sibling of post in pending_test.go, which
// sends nothing at all.
func postForm(t *testing.T, h http.Handler, path string, form url.Values, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()

	r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range cookies {
		r.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

func TestSettingAPIN(t *testing.T) {
	store := newMemStore()
	h := newHandlerWith(store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))

	joined := join(t, h, "Anna")
	cookie := sessionCookie(t, joined)
	players, _ := store.Players().List(t.Context())
	anna := players[0]

	rec := postForm(t, h, "/credentials/pin", url.Values{"pin": {"246813"}}, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("setting a PIN = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	stored, err := store.Credentials().ForPlayer(t.Context(), anna.ID, domain.CredentialPIN)
	if err != nil {
		t.Fatalf("no PIN was stored: %v", err)
	}
	if ok, err := credential.Verify(stored.Hash, "246813"); err != nil || !ok {
		t.Errorf("the stored PIN does not verify: %v, %v", ok, err)
	}
	if strings.Contains(stored.Hash, "246813") {
		t.Error("the stored hash contains the PIN in the clear")
	}

	// It must not be handed back into the page either.
	if strings.Contains(rec.Body.String(), "246813") {
		t.Errorf("the response echoes the PIN: %s", rec.Body.String())
	}
}

// The PIN sits on top of the code, it does not replace it. Somebody who sets
// one and then forgets it still has the code, and that is the whole reason
// both exist.
func TestSettingAPINLeavesTheRecoveryCodeAlone(t *testing.T) {
	store := newMemStore()
	h := newHandlerWith(store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))

	joined := join(t, h, "Anna")
	cookie := sessionCookie(t, joined)
	code := codeInPage.FindStringSubmatch(joined.Body.String())

	players, _ := store.Players().List(t.Context())
	anna := players[0]

	before, err := store.Credentials().ForPlayer(t.Context(), anna.ID, domain.CredentialRecovery)
	if err != nil {
		t.Fatalf("ForPlayer(): %v", err)
	}

	postForm(t, h, "/credentials/pin", url.Values{"pin": {"246813"}}, cookie)

	after, err := store.Credentials().ForPlayer(t.Context(), anna.ID, domain.CredentialRecovery)
	if err != nil {
		t.Fatalf("the recovery code is gone after setting a PIN: %v", err)
	}
	if after.Hash != before.Hash {
		t.Error("setting a PIN changed the recovery code")
	}
	if got := signIn(t, h, anna.ID.String(), code[1]).Code; got != http.StatusOK {
		t.Errorf("the recovery code no longer signs in: %d", got)
	}
}

// Digits only, and that is not pedantry: ADR-0006 refuses a self-chosen secret
// because a field somebody may type anything into becomes a field somebody
// types their company password into. The shape of the field is the answer.
func TestAPINTakesOnlyDigits(t *testing.T) {
	store := newMemStore()
	h := newHandlerWith(store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))

	cookie := sessionCookie(t, join(t, h, "Anna"))
	players, _ := store.Players().List(t.Context())
	anna := players[0]

	tests := []struct{ name, pin string }{
		{"empty", ""},
		{"too short", "12345"},
		{"letters", "geheim"},
		{"a password", "Sommer2026!"},
		{"digits with a letter", "12345a"},
		{"spaces inside", "123 456"},
		{"far too long", strings.Repeat("1", 33)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := postForm(t, h, "/credentials/pin", url.Values{"pin": {tc.pin}}, cookie)
			if rec.Code != http.StatusUnprocessableEntity {
				t.Errorf("setting %q = %d, want %d", tc.pin, rec.Code, http.StatusUnprocessableEntity)
			}
		})
	}

	if _, err := store.Credentials().ForPlayer(t.Context(), anna.ID, domain.CredentialPIN); err == nil {
		t.Error("a refused PIN was stored anyway")
	}
}

// A PIN somebody else knows is not a PIN. Nothing may set one but the player,
// from their own session (ADR-0007, open point 3).
func TestOnlyASignedInPlayerCanSetAPIN(t *testing.T) {
	store := newMemStore()
	h := newHandlerWith(store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))

	join(t, h, "Anna")

	rec := postForm(t, h, "/credentials/pin", url.Values{"pin": {"246813"}})
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("setting a PIN without a session = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestReplacingAPIN(t *testing.T) {
	store := newMemStore()
	h := newHandlerWith(store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))

	cookie := sessionCookie(t, join(t, h, "Anna"))
	players, _ := store.Players().List(t.Context())
	anna := players[0]

	postForm(t, h, "/credentials/pin", url.Values{"pin": {"246813"}}, cookie)
	postForm(t, h, "/credentials/pin", url.Values{"pin": {"975310"}}, cookie)

	if got := signIn(t, h, anna.ID.String(), "975310").Code; got != http.StatusOK {
		t.Errorf("the new PIN does not sign in: %d", got)
	}
	if got := signIn(t, h, anna.ID.String(), "246813").Code; got == http.StatusOK {
		t.Error("the replaced PIN still signs in")
	}
}

// No kiosk runs in an ordinary week, so a way to get a fresh code that only
// existed on tournament evenings would be no way at all from Wednesday to
// Tuesday (ADR-0006).
func TestIssuingYourselfANewRecoveryCode(t *testing.T) {
	store := newMemStore()
	h := newHandlerWith(store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))

	joined := join(t, h, "Anna")
	cookie := sessionCookie(t, joined)
	old := codeInPage.FindStringSubmatch(joined.Body.String())

	players, _ := store.Players().List(t.Context())
	anna := players[0]

	rec := postForm(t, h, "/credentials/recovery", nil, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("issuing a code = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	fresh := codeInPage.FindStringSubmatch(rec.Body.String())
	if fresh == nil {
		t.Fatalf("no code came back: %s", rec.Body.String())
	}
	if fresh[1] == old[1] {
		t.Fatal("the same code came back, so nothing was replaced")
	}

	if got := signIn(t, h, anna.ID.String(), fresh[1]).Code; got != http.StatusOK {
		t.Errorf("the new code does not sign in: %d", got)
	}
	// A new code invalidates the old one immediately. That is the whole
	// reason it is worth being able to issue one.
	if got := signIn(t, h, anna.ID.String(), old[1]).Code; got == http.StatusOK {
		t.Error("the replaced code still signs in")
	}
}

func TestOnlyASignedInPlayerCanIssueACode(t *testing.T) {
	store := newMemStore()
	h := newHandlerWith(store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))

	join(t, h, "Anna")

	if got := postForm(t, h, "/credentials/recovery", nil).Code; got != http.StatusUnauthorized {
		t.Errorf("issuing a code without a session = %d, want %d", got, http.StatusUnauthorized)
	}
}

// The offer has to be where somebody lands, or nobody sets one. Issue #88 is
// explicit that it must not end up buried behind the code.
func TestJoiningOffersAPIN(t *testing.T) {
	store := newMemStore()
	h := newHandlerWith(store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))

	body := join(t, h, "Anna").Body.String()

	if !strings.Contains(body, `hx-post="/credentials/pin"`) {
		t.Errorf("joining does not offer a PIN: %s", body)
	}
	// And it must not take the code with it when it swaps.
	if !strings.Contains(body, `hx-target="#pin-offer"`) {
		t.Error("the PIN form does not swap itself, so setting one would remove the code")
	}
}

// Your own page carries the way back; somebody else's does not.
func TestTheAccessSectionIsOnlyOnYourOwnProfile(t *testing.T) {
	store := newMemStore()
	h := newHandlerWith(store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))

	annaCookie := sessionCookie(t, join(t, h, "Anna"))
	bodoCookie := sessionCookie(t, join(t, h, "Bodo"))

	players, _ := store.Players().List(t.Context())
	var anna domain.Player
	for _, p := range players {
		if p.DisplayName == "Anna" {
			anna = p
		}
	}

	get := func(cookie *http.Cookie) string {
		r := httptest.NewRequest(http.MethodGet, "/players/"+anna.ID.String(), nil)
		if cookie != nil {
			r.AddCookie(cookie)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		return rec.Body.String()
	}

	if own := get(annaCookie); !strings.Contains(own, `hx-post="/credentials/pin"`) {
		t.Errorf("anna's own page does not offer a PIN: %s", own)
	}
	for name, body := range map[string]string{"bodo": get(bodoCookie), "a stranger": get(nil)} {
		if strings.Contains(body, `hx-post="/credentials/pin"`) {
			t.Errorf("%s is offered a PIN on anna's page", name)
		}
		if strings.Contains(body, `hx-post="/credentials/recovery"`) {
			t.Errorf("%s is offered a recovery code on anna's page", name)
		}
	}
}
