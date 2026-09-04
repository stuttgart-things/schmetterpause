package server_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
)

// The kiosk is the only page that can reach a tournament's entry view: the
// grant cookie is scoped to /kiosk, so nothing outside it can know it is the
// machine at the table. Before #124 the kiosk did not mention tournaments at
// all, and the way in was somebody retyping a UUID out of the address bar.
func TestTheKioskLinksToARunningTournament(t *testing.T) {
	h, store := kioskHandler(t)
	cookie := unlock(t, h, store)
	field := seedField(t, store)

	open := seedTournament(t, store, "Mittwochs-Cup", domain.TournamentOpen, field)
	body := kioskBody(t, h, cookie)

	if !strings.Contains(body, "/kiosk/tournaments/"+open) {
		t.Error("the kiosk does not link to the tournament's entry view")
	}
	if !strings.Contains(body, "Mittwochs-Cup") {
		t.Error("the kiosk does not name the tournament")
	}
	// The public copy takes no results, so linking it here would send the one
	// machine that can score to the page that cannot.
	if strings.Contains(body, `"/tournaments/`+open) {
		t.Error("the kiosk links to the read-only page instead of the entry view")
	}
}

// A closed tournament takes no results. Offering it is a door that is already
// locked, and at a table that costs somebody a walk over to find out.
func TestTheKioskLeavesOutClosedTournaments(t *testing.T) {
	h, store := kioskHandler(t)
	cookie := unlock(t, h, store)
	field := seedField(t, store)

	done := seedTournament(t, store, "Letzte Woche", domain.TournamentClosed, field)
	body := kioskBody(t, h, cookie)

	if strings.Contains(body, done) {
		t.Error("the kiosk offers a closed tournament")
	}
	if strings.Contains(body, "Offene Turniere") {
		t.Error("the kiosk shows the heading with nothing running")
	}
}

// With nothing running the kiosk is what it was. An empty section at the top
// would be noise on the page people look at most.
func TestTheKioskStaysPlainWithoutATournament(t *testing.T) {
	h, store := kioskHandler(t)
	cookie := unlock(t, h, store)
	seedField(t, store)

	body := kioskBody(t, h, cookie)

	if strings.Contains(body, "Offene Turniere") {
		t.Error("the kiosk announces tournaments when there are none")
	}
	if !strings.Contains(body, "Ergebnis eintragen") {
		t.Error("the kiosk lost its result entry")
	}
}

// seedField puts two players in the store and returns their ids.
func seedField(t *testing.T, store *memStore) []uuid.UUID {
	t.Helper()

	var ids []uuid.UUID
	for _, name := range []string{"Anna", "Bodo"} {
		p, err := store.Players().Create(t.Context(), name, domain.DefaultTTR)
		if err != nil {
			t.Fatalf("seeding %s: %v", name, err)
		}
		ids = append(ids, p.ID)
	}
	return ids
}

// seedTournament stores one in the given state and returns its id as a string,
// which is the form the page carries it in.
func seedTournament(t *testing.T, store *memStore, name string,
	status domain.TournamentStatus, field []uuid.UUID,
) string {
	t.Helper()

	tour := domain.Tournament{
		Name:      name,
		Format:    domain.TournamentRoundRobin,
		Status:    status,
		CreatedBy: field[0],
		CreatedAt: time.Now(),
		Players:   field,
	}
	if status == domain.TournamentClosed {
		closed := time.Now()
		tour.ClosedAt = &closed
	}

	created, err := store.tournaments.Create(t.Context(), tour)
	if err != nil {
		t.Fatalf("seeding the tournament: %v", err)
	}
	return created.ID.String()
}

// kioskBody fetches /kiosk with the grant and returns the page.
func kioskBody(t *testing.T, h http.Handler, cookie *http.Cookie) string {
	t.Helper()

	rec := fragment(t, h, "/kiosk", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /kiosk = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}
