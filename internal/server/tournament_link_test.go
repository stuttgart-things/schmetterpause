package server_test

import (
	"net/http"
	"strings"
	"testing"
)

// Two people in a row read "unter derselben Adresse mit /kiosk davor" and
// asked how to get from there to entering a result. A sentence that describes
// an address is a dead end with a caption; this is the link.
func TestTheDrawLinksToWhereResultsAreEntered(t *testing.T) {
	h, store := kioskHandler(t)
	field := seedField(t, store)
	id := seedTournamentMode(t, store, 3, 11, field)

	body := drawBodyAt(t, h, "/tournaments/"+id)

	if !strings.Contains(body, `href="/kiosk/tournaments/`+id+`"`) {
		t.Error("the public draw does not link to the entry view")
	}
	// The link is not a promise: it works for one device, and the sentence
	// beside it has to say so.
	if !strings.Contains(body, "freigeschalteten Gerät") {
		t.Error("the link does not say who it works for")
	}
}

// The kiosk copy renders for anybody, just without the boxes — so an
// un-unlocked device landing there used to read that it should go to the
// address it was already on.
func TestTheKioskCopyDoesNotSendYouWhereYouAre(t *testing.T) {
	h, store := kioskHandler(t)
	field := seedField(t, store)
	id := seedTournamentMode(t, store, 3, 11, field)

	body := drawBodyAt(t, h, "/kiosk/tournaments/"+id)

	if strings.Contains(body, `href="/kiosk/tournaments/`+id+`"`) {
		t.Error("the page links to itself")
	}
	if !strings.Contains(body, "nicht freigeschaltet") {
		t.Error("the page does not say why there is nothing to type into")
	}
}

// With the grant the boxes are there and neither sentence appears: nothing to
// explain to somebody who can already enter.
func TestTheUnlockedDrawJustOffersTheBoxes(t *testing.T) {
	h, store := kioskHandler(t)
	cookie := unlock(t, h, store)
	field := seedField(t, store)
	id := seedTournamentMode(t, store, 3, 11, field)

	body := drawBody(t, h, cookie, id)

	if !strings.Contains(body, `name="set_home_1"`) {
		t.Fatal("the unlocked draw has no entry boxes")
	}
	for _, unwanted := range []string{"nicht freigeschaltet", "Ergebnisse eintragen"} {
		if strings.Contains(body, unwanted) {
			t.Errorf("the unlocked draw still explains %q", unwanted)
		}
	}
}

// drawBodyAt fetches a draw at an exact path, with no kiosk grant.
func drawBodyAt(t *testing.T, h http.Handler, path string) string {
	t.Helper()

	rec := get(t, h, path)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s = %d, want 200: %s", path, rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}
