package server_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// A result entered at the tournament can be confirmed there.
//
// It always could be confirmed — the buttons just lived on the start page, so
// the opponent had to leave the tournament, find the entry among everything
// else waiting on them, and come back. On a phone next to a table that is the
// difference between confirming now and confirming later, and later is what
// leaves a tournament standing open.
func TestTheTournamentPageOffersTheConfirmation(t *testing.T) {
	h, store, anna, bodo := twoBrowsers(t)
	annaID, bodoID := playerIDs(t, store, "Anna", "Bodo")
	id := seedRated(t, store, []uuid.UUID{annaID, bodoID}, true)

	// Anna reports her own tournament result; Bodo is the one who has to
	// rule on it (docs/adr/0010).
	reportOwn(t, h, id, annaID, bodoID, anna)

	// Bodo, on the tournament page rather than the start page.
	body := fragment(t, h, "/tournaments/"+id, bodo).Body.String()
	if !strings.Contains(body, `id="pending"`) {
		t.Fatalf("the tournament page carries no pending section: %s", body)
	}
	if !strings.Contains(body, "/confirm") {
		t.Errorf("the tournament page does not offer to confirm: %s", body)
	}

	// And Anna, who reported it, is not offered the decision anywhere — that
	// is the whole point of the step.
	if own := fragment(t, h, "/tournaments/"+id, anna).Body.String(); strings.Contains(own, "/confirm") {
		t.Errorf("the reporter is offered their own confirmation: %s", own)
	}
}

// Confirming from the tournament page has to move the table in the same
// response. A table that still says nothing was played reads as a
// confirmation that did not take.
func TestConfirmingOnTheTournamentPageMovesItsTable(t *testing.T) {
	h, store, anna, bodo := twoBrowsers(t)
	annaID, bodoID := playerIDs(t, store, "Anna", "Bodo")
	id := seedRated(t, store, []uuid.UUID{annaID, bodoID}, true)

	reportOwn(t, h, id, annaID, bodoID, anna)

	matches := store.matches.all()
	if len(matches) != 1 {
		t.Fatalf("%d matches stored, want 1", len(matches))
	}

	body := post(t, h, "/matches/"+matches[0].ID.String()+"/confirm", bodo).Body.String()

	if !strings.Contains(body, `id="tournament-table"`) {
		t.Errorf("the confirmation does not carry the tournament table: %s", body)
	}
	if !strings.Contains(body, `hx-swap-oob="true"`) {
		t.Errorf("the table is not marked for an out-of-band swap: %s", body)
	}
	// The table it carries is the one after the result, not before it.
	if !strings.Contains(body, "<td>Anna</td><td>1</td><td>0</td>") {
		t.Errorf("the swapped table does not hold the confirmed result: %s", body)
	}
}

// A casual match carries no table with it: there is none to carry.
func TestConfirmingACasualMatchCarriesNoTable(t *testing.T) {
	h, store, anna, bodo := twoBrowsers(t)
	id := reportedByAnna(t, h, store, anna)

	body := post(t, h, "/matches/"+id+"/confirm", bodo).Body.String()

	if strings.Contains(body, `id="tournament-table"`) {
		t.Errorf("a casual confirmation carries a tournament table: %s", body)
	}
	// It still refreshes what it should.
	if !strings.Contains(body, `id="pending"`) {
		t.Errorf("the confirmation does not refresh the pending list: %s", body)
	}
}

// reportOwn has a player enter their own pairing from the draw, the way
// docs/adr/0010 has it: they report, the opponent rules.
func reportOwn(t *testing.T, h http.Handler, tournamentID string, home, away uuid.UUID, cookie *http.Cookie) {
	t.Helper()

	rec := postAs(t, h, "/matches", cookie, url.Values{
		"tournament_id":    {tournamentID},
		"tournament_round": {"1"},
		"home_id":          {home.String()},
		"away_id":          {away.String()},
		"best_of":          {"3"},
		"set_home_1":       {"11"}, "set_away_1": {"5"},
		"set_home_2": {"11"}, "set_away_2": {"7"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("reporting = %d, want 303: %s", rec.Code, rec.Body.String())
	}
}
