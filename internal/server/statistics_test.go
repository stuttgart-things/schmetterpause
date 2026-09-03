package server_test

import (
	"net/http"
	"strings"
	"testing"
)

// beat has one player report a win over the other and the other confirm it.
func beat(t *testing.T, h http.Handler, store *memStore, winner, loser *http.Cookie, loserName string) {
	t.Helper()

	before := len(store.matches.all())
	if rec := recordMatch(t, h, winner, opponentID(t, store, loserName), 3, 11, "11:9", "11:7"); rec.Code != http.StatusOK {
		t.Fatalf("recording: status %d: %s", rec.Code, rec.Body.String())
	}
	all := store.matches.all()
	if len(all) != before+1 {
		t.Fatalf("%d matches stored, want %d", len(all), before+1)
	}
	if rec := post(t, h, "/matches/"+all[len(all)-1].ID.String()+"/confirm", loser); rec.Code != http.StatusOK {
		t.Fatalf("confirming: status %d: %s", rec.Code, rec.Body.String())
	}
}

// The heading over the matrix. A cell like "3:6" was read as a set score,
// which is what a colon between two numbers means everywhere else here, so
// the page has to say what it is counting before anybody reads one.
func TestTheMatrixSaysWhatItCounts(t *testing.T) {
	h, store, anna, bodo := twoBrowsers(t)
	settledMatch(t, h, store, anna, bodo)

	body := listBody(t, h, "/statistics")

	for _, want := range []string{"Siege:Niederlagen", "nicht Sätze"} {
		if !strings.Contains(body, want) {
			t.Errorf("the matrix does not say %q: %s", want, body)
		}
	}
}

// The matrix is everybody against everybody, and it is not colour-coded.
//
// It was, briefly. In a full matrix every winning record is the mirror of a
// losing one on the other side of the diagonal, so colouring both says the
// same thing twice and turns a reading of the office into a scoreboard with a
// winning half and a losing half. Where a page really is written from one
// person's side — the match list — the colour stays.
func TestTheMatrixIsNotColoured(t *testing.T) {
	h, store, anna, bodo := twoBrowsers(t)
	beat(t, h, store, anna, bodo, "Bodo")
	beat(t, h, store, bodo, anna, "Anna")
	beat(t, h, store, anna, bodo, "Bodo")

	body := listBody(t, h, "/statistics")

	for _, marker := range []string{"record-up", "record-down", "gain", "loss"} {
		if strings.Contains(body, `class="`+marker) {
			t.Errorf("the matrix marks a record with %q: %s", marker, body)
		}
	}
	// Still a table with records in it: 2:1 for Anna, 1:2 for Bodo.
	for _, want := range []string{"2:1", "1:2"} {
		if !strings.Contains(body, want) {
			t.Errorf("the matrix does not show %q: %s", want, body)
		}
	}
}

// Whoever hears the table gets the record in words. "1:0" spoken is a set
// score, which is the same misreading the heading now guards against.
func TestTheMatrixSpellsOutARecord(t *testing.T) {
	h, store, anna, bodo := twoBrowsers(t)
	settledMatch(t, h, store, anna, bodo)

	body := listBody(t, h, "/statistics")

	for _, want := range []string{
		"gegen Bodo: 1 Sieg, 0 Niederlagen",
		"gegen Anna: 0 Siege, 1 Niederlage",
		"insgesamt 1 Sieg, 0 Niederlagen",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the matrix does not say %q: %s", want, body)
		}
	}
	// Said once, not twice: the figure beside it is hidden from the reading.
	if !strings.Contains(body, `<span aria-hidden="true">1:0</span>`) {
		t.Errorf("the visible record is read out as well: %s", body)
	}
}
