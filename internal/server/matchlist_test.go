package server_test

import (
	"net/http"
	"strings"
	"testing"
)

// TestMatchListShowsEveryMatchFromTheWinnersSide is the point of the page:
// what happened at the table, for everybody, not filtered to one player.
func TestMatchListShowsEveryMatchFromTheWinnersSide(t *testing.T) {
	h, store, anna, bodo := twoBrowsers(t)

	id := reportedByAnna(t, h, store, anna)
	if rec := post(t, h, "/matches/"+id+"/confirm", bodo); rec.Code != http.StatusOK {
		t.Fatalf("confirming: status %d", rec.Code)
	}

	body := fragment(t, h, "/matches", anna).Body.String()

	// Anna reported 11:9 and 12:10, so she won and the row reads from her
	// side — the sets in the order she scored them, not the order they were
	// stored in.
	for _, want := range []string{"Anna", "Bodo", "2:0", "11:9", "12:10"} {
		if !strings.Contains(body, want) {
			t.Errorf("the list does not contain %q: %s", want, body)
		}
	}
	// Winner first: whoever was picked as "home" is an artefact of the form,
	// so the row must not depend on it.
	//
	// Compared inside the table only. Anna's name is also in the top bar,
	// above everything, and a whole-page comparison would pass no matter
	// which way round the row was.
	row := body[strings.Index(body, "<tbody>"):]
	if strings.Index(row, "Anna") > strings.Index(row, "Bodo") {
		t.Errorf("the loser is listed before the winner: %s", row)
	}
	// A confirmed match moved a rating, and the winner's change is the one
	// the row carries.
	if !strings.Contains(body, ">+8<") {
		t.Errorf("the list does not say what the win was worth: %s", body)
	}
	if strings.Contains(body, "offen") || strings.Contains(body, "strittig") {
		t.Errorf("a confirmed match is marked as unsettled: %s", body)
	}
}

// TestMatchListMarksWhatDoesNotCountYet is the reason to show unconfirmed
// matches at all: "where is my match" is exactly the question this page is
// opened with, and leaving them out answers it with silence.
func TestMatchListMarksWhatDoesNotCountYet(t *testing.T) {
	h, store, anna, _ := twoBrowsers(t)

	reportedByAnna(t, h, store, anna)

	body := fragment(t, h, "/matches", anna).Body.String()

	if !strings.Contains(body, "offen") {
		t.Errorf("a match waiting for its opponent is not marked: %s", body)
	}
	// No rating has moved, so no number may be shown — "±0" would claim the
	// match was played and worth nothing.
	if strings.Contains(body, ">+8<") || strings.Contains(body, "±0") {
		t.Errorf("an unconfirmed match carries a rating change: %s", body)
	}
	// The result itself is there: it happened, it just does not count.
	if !strings.Contains(body, "11:9") {
		t.Errorf("the list does not show the sets of an unconfirmed match: %s", body)
	}
}

func TestMatchListSaysWhenThereIsNothing(t *testing.T) {
	h := newHandler(newMemStore())

	body := fragment(t, h, "/matches", nil).Body.String()

	if !strings.Contains(body, "Noch nichts eingetragen") {
		t.Errorf("an empty list does not say so: %s", body)
	}
}

// TestEveryPageLinksToTheMatchList: a page nothing leads to is a page nobody
// finds. The footer is on every page, including the kiosk.
func TestEveryPageLinksToTheMatchList(t *testing.T) {
	h := newHandler(newMemStore())

	for _, path := range []string{"/", "/matches", "/qr"} {
		if body := get(t, h, path).Body.String(); !strings.Contains(body, `href="/matches"`) {
			t.Errorf("%s does not link to the match list", path)
		}
	}
}
