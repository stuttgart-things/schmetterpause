package server_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
)

// The kiosk is the last resort ADR-0006 names: somebody who has lost the code
// *and* every device that knew them holds nothing that can prove who they
// are, and what stands in for the proof is the room.
func TestTheKioskIssuesACodeForSomebodyElse(t *testing.T) {
	h, store := kioskHandler(t)
	kiosk := unlock(t, h)

	anna, err := store.Players().Create(t.Context(), "Anna", domain.DefaultTTR)
	if err != nil {
		t.Fatalf("Create(): %v", err)
	}

	rec := kioskPost(t, h, "/kiosk/credentials", kiosk, url.Values{"player_id": {anna.ID.String()}})
	if rec.Code != http.StatusOK {
		t.Fatalf("issuing a code = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	code := codeInPage.FindStringSubmatch(rec.Body.String())
	if code == nil {
		t.Fatalf("no code came back: %s", rec.Body.String())
	}
	// It says whose it is. A bare code on a laptop at a table tells nobody
	// who should be writing it down.
	if !strings.Contains(rec.Body.String(), "Neuer Code für Anna") {
		t.Error("the display does not name who the code is for")
	}

	// And it works: a player the kiosk created had no way onto their own
	// phone at all before this.
	if got := signIn(t, h, anna.ID.String(), code[1]).Code; got != http.StatusOK {
		t.Errorf("the issued code does not sign in: %d", got)
	}
}

// A player entered at the kiosk during a tournament holds no identity. The
// issued code is the whole path from there to their own phone.
func TestAKioskPlayerCanBeGivenAWayIn(t *testing.T) {
	h, store := kioskHandler(t)
	kiosk := unlock(t, h)

	kioskPost(t, h, "/kiosk/players", kiosk, url.Values{"display_name": {"Anna"}})

	players, err := store.Players().List(t.Context())
	if err != nil || len(players) != 1 {
		t.Fatalf("List() = %v, %v", players, err)
	}
	anna := players[0]

	// Creating them does not hand out a code: the laptop's screen is not
	// where a code belongs at a moment when the person may be three tables
	// away.
	if _, err := store.Credentials().ForPlayer(t.Context(), anna.ID, domain.CredentialRecovery); err == nil {
		t.Error("creating a player at the kiosk issued a code nobody asked for")
	}

	rec := kioskPost(t, h, "/kiosk/credentials", kiosk, url.Values{"player_id": {anna.ID.String()}})
	code := codeInPage.FindStringSubmatch(rec.Body.String())
	if code == nil {
		t.Fatalf("no code came back: %s", rec.Body.String())
	}
	if got := signIn(t, h, anna.ID.String(), code[1]).Code; got != http.StatusOK {
		t.Errorf("a kiosk player still cannot sign in: %d", got)
	}
}

// A new code invalidates the old one immediately, wherever it was issued.
func TestAKioskCodeReplacesTheOldOne(t *testing.T) {
	h, store := kioskHandler(t)
	kiosk := unlock(t, h)

	joined := join(t, h, "Anna")
	old := codeInPage.FindStringSubmatch(joined.Body.String())

	players, _ := store.Players().List(t.Context())
	anna := players[0]

	rec := kioskPost(t, h, "/kiosk/credentials", kiosk, url.Values{"player_id": {anna.ID.String()}})
	fresh := codeInPage.FindStringSubmatch(rec.Body.String())
	if fresh == nil {
		t.Fatalf("no code came back: %s", rec.Body.String())
	}

	if got := signIn(t, h, anna.ID.String(), old[1]).Code; got == http.StatusOK {
		t.Error("the old code still signs in after the kiosk issued a new one")
	}
	if got := signIn(t, h, anna.ID.String(), fresh[1]).Code; got != http.StatusOK {
		t.Errorf("the new code does not sign in: %d", got)
	}
}

// The kiosk may hand out a code; it may not set a PIN. A PIN somebody else
// knows is not a PIN (ADR-0007, open point 3).
func TestTheKioskCannotSetAPIN(t *testing.T) {
	h, store := kioskHandler(t)
	kiosk := unlock(t, h)

	anna, _ := store.Players().Create(t.Context(), "Anna", domain.DefaultTTR)

	// The kiosk cookie is not a session, so the route that sets a PIN turns
	// it away like anybody else.
	rec := kioskPost(t, h, "/credentials/pin", kiosk,
		url.Values{"pin": {"246813"}, "player_id": {anna.ID.String()}})
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("the kiosk set a PIN: %d", rec.Code)
	}
	if _, err := store.Credentials().ForPlayer(t.Context(), anna.ID, domain.CredentialPIN); err == nil {
		t.Error("a PIN was stored for somebody the kiosk was standing in for")
	}

	// And the page does not offer it either.
	r := httptest.NewRequest(http.MethodGet, "/kiosk", nil)
	r.AddCookie(kiosk)
	page := httptest.NewRecorder()
	h.ServeHTTP(page, r)
	if strings.Contains(page.Body.String(), "/credentials/pin") {
		t.Error("the kiosk page offers to set a PIN")
	}
}

// Locked, the whole thing does not exist — the same rule as every other kiosk
// action.
func TestIssuingACodeNeedsTheKioskUnlocked(t *testing.T) {
	h, store := kioskHandler(t)

	anna, _ := store.Players().Create(t.Context(), "Anna", domain.DefaultTTR)

	rec := kioskPost(t, h, "/kiosk/credentials", nil, url.Values{"player_id": {anna.ID.String()}})
	if rec.Code != http.StatusForbidden {
		t.Errorf("issuing a code without the kiosk cookie = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if _, err := store.Credentials().ForPlayer(t.Context(), anna.ID, domain.CredentialRecovery); err == nil {
		t.Error("a code was issued from a locked kiosk")
	}
}

func TestIssuingACodeNeedsAPlayer(t *testing.T) {
	h, _ := kioskHandler(t)
	kiosk := unlock(t, h)

	for _, id := range []string{"", "not-a-uuid", "3f9d3b1e-0000-4000-8000-000000000000"} {
		// 422, the same shape every other refused kiosk form takes: the
		// request was well formed, what was in it was not.
		rec := kioskPost(t, h, "/kiosk/credentials", kiosk, url.Values{"player_id": {id}})
		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("player_id=%q = %d, want %d", id, rec.Code, http.StatusUnprocessableEntity)
		}
		if codeInPage.MatchString(rec.Body.String()) {
			t.Errorf("player_id=%q produced a code anyway", id)
		}
	}
}
