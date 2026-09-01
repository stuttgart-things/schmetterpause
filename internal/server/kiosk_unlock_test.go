package server_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// The code goes in a form so it does not end up in the history and the
// autocomplete of a laptop somebody borrows next, and so a link nobody meant
// to share does not carry it.
func TestTheCodeFormUnlocksTheKiosk(t *testing.T) {
	h, _ := kioskHandler(t)

	rec := kioskPost(t, h, "/kiosk/unlock", nil, url.Values{"code": {testKioskToken}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("unlocking = %d, want 303: %s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); loc != "/kiosk" {
		t.Errorf("Location = %q, want /kiosk — a reload must not repeat the attempt", loc)
	}

	var grant *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == "schmetterpause_kiosk" {
			grant = c
		}
	}
	if grant == nil {
		t.Fatal("no grant was issued")
	}
	if body := kioskBody(t, h, grant); !strings.Contains(body, "Ergebnis eintragen") {
		t.Error("the grant does not open the kiosk")
	}
}

// A wrong code is a refusal that comes back as the same form, and the code is
// never echoed into the page.
func TestAWrongCodeIsRefusedWithoutEchoingIt(t *testing.T) {
	h, _ := kioskHandler(t)

	rec := kioskPost(t, h, "/kiosk/unlock", nil, url.Values{"code": {"nicht-das"}})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("got %d, want 401", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Das ist nicht der Zugangscode.") {
		t.Error("the refusal does not say what was wrong")
	}
	if strings.Contains(body, "nicht-das") || strings.Contains(body, testKioskToken) {
		t.Error("the page carries a code where the next borrower can read it")
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == "schmetterpause_kiosk" {
			t.Fatal("a wrong code issued a grant")
		}
	}
}

// Without a brake the door is an oracle for a shared secret, and the code is
// a word somebody chose, not sixteen random characters.
func TestGuessingAtTheCodeSlowsDown(t *testing.T) {
	h, _ := kioskHandler(t)

	// Three failures pass free, so the fourth attempt is still answered and
	// the fifth is the one that has to wait. Somebody mistyping a code twice
	// is not an attack, which is what Free is for.
	for i := range 4 {
		if rec := kioskPost(t, h, "/kiosk/unlock", nil, url.Values{"code": {"falsch"}}); rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d got %d, want 401", i+1, rec.Code)
		}
	}
	rec := kioskPost(t, h, "/kiosk/unlock", nil, url.Values{"code": {"falsch"}})
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("the fifth wrong code got %d, want 429", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("a client that reads Retry-After learns nothing")
	}

	// The brake is on the code, not on the door: the right code still has to
	// wait, or a wrong guess would be a way to check whether somebody else is
	// being slowed down.
	if rec := kioskPost(t, h, "/kiosk/unlock", nil, url.Values{"code": {testKioskToken}}); rec.Code != http.StatusTooManyRequests {
		t.Errorf("the right code got %d during the wait, want 429", rec.Code)
	}
}

// The query string is the other door to the same lock. A brake fitted to one
// of them that could be walked around through the other is not a brake.
func TestTheQueryStringIsBrakedToo(t *testing.T) {
	h, _ := kioskHandler(t)

	for range 5 {
		get(t, h, "/kiosk?token=falsch")
	}
	if rec := kioskPost(t, h, "/kiosk/unlock", nil, url.Values{"code": {testKioskToken}}); rec.Code != http.StatusTooManyRequests {
		t.Errorf("the form got %d after failures in the query, want 429", rec.Code)
	}
}

// An unset token removes the routes rather than leaving them open, and that
// has to include the new one.
func TestUnlockingDoesNotExistWithoutAToken(t *testing.T) {
	h := newHandler(newMemStore())

	if rec := kioskPost(t, h, "/kiosk/unlock", nil, url.Values{"code": {"x"}}); rec.Code != http.StatusNotFound {
		t.Errorf("POST /kiosk/unlock = %d, want 404", rec.Code)
	}
}
