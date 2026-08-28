package server_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stuttgart-things/schmetterpause/internal/auth"
	"github.com/stuttgart-things/schmetterpause/internal/server"
)

// timeNow is the clock the tests ask the store with. A function so a reader
// sees that the store's own answers depend on one.
func timeNow() time.Time { return time.Now() }

// adminAndKiosk wires a server with both the kiosk unlocked by a token and a
// bootstrap admin, which is what the revoke surface needs.
func adminAndKiosk(t *testing.T) (*server.Server, *memStore) {
	t.Helper()

	store := newMemStore()
	cfg := testConfig()
	cfg.SessionKey = testSessionKey
	cfg.KioskToken = testKioskToken
	cfg.BootstrapAdmin = "Anna"

	srv := server.New(cfg, store, discardLogger(),
		auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false), "test")
	return srv, store
}

// The heart of issue #77: the cookie used to be a derived constant, so every
// browser that had ever opened the token URL held the identical value and the
// laptop at the table was indistinguishable from a phone that read the token
// over a shoulder.
func TestEachMachineGetsItsOwnGrant(t *testing.T) {
	h, _ := kioskHandler(t)

	first := unlock(t, h)
	second := unlock(t, h)

	if first.Value == second.Value {
		t.Fatal("two machines were handed the same cookie value")
	}
	for i, c := range []*http.Cookie{first, second} {
		if got := kioskPost(t, h, "/kiosk/players", c, url.Values{"display_name": {"P" + string(rune('A'+i))}}).Code; got != http.StatusOK {
			t.Errorf("machine %d is not unlocked: %d", i+1, got)
		}
	}
}

// Revoking one leaves the other alone. Under the old constant, taking one
// browser back meant changing the token and restarting, which logged out the
// table along with everybody else.
func TestRevokingOneMachineLeavesTheOther(t *testing.T) {
	srv, store := adminAndKiosk(t)
	h := srv.Handler()

	annaCookie := sessionCookie(t, join(t, h, "Anna"))
	srv.GrantBootstrapAdmin(t.Context())

	table := unlock(t, h)
	shoulder := unlock(t, h)

	grants, err := store.KioskGrants().Active(t.Context(), timeNow())
	if err != nil || len(grants) != 2 {
		t.Fatalf("Active() = %v, %v, want two machines", grants, err)
	}

	// The second one is the one that walked away with the token.
	var target string
	for _, g := range grants {
		target = g.ID.String()
	}

	rec := postForm(t, h, "/admin/kiosk/"+target+"/revoke", nil, annaCookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("revoking = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body.String())
	}

	left, _ := store.KioskGrants().Active(t.Context(), timeNow())
	if len(left) != 1 {
		t.Fatalf("Active() = %d machines, want 1", len(left))
	}

	// Exactly one of the two cookies still works, and the other does not.
	var unlocked int
	for _, c := range []*http.Cookie{table, shoulder} {
		if kioskPost(t, h, "/kiosk/players", c, url.Values{"display_name": {"X"}}).Code == http.StatusOK {
			unlocked++
		}
	}
	if unlocked != 1 {
		t.Errorf("%d of the two machines are still unlocked, want 1", unlocked)
	}
}

// The answer to "somebody read the token over a shoulder" that does not
// involve a restart. It takes the table with it on purpose.
func TestRevokingAllMachines(t *testing.T) {
	srv, store := adminAndKiosk(t)
	h := srv.Handler()

	annaCookie := sessionCookie(t, join(t, h, "Anna"))
	srv.GrantBootstrapAdmin(t.Context())

	first := unlock(t, h)
	second := unlock(t, h)

	if got := postForm(t, h, "/admin/kiosk/revoke-all", nil, annaCookie).Code; got != http.StatusSeeOther {
		t.Fatalf("revoking all = %d, want %d", got, http.StatusSeeOther)
	}

	left, _ := store.KioskGrants().Active(t.Context(), timeNow())
	if len(left) != 0 {
		t.Errorf("Active() = %d machines, want none", len(left))
	}
	for i, c := range []*http.Cookie{first, second} {
		if got := kioskPost(t, h, "/kiosk/players", c, url.Values{"display_name": {"X"}}).Code; got != http.StatusForbidden {
			t.Errorf("machine %d is still unlocked: %d", i+1, got)
		}
	}

	// And the way back is the same as the way in: enter the token again.
	// A revocation must not be a restart in disguise.
	again := unlock(t, h)
	if got := kioskPost(t, h, "/kiosk/players", again, url.Values{"display_name": {"Y"}}).Code; got != http.StatusOK {
		t.Errorf("a revoked machine cannot unlock again: %d", got)
	}
}

// The surface belongs to somebody, which is what ADR-0008 settled and what
// #77 was waiting for.
func TestRevokingNeedsAnAdmin(t *testing.T) {
	srv, store := adminAndKiosk(t)
	h := srv.Handler()

	join(t, h, "Anna")
	bodoCookie := sessionCookie(t, join(t, h, "Bodo"))
	srv.GrantBootstrapAdmin(t.Context())

	unlock(t, h)
	grants, _ := store.KioskGrants().Active(t.Context(), timeNow())
	id := grants[0].ID.String()

	for name, cookie := range map[string]*http.Cookie{"a plain player": bodoCookie, "a stranger": nil} {
		got := postForm(t, h, "/admin/kiosk/"+id+"/revoke", nil, cookie).Code
		if got == http.StatusSeeOther {
			t.Errorf("%s revoked a kiosk machine", name)
		}
	}
	left, _ := store.KioskGrants().Active(t.Context(), timeNow())
	if len(left) != 1 {
		t.Errorf("Active() = %d, want the machine still unlocked", len(left))
	}
}

// "Which machines are kiosks right now" is the question #77 filed, and the
// derived cookie could not answer it at all.
func TestTheAdminPageListsUnlockedMachines(t *testing.T) {
	srv, _ := adminAndKiosk(t)
	h := srv.Handler()

	annaCookie := sessionCookie(t, join(t, h, "Anna"))
	srv.GrantBootstrapAdmin(t.Context())

	before := getWith(t, h, "/admin", annaCookie).Body.String()
	if !strings.Contains(before, "Gerade keins") {
		t.Errorf("with nothing unlocked the page does not say so: %s", before)
	}

	r := httptest.NewRequest(http.MethodGet, "/kiosk?token="+testKioskToken, nil)
	r.Header.Set("User-Agent", "Turnier-Laptop/1.0")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)

	after := getWith(t, h, "/admin", annaCookie).Body.String()
	if !strings.Contains(after, "Turnier-Laptop/1.0") {
		t.Errorf("the unlocked machine is not listed: %s", after)
	}
	if !strings.Contains(after, "/revoke") {
		t.Error("the list offers no way to take a machine back")
	}
}

// A user agent is whatever the caller says it is, so it must not be able to
// fill the page or pretend to be markup.
func TestTheMachineLabelIsBounded(t *testing.T) {
	srv, store := adminAndKiosk(t)
	h := srv.Handler()

	annaCookie := sessionCookie(t, join(t, h, "Anna"))
	srv.GrantBootstrapAdmin(t.Context())

	r := httptest.NewRequest(http.MethodGet, "/kiosk?token="+testKioskToken, nil)
	r.Header.Set("User-Agent", "<script>alert(1)</script>"+strings.Repeat("A", 500))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)

	grants, _ := store.KioskGrants().Active(t.Context(), timeNow())
	if len(grants) != 1 {
		t.Fatalf("Active() = %d, want 1", len(grants))
	}
	if len(grants[0].UserAgent) > 200 {
		t.Errorf("the label is %d characters, want it bounded", len(grants[0].UserAgent))
	}

	page := getWith(t, h, "/admin", annaCookie).Body.String()
	if strings.Contains(page, "<script>alert(1)</script>") {
		t.Error("the label reached the page as markup")
	}
}
