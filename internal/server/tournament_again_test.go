package server_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
)

// A second lunch break is a second tournament. What nobody does twice is tick
// the same eight names, so the link at the end of one carries the field and
// the mode into the form for the next.
func TestPlayingAgainCarriesTheFieldAndTheMode(t *testing.T) {
	h, store := kioskHandler(t)
	field := seedField(t, store)
	stranger, err := store.Players().Create(t.Context(), "Dora", domain.DefaultTTR)
	if err != nil {
		t.Fatalf("creating Dora: %v", err)
	}
	id := seedTournamentMode(t, store, 1, 21, field)

	body := listBody(t, h, "/tournaments?nochmal="+id)

	for _, player := range field {
		if !strings.Contains(body, checkedBox(player)) {
			t.Errorf("player %s is not ticked", player)
		}
	}
	if strings.Contains(body, checkedBox(stranger.ID)) {
		t.Error("somebody who was not in the tournament came back ticked")
	}
	if !strings.Contains(body, `<option value="1" selected>`) {
		t.Error("the mode did not come back")
	}
	if !strings.Contains(body, `<option value="21" selected>`) {
		t.Error("the target score did not come back")
	}
	// The name is the one word that is quick to type, and two tournaments
	// called the same thing are two rows nobody can tell apart.
	if strings.Contains(body, `name="name" type="text" maxlength="60" placeholder="Schnelles Turnier" value="Modus 1"`) {
		t.Error("the name was carried over")
	}
}

// A finished tournament offers the link; one still being played does not —
// there is nothing to repeat yet.
func TestTheLinkAppearsWhenThereIsSomethingToRepeat(t *testing.T) {
	h, store := kioskHandler(t)
	field := seedField(t, store)

	open := seedTournamentMode(t, store, 3, 11, field)
	if body := drawBodyAt(t, h, "/tournaments/"+open); strings.Contains(body, "nochmal=") {
		t.Error("a tournament with no results offers to be repeated")
	}

	closed := seedTournamentClosed(t, store, field)
	if body := drawBodyAt(t, h, "/tournaments/"+closed); !strings.Contains(body, "nochmal="+closed) {
		t.Error("a finished tournament does not offer to be repeated")
	}
}

// A link to something that is gone is an empty form, not a refusal: nothing on
// this page has to succeed for it to be useful.
func TestPlayingAgainSurvivesRubbish(t *testing.T) {
	h, store := kioskHandler(t)
	seedField(t, store)

	for _, q := range []string{"", "?nochmal=", "?nochmal=nope", "?nochmal=" + uuid.New().String()} {
		rec := get(t, h, "/tournaments"+q)
		if rec.Code != http.StatusOK {
			t.Errorf("GET /tournaments%s = %d, want 200", q, rec.Code)
		}
	}
}

func checkedBox(id uuid.UUID) string {
	return `value="` + id.String() + `" checked`
}

func listBody(t *testing.T, h http.Handler, path string) string {
	t.Helper()

	rec := get(t, h, path)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s = %d, want 200: %s", path, rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}
