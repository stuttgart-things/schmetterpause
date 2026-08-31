package server_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stuttgart-things/schmetterpause/internal/auth"
	"github.com/stuttgart-things/schmetterpause/internal/credential"
	"github.com/stuttgart-things/schmetterpause/internal/domain"
	"github.com/stuttgart-things/schmetterpause/internal/server"
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

// TestTheStartPageOpensOnSignInOnceAnybodyIsOnTheRoster is the swap: joining
// happens once per person, a browser forgetting them happens again and again,
// so the picker is the card and joining is the link under it.
func TestTheStartPageOpensOnSignInOnceAnybodyIsOnTheRoster(t *testing.T) {
	store := newMemStore()
	h := newHandlerWith(store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))

	if _, err := store.Players().Create(t.Context(), "Anna", domain.DefaultTTR); err != nil {
		t.Fatalf("Create(): %v", err)
	}

	body := get(t, h, "/").Body.String()

	if !strings.Contains(body, `name="player_id"`) {
		t.Errorf("the start page does not open on the name picker: %s", body)
	}
	if !strings.Contains(body, ">Anna<") {
		t.Errorf("the picker does not list anybody: %s", body)
	}
	// And the other door is still there, one click away.
	if !strings.Contains(body, `hx-get="/fragments/join"`) {
		t.Errorf("a newcomer is not offered a way to join: %s", body)
	}
	if strings.Contains(body, `name="display_name"`) {
		t.Errorf("both forms are on the page at once: %s", body)
	}
}

// TestAnEmptyRosterOpensOnJoining is the case a picker cannot serve: with
// nobody to pick, "Namen wählen …" is a dead end, and the first player has to
// be able to get in.
func TestAnEmptyRosterOpensOnJoining(t *testing.T) {
	store := newMemStore()
	h := newHandlerWith(store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))

	body := get(t, h, "/").Body.String()

	if !strings.Contains(body, `name="display_name"`) {
		t.Errorf("the first player is offered no way in: %s", body)
	}
	if strings.Contains(body, `name="player_id"`) {
		t.Errorf("an empty roster still renders a picker with nothing in it: %s", body)
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

// The brake is a shipping condition, not a nicety (ADR-0007).
//
// Three failures cost nothing — mistyping a sixteen-character code is what
// the person this whole way back exists for actually does. The fourth earns
// a wait, and the fifth attempt is the one that runs into it.
func TestSigningInSlowsDownAfterWrongGuesses(t *testing.T) {
	store := newMemStore()
	h := newHandlerWith(store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))

	join(t, h, "Anna")
	players, _ := store.Players().List(t.Context())
	anna := players[0]

	for i := range 4 {
		if got := signIn(t, h, anna.ID.String(), "XXXX-XXXX-XXXX-XXXX").Code; got != http.StatusUnprocessableEntity {
			t.Fatalf("guess %d = %d, want %d", i+1, got, http.StatusUnprocessableEntity)
		}
	}

	rec := signIn(t, h, anna.ID.String(), "XXXX-XXXX-XXXX-XXXX")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("the fifth guess = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
	if got := rec.Header().Get("Retry-After"); got == "" || got == "0" {
		t.Errorf("Retry-After = %q, want a number of seconds", got)
	}
	if !strings.Contains(rec.Body.String(), "Zu viele Fehlversuche") {
		t.Errorf("the refusal does not say what happened: %s", rec.Body.String())
	}
}

// Being held back must not become being locked out. The right code has to
// work again once the wait has run out — this is the property that keeps the
// brake from rebuilding issue #70.
func TestTheBrakeLetsGoAgain(t *testing.T) {
	store := newMemStore()
	h := newHandlerWith(store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))

	code := codeInPage.FindStringSubmatch(join(t, h, "Anna").Body.String())
	players, _ := store.Players().List(t.Context())
	anna := players[0]

	for range 4 {
		signIn(t, h, anna.ID.String(), "XXXX-XXXX-XXXX-XXXX")
	}

	// Even the right code is held back now. That is the point: the brake
	// cannot know which this is without paying for the guess.
	blocked := signIn(t, h, anna.ID.String(), code[1])
	if blocked.Code != http.StatusTooManyRequests {
		t.Fatalf("the brake is not holding: %d", blocked.Code)
	}

	// The wait is two seconds at this point, per signInPlayerPolicy.
	wait, err := strconv.Atoi(blocked.Header().Get("Retry-After"))
	if err != nil {
		t.Fatalf("Retry-After is not a number: %v", err)
	}
	if wait > 5 {
		t.Fatalf("the first wait is %ds, too long to sit out in a test", wait)
	}
	time.Sleep(time.Duration(wait)*time.Second + 200*time.Millisecond)

	rec := signIn(t, h, anna.ID.String(), code[1])
	if rec.Code != http.StatusOK {
		t.Fatalf("after sitting out the wait, signIn() = %d, want %d: %s",
			rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := recognisedAs(t, h, sessionCookie(t, rec)); got != "Anna" {
		t.Errorf("the page greets %q, want %q", got, "Anna")
	}
}

// One player's wrong guesses must not slow down anybody else, or a single
// person hammering the form takes the office out with them.
func TestOnePlayersGuessesDoNotBlockAnother(t *testing.T) {
	store := newMemStore()
	h := newHandlerWith(store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))

	join(t, h, "Anna")
	bodoJoin := join(t, h, "Bodo")
	bodoCode := codeInPage.FindStringSubmatch(bodoJoin.Body.String())

	players, _ := store.Players().List(t.Context())
	var anna, bodo domain.Player
	for _, p := range players {
		switch p.DisplayName {
		case "Anna":
			anna = p
		case "Bodo":
			bodo = p
		}
	}

	for range 4 {
		signIn(t, h, anna.ID.String(), "XXXX-XXXX-XXXX-XXXX")
	}
	if signIn(t, h, anna.ID.String(), "XXXX-XXXX-XXXX-XXXX").Code != http.StatusTooManyRequests {
		t.Fatal("anna is not being held back")
	}

	if got := signIn(t, h, bodo.ID.String(), bodoCode[1]).Code; got != http.StatusOK {
		t.Errorf("bodo is held back by anna's guesses: %d", got)
	}
}

// A right answer clears the count, so the next honest mistake starts from the
// free tries again rather than from where the guessing left off.
func TestSigningInClearsTheCount(t *testing.T) {
	store := newMemStore()
	h := newHandlerWith(store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))

	code := codeInPage.FindStringSubmatch(join(t, h, "Anna").Body.String())
	players, _ := store.Players().List(t.Context())
	anna := players[0]

	for range 3 {
		signIn(t, h, anna.ID.String(), "XXXX-XXXX-XXXX-XXXX")
	}
	if got := signIn(t, h, anna.ID.String(), code[1]).Code; got != http.StatusOK {
		t.Fatalf("signIn() with the right code = %d, want %d", got, http.StatusOK)
	}

	// The free tries are back, rather than the count carrying on from three.
	for i := range 4 {
		if got := signIn(t, h, anna.ID.String(), "XXXX-XXXX-XXXX-XXXX").Code; got != http.StatusUnprocessableEntity {
			t.Fatalf("guess %d after a success = %d, want %d", i+1, got, http.StatusUnprocessableEntity)
		}
	}
}

func TestWaitInWords(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   time.Duration
		want string
	}{
		{500 * time.Millisecond, "einer Sekunde"},
		{time.Second, "einer Sekunde"},
		{2 * time.Second, "2 Sekunden"},
		{2500 * time.Millisecond, "3 Sekunden"},
		{59 * time.Second, "59 Sekunden"},
		{time.Minute, "einer Minute"},
		{90 * time.Second, "2 Minuten"},
		{5 * time.Minute, "5 Minuten"},
	}

	for _, tc := range tests {
		if got := server.WaitInWords(tc.in); got != tc.want {
			t.Errorf("WaitInWords(%s) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
