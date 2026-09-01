package server_test

import (
	"net/http"
	"strings"
	"testing"
)

// Not recognised: the whole office, as before. The list belongs to everybody
// and a stranger has no own row to start from.
func TestTheMatchListShowsEverybodyToAStranger(t *testing.T) {
	h, store, anna, bodo := twoBrowsers(t)
	settledMatch(t, h, store, anna, bodo)

	body := listBody(t, h, "/matches")
	if !strings.Contains(body, "Alles, was an der Platte passiert ist") {
		t.Error("a stranger does not get the whole list")
	}
	if !strings.Contains(body, `value="alle" selected`) {
		t.Error("the picker does not say it is showing everybody")
	}
}

// Recognised and not asked: their own. A reader with an account is almost
// always looking for their own row first.
func TestTheMatchListStartsWithYourOwn(t *testing.T) {
	h, store, anna, bodo := twoBrowsers(t)
	settledMatch(t, h, store, anna, bodo)

	body := fragment(t, h, "/matches", anna).Body.String()
	if !strings.Contains(body, "Alles, was Anna gespielt hat") {
		t.Errorf("a recognised reader does not get their own matches: %s", body)
	}
	if strings.Contains(body, `value="alle" selected`) {
		t.Error("the picker claims to be showing everybody")
	}
}

// "alle" is an answer, not the absence of one — which is what lets the default
// be different for somebody who has an account.
func TestAskingForEverybodyOverridesTheDefault(t *testing.T) {
	h, store, anna, bodo := twoBrowsers(t)
	settledMatch(t, h, store, anna, bodo)

	body := fragment(t, h, "/matches?spieler=alle", anna).Body.String()
	if !strings.Contains(body, "Alles, was an der Platte passiert ist") {
		t.Error("asking for everybody did not override the default")
	}
}

// Somebody else's, which is the whole point of a picker rather than a toggle.
func TestTheMatchListCanShowSomebodyElse(t *testing.T) {
	h, store, anna, bodo := twoBrowsers(t)
	settledMatch(t, h, store, anna, bodo)

	id := opponentID(t, store, "Bodo")
	body := fragment(t, h, "/matches?spieler="+id, anna).Body.String()
	if !strings.Contains(body, "Alles, was Bodo gespielt hat") {
		t.Errorf("the list does not follow the picker: %s", body)
	}
	if !strings.Contains(body, `value="`+id+`" selected`) {
		t.Error("the picker does not hold what was chosen")
	}
}

// A name nobody has is not worth a refusal on a page that reads perfectly
// well without one.
func TestRubbishInThePickerFallsBackRatherThanFailing(t *testing.T) {
	h, store, anna, bodo := twoBrowsers(t)
	settledMatch(t, h, store, anna, bodo)

	for _, q := range []string{"?spieler=", "?spieler=niemand", "?spieler=00000000-0000-0000-0000-000000000000"} {
		rec := fragment(t, h, "/matches"+q, anna)
		if rec.Code != http.StatusOK {
			t.Errorf("GET /matches%s = %d, want 200", q, rec.Code)
		}
	}
}
