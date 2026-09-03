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

// One win for Anna: her cell against Bodo and her total are ahead, his are
// behind. Four marks and no more — the counts catch a rule that colours the
// diagonal or an unplayed pairing.
func TestALeadIsMarkedOnBothSides(t *testing.T) {
	h, store, anna, bodo := twoBrowsers(t)
	settledMatch(t, h, store, anna, bodo)

	body := listBody(t, h, "/statistics")

	if got := strings.Count(body, "record-up"); got != 2 {
		t.Errorf("%d cells marked as ahead, want 2 (the cell and the total): %s", got, body)
	}
	if got := strings.Count(body, "record-down"); got != 2 {
		t.Errorf("%d cells marked as behind, want 2 (the cell and the total): %s", got, body)
	}
}

// One each. A level record is a state rather than a result, and marking it in
// either colour would claim something the matches do not say.
func TestALevelRecordIsNotMarked(t *testing.T) {
	h, store, anna, bodo := twoBrowsers(t)
	beat(t, h, store, anna, bodo, "Bodo")
	beat(t, h, store, bodo, anna, "Anna")

	body := listBody(t, h, "/statistics")

	if strings.Contains(body, "record-up") || strings.Contains(body, "record-down") {
		t.Errorf("a level record is marked: %s", body)
	}
	// And it is still a cell with a record in it, not an empty one.
	if !strings.Contains(body, "1:1") {
		t.Errorf("the level record is not shown at all: %s", body)
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
