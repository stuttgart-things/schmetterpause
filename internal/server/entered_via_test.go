package server_test

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stuttgart-things/schmetterpause/internal/auth"
	"github.com/stuttgart-things/schmetterpause/internal/domain"
)

// Issue #71: a tournament evening and a normal Tuesday were indistinguishable
// in the data, so the Definition of Done counted a scorekeeper's typing as
// people logging their own results.
func TestKioskResultsAreMarkedAsSuch(t *testing.T) {
	h, store := kioskHandler(t)
	kiosk := unlock(t, h, store)

	anna, _ := store.Players().Create(t.Context(), "Anna", domain.DefaultTTR)
	bodo, _ := store.Players().Create(t.Context(), "Bodo", domain.DefaultTTR)

	rec := kioskPost(t, h, "/kiosk/matches", kiosk, url.Values{
		"home_id":       {anna.ID.String()},
		"away_id":       {bodo.ID.String()},
		"best_of":       {"3"},
		"points_to_win": {"11"},
		"set_home_1":    {"11"}, "set_away_1": {"9"},
		"set_home_2": {"11"}, "set_away_2": {"7"},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("recording at the kiosk = %d: %s", rec.Code, rec.Body.String())
	}

	stored, err := store.Matches().Recent(t.Context(), 10)
	if err != nil || len(stored) != 1 {
		t.Fatalf("Recent() = %v, %v", stored, err)
	}
	if stored[0].EnteredVia != domain.EnteredViaKiosk {
		t.Errorf("a kiosk result is marked %q, want %q", stored[0].EnteredVia, domain.EnteredViaKiosk)
	}
}

// And the other side of it: a result somebody enters on their own phone is
// the kind the measurement counts.
func TestAPlayersOwnResultIsMarkedAsSuch(t *testing.T) {
	store := newMemStore()
	h := newHandlerWith(store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))

	annaCookie := sessionCookie(t, join(t, h, "Anna"))
	join(t, h, "Bodo")

	players, _ := store.Players().List(t.Context())
	var bodo domain.Player
	for _, p := range players {
		if p.DisplayName == "Bodo" {
			bodo = p
		}
	}

	rec := postForm(t, h, "/matches", url.Values{
		"opponent_id":   {bodo.ID.String()},
		"best_of":       {"3"},
		"points_to_win": {"11"},
		"set_home_1":    {"11"}, "set_away_1": {"9"},
		"set_home_2": {"11"}, "set_away_2": {"7"},
	}, annaCookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("recording from a phone = %d: %s", rec.Code, rec.Body.String())
	}

	stored, err := store.Matches().Recent(t.Context(), 10)
	if err != nil || len(stored) != 1 {
		t.Fatalf("Recent() = %v, %v", stored, err)
	}
	if stored[0].EnteredVia != domain.EnteredViaPlayer {
		t.Errorf("a phone result is marked %q, want %q", stored[0].EnteredVia, domain.EnteredViaPlayer)
	}
}
