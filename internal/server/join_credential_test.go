package server_test

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/stuttgart-things/schmetterpause/internal/auth"
	"github.com/stuttgart-things/schmetterpause/internal/credential"
	"github.com/stuttgart-things/schmetterpause/internal/domain"
)

// codeInPage picks the recovery code out of the response. It is the only
// place in the application where the code is readable, which is what makes
// the pattern this narrow.
var codeInPage = regexp.MustCompile(`<code>([0-9A-Z-]{16,})</code>`)

func TestJoiningIssuesARecoveryCode(t *testing.T) {
	store := newMemStore()
	h := newHandlerWith(store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))

	rec := join(t, h, "Anna")
	body := rec.Body.String()

	match := codeInPage.FindStringSubmatch(body)
	if match == nil {
		t.Fatalf("joining showed no recovery code: %s", body)
	}
	shown := match[1]

	// It has to be stored against the player that was just created, or the
	// code on the screen opens nothing.
	players, err := store.Players().List(t.Context())
	if err != nil {
		t.Fatalf("List(): %v", err)
	}
	if len(players) != 1 {
		t.Fatalf("List() = %d players, want 1", len(players))
	}

	stored, err := store.Credentials().ForPlayer(t.Context(), players[0].ID, domain.CredentialRecovery)
	if err != nil {
		t.Fatalf("the new player has no recovery credential: %v", err)
	}

	ok, err := credential.Verify(stored.Hash, credential.NormalizeCode(shown))
	if err != nil {
		t.Fatalf("Verify(): %v", err)
	}
	if !ok {
		t.Errorf("the stored hash does not match the code that was shown (%q)", shown)
	}

	// Never in the clear. A hash that contained the code would make the
	// database a list of valid credentials.
	if strings.Contains(stored.Hash, credential.NormalizeCode(shown)) {
		t.Error("the stored hash contains the code in the clear")
	}
}

// Once means once. docs/adr/0006 accepts that a code nobody saved is gone —
// what it does not accept is the code being readable again later, because
// then it is no longer a secret the player alone holds.
func TestTheRecoveryCodeIsShownOnlyOnce(t *testing.T) {
	store := newMemStore()
	h := newHandlerWith(store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))

	rec := join(t, h, "Anna")
	shown := codeInPage.FindStringSubmatch(rec.Body.String())
	if shown == nil {
		t.Fatal("joining showed no recovery code")
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(sessionCookie(t, rec))
	next := httptest.NewRecorder()
	h.ServeHTTP(next, r)

	if strings.Contains(next.Body.String(), shown[1]) {
		t.Error("the recovery code is still on the page on the next visit")
	}
}

// Two players must not share a code, or one of them can sign in as the other
// by typing what is on their own screen.
func TestEveryPlayerGetsTheirOwnCode(t *testing.T) {
	store := newMemStore()
	h := newHandlerWith(store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))

	first := codeInPage.FindStringSubmatch(join(t, h, "Anna").Body.String())
	second := codeInPage.FindStringSubmatch(join(t, h, "Bodo").Body.String())

	if first == nil || second == nil {
		t.Fatal("one of the joins showed no recovery code")
	}
	if first[1] == second[1] {
		t.Errorf("both players were given the same code %q", first[1])
	}
}

// A rejected join must leave nothing behind: no player, and no credential for
// the player it did not create.
func TestARejectedJoinIssuesNoCode(t *testing.T) {
	store := newMemStore()
	h := newHandlerWith(store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))

	join(t, h, "Anna")

	rec := join(t, h, "Anna")
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("joining under a taken name = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	if codeInPage.MatchString(rec.Body.String()) {
		t.Errorf("a rejected join showed a recovery code: %s", rec.Body.String())
	}
}
