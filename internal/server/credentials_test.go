package server_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
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
		// A nil entry means "no cookie", which is how a caller says
		// "as a stranger" in a table of cases.
		if c != nil {
			r.AddCookie(c)
		}
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

	before, err := store.Credentials().ForPlayer(t.Context(), anna.ID, domain.CredentialPIN)
	if err != nil {
		t.Fatalf("joining left no PIN: %v", err)
	}

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

	after, err := store.Credentials().ForPlayer(t.Context(), anna.ID, domain.CredentialPIN)
	if err != nil || after.Hash != before.Hash {
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

// The PIN is part of joining, not an offer after it (docs/adr/0018). Issue #88
// said the PIN carries the daily load because few people save a code; an
// offer somebody could scroll past left most players with the code alone.
func TestJoiningSetsThePIN(t *testing.T) {
	store := newMemStore()
	h := newHandlerWith(store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))

	rec := join(t, h, "Anna")
	if rec.Code != http.StatusOK {
		t.Fatalf("joining = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	players, _ := store.Players().List(t.Context())
	anna := players[0]

	stored, err := store.Credentials().ForPlayer(t.Context(), anna.ID, domain.CredentialPIN)
	if err != nil {
		t.Fatalf("joining stored no PIN: %v", err)
	}
	if ok, err := credential.Verify(stored.Hash, testPIN); err != nil || !ok {
		t.Errorf("the stored PIN does not verify: %v, %v", ok, err)
	}

	body := rec.Body.String()
	if strings.Contains(body, testPIN) {
		t.Errorf("the response echoes the PIN: %s", body)
	}
	// Chosen already, so the card after joining does not ask again.
	if strings.Contains(body, `hx-post="/credentials/pin"`) {
		t.Errorf("joining still offers a PIN after one was chosen: %s", body)
	}

	if got := signIn(t, h, anna.ID.String(), testPIN).Code; got != http.StatusOK {
		t.Errorf("the PIN chosen at joining does not sign in: %d", got)
	}
}

// No PIN, no player. A player created first and asked afterwards is one closed
// tab away from having none.
func TestJoiningNeedsAPIN(t *testing.T) {
	store := newMemStore()
	h := newHandlerWith(store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))

	for _, pin := range []string{"", "12345", "Sommer2026!"} {
		rec := postForm(t, h, "/players", url.Values{"display_name": {"Anna"}, "pin": {pin}})
		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("joining with PIN %q = %d, want %d", pin, rec.Code, http.StatusUnprocessableEntity)
		}
		body := rec.Body.String()
		// The name survives, so only the PIN has to be typed again.
		if !strings.Contains(body, `value="Anna"`) {
			t.Errorf("a refused PIN lost the name: %s", body)
		}
		if pin != "" && strings.Contains(body, pin) {
			t.Errorf("the refusal echoes the PIN %q: %s", pin, body)
		}
		for _, c := range rec.Result().Cookies() {
			if c.Name == auth.SessionCookieName {
				t.Errorf("a refused join with PIN %q started a session", pin)
			}
		}
	}

	if players, _ := store.Players().List(t.Context()); len(players) != 0 {
		t.Errorf("a join without a usable PIN created %d players", len(players))
	}
}

// The code as a file, beside the code on the screen (docs/adr/0018). Made from
// the page itself, so there is nothing on the server that could hand it out
// again — and it has to hold the same code the screen shows, or the file is a
// way back that opens nothing.
func TestTheRecoveryCodeCanBeSavedAsAFile(t *testing.T) {
	store := newMemStore()
	h := newHandlerWith(store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))

	fileIn := func(t *testing.T, body string) string {
		t.Helper()
		if !strings.Contains(body, `download="schmetterpause-wiederherstellungscode.txt"`) {
			t.Fatalf("no download is offered: %s", body)
		}
		href := regexp.MustCompile(`href="data:text/plain;charset=utf-8,([^"]*)"`).FindStringSubmatch(body)
		if href == nil {
			t.Fatalf("the download is not a data: URL made from the page: %s", body)
		}
		text, err := url.PathUnescape(href[1])
		if err != nil {
			t.Fatalf("the file does not decode: %v", err)
		}
		return text
	}

	joined := join(t, h, "Anna")
	shown := codeInPage.FindStringSubmatch(joined.Body.String())
	file := fileIn(t, joined.Body.String())
	if !strings.Contains(file, shown[1]) || !strings.Contains(file, "Anna") {
		t.Errorf("the file does not hold Anna's code %q: %q", shown[1], file)
	}

	// And the same for a code issued later from the profile.
	reissued := postForm(t, h, "/credentials/recovery", nil, sessionCookie(t, joined))
	fresh := codeInPage.FindStringSubmatch(reissued.Body.String())
	if fresh == nil {
		t.Fatalf("no code came back: %s", reissued.Body.String())
	}
	if file := fileIn(t, reissued.Body.String()); !strings.Contains(file, fresh[1]) {
		t.Errorf("the file does not hold the fresh code %q: %q", fresh[1], file)
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
		// Nor the heading that introduces the two of them.
		if strings.Contains(body, "zwei Schlüssel") {
			t.Errorf("%s is told how anna gets back in", name)
		}
	}
}

// The other half of the feedback: the recovery code and the PIN sat next to
// each other on the profile with nothing saying which was which. Both cards
// explain themselves; what was missing is the sentence that tells them apart.
func TestTheProfileSaysHowTheTwoKeysDiffer(t *testing.T) {
	h, store, cookie := twoPlayers(t)
	anna, _ := playerIDs(t, store, "Anna", "Bodo")

	body := fragment(t, h, "/players/"+anna.String(), cookie).Body.String()

	for _, want := range []string{
		"Zugang", "zwei Schlüssel", "<strong>PIN</strong>",
		"<strong>Wiederherstellungscode</strong>",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the access section does not carry %q: %s", want, body)
		}
	}
	// The cards themselves are untouched, and both are still there.
	if !strings.Contains(body, `hx-post="/credentials/pin"`) {
		t.Errorf("the PIN form is gone: %s", body)
	}
	if !strings.Contains(body, `hx-post="/credentials/recovery"`) {
		t.Errorf("the way to a fresh recovery code is gone: %s", body)
	}
}
