package server_test

import (
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
)

// The order a level table is decided in, written on the page.
//
// At six to twelve players a round robin ties almost every time, and a rule
// nobody can read is a rule everybody has an opinion on — which is the office
// arguing instead of playing. The order itself has been in
// internal/tournament since the table existed; what was missing was saying so.
func TestTheTournamentPageStatesTheTieBreaks(t *testing.T) {
	h, store, _, _ := twoBrowsers(t)
	id := seedTournament(t, store, "Ties", domain.TournamentOpen, field(t, store))

	body := listBody(t, h, "/tournaments/"+id)

	for _, want := range []string{
		"Wenn zwei gleich stehen",
		"Direkter Vergleich",
		"Satzdifferenz",
		"Ballpunktdifferenz",
		// The last step is not something the app can compute, so it says who
		// decides it and how. "Kein Los" is the point of naming it at all.
		"Satz bis 11",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the tournament page does not state %q: %s", want, body)
		}
	}
}

// Folded while it is background, open by itself once it is the question.
func TestTheTieBreaksOpenWhenAPlaceIsShared(t *testing.T) {
	h, store, _, _ := twoBrowsers(t)
	id := seedTournament(t, store, "Ties", domain.TournamentOpen, field(t, store))

	// Nobody has played: two rows of zeroes, nobody ranked, nothing shared.
	quiet := listBody(t, h, "/tournaments/"+id)
	if strings.Contains(quiet, `<details class="tiebreak" open`) {
		t.Errorf("the tie-breaks are open with nothing to break: %s", quiet)
	}
	if !strings.Contains(quiet, `class="tiebreak"`) {
		t.Errorf("the tie-breaks are not on the page at all: %s", quiet)
	}
}

// field is the two players twoBrowsers created, in the shape a tournament
// wants them.
func field(t *testing.T, store *memStore) []uuid.UUID {
	t.Helper()

	anna, bodo := playerIDs(t, store, "Anna", "Bodo")
	return []uuid.UUID{anna, bodo}
}
