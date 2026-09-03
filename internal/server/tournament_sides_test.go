package server_test

import (
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
)

// The ends of the table get names, per tournament, because the table does not
// always stand in the same room (docs/adr/0013).
func TestTheScheduleNamesTheSides(t *testing.T) {
	h, store := kioskHandler(t)
	field := seedField(t, store)
	id := seedSides(t, store, field, "Fenster", "Tür")

	body := listBody(t, h, "/tournaments/"+id)

	for _, want := range []string{"Fenster", "Tür"} {
		if !strings.Contains(body, want) {
			t.Errorf("the schedule does not name side %q: %s", want, body)
		}
	}
	// Said once, above the list, rather than behind all twenty-eight
	// pairings.
	if got := strings.Count(body, "hat den ersten"); got != 1 {
		t.Errorf("the serve convention is stated %d times, want 1: %s", got, body)
	}
	if !strings.Contains(body, "im Turnier wird er nicht ausgespielt") {
		t.Errorf("the schedule does not say the serve is given rather than played out: %s", body)
	}
}

// Left alone, the ends are still told apart — they just have no name.
func TestSidesFallBackToLetters(t *testing.T) {
	h, store := kioskHandler(t)
	field := seedField(t, store)
	id := seedSides(t, store, field, domain.DefaultSideA, domain.DefaultSideB)

	body := listBody(t, h, "/tournaments/"+id)

	if !strings.Contains(body, `<span class="side">A</span>`) ||
		!strings.Contains(body, `<span class="side">B</span>`) {
		t.Errorf("the schedule does not fall back to A and B: %s", body)
	}
}

// A blank field means the default, not an empty label: the column refuses an
// empty name, and a feature meant to save time at the table must not cost
// time at the form.
func TestBlankAndOverlongSideNames(t *testing.T) {
	h, store := kioskHandler(t)
	field := seedField(t, store)

	form := url.Values{
		"name":      {"Seiten"},
		"player_id": {field[0].String(), field[1].String()},
		"best_of":   {"3"}, "points_to_win": {"11"},
		"side_a": {"   "},
		"side_b": {strings.Repeat("ü", domain.MaxSideNameLen+7)},
	}
	if rec := postAs(t, h, "/tournaments", nil, form); rec.Code >= 500 {
		t.Fatalf("creating = %d: %s", rec.Code, rec.Body.String())
	}

	created, err := store.tournaments.List(t.Context(), 10)
	if err != nil {
		t.Fatalf("List(): %v", err)
	}
	if len(created) != 1 {
		t.Fatalf("%d tournaments stored, want 1", len(created))
	}
	if created[0].SideA != domain.DefaultSideA {
		t.Errorf("a blank side name became %q, want the default", created[0].SideA)
	}
	// Trimmed rather than refused, and counted in runes: "ü" is two bytes,
	// and a cap that counted those would cut a name in half.
	if got := len([]rune(created[0].SideB)); got != domain.MaxSideNameLen {
		t.Errorf("an over-long side name kept %d runes, want %d", got, domain.MaxSideNameLen)
	}
}

// The sheet has to carry the exception, because the sheet is what hangs on
// the wall.
func TestTheRulesSheetNamesTheTournamentException(t *testing.T) {
	body := rulesSheet(t)

	if !strings.Contains(body, "Im Turnier nicht") {
		t.Errorf("the sheet does not except the tournament from the toss: %s", body)
	}
	// Still part of the serve rule rather than a rule of its own.
	if strings.Count(body, "<dt>") != 7 {
		t.Errorf("the exception became its own rule: %s", body)
	}
}

func seedSides(t *testing.T, store *memStore, field []uuid.UUID, a, b string) string {
	t.Helper()

	created, err := store.tournaments.Create(t.Context(), domain.Tournament{
		Name: "Seiten", Format: domain.TournamentRoundRobin,
		Status: domain.TournamentOpen, CreatedBy: field[0],
		BestOf: 3, PointsToWin: 11, Rated: true,
		SideA: a, SideB: b, Players: field,
	})
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}
	return created.ID.String()
}
