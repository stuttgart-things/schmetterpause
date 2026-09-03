package server_test

import (
	"net/http"
	"strings"
	"testing"
)

// The list is winner-first, so a row somebody lost used to show the winner's
// gain in green — on the one list that reader opened to see how they were
// doing. These hold the rating change to the side of whoever the list is
// about.
//
// Anna beats Bodo 2:0, which is worth eight points either way.

func TestTheChangeFollowsTheListRatherThanTheWinner(t *testing.T) {
	h, store, anna, bodo := twoBrowsers(t)
	settledMatch(t, h, store, anna, bodo)

	// Bodo lost, and Bodo's own list has to say so in his own sign.
	own := fragment(t, h, "/matches", bodo).Body.String()
	if !strings.Contains(own, `<span class="loss">-8</span>`) {
		t.Errorf("the loser's list does not show the loss: %s", own)
	}
	if strings.Contains(own, `<span class="gain">+8</span>`) {
		t.Errorf("the loser's list shows the winner's gain: %s", own)
	}

	// And Anna's own list is unchanged by all of this.
	if won := fragment(t, h, "/matches", anna).Body.String(); !strings.Contains(won, `<span class="gain">+8</span>`) {
		t.Errorf("the winner's list does not show the gain: %s", won)
	}
}

// Somebody else's list, which is the same question asked about a third party:
// it is Bodo's list, so it reads in Bodo's sign whoever opened it.
func TestSomebodyElsesListReadsFromTheirSide(t *testing.T) {
	h, store, anna, bodo := twoBrowsers(t)
	settledMatch(t, h, store, anna, bodo)

	body := fragment(t, h, "/matches?spieler="+opponentID(t, store, "Bodo"), anna).Body.String()

	if !strings.Contains(body, `<span class="loss">-8</span>`) {
		t.Errorf("Bodo's list does not read from Bodo's side: %s", body)
	}
}

// A row nobody reading it played keeps the winner's number, which is what
// every row was before there was a side to read it from.
func TestARowWithoutASideKeepsTheWinnersChange(t *testing.T) {
	h, store, anna, bodo := twoBrowsers(t)
	settledMatch(t, h, store, anna, bodo)

	body := listBody(t, h, "/matches?spieler=alle")

	if !strings.Contains(body, `<span class="gain">+8</span>`) {
		t.Errorf("a row belonging to nobody in particular lost its change: %s", body)
	}
}

// The colour is the shortcut, not the information: whoever cannot see it gets
// the name in front of the number.
func TestTheChangeSaysWhoseItIs(t *testing.T) {
	h, store, anna, bodo := twoBrowsers(t)
	settledMatch(t, h, store, anna, bodo)

	own := fragment(t, h, "/matches", bodo).Body.String()
	if !strings.Contains(own, `<span class="sr-only">Bodo: </span>`) {
		t.Errorf("the change does not say whose it is: %s", own)
	}
	// Nobody's list, nobody's name — the row falls back to the winner and
	// naming them there would claim a side the list does not have.
	if all := listBody(t, h, "/matches?spieler=alle"); strings.Contains(all, `class="sr-only">Anna: `) {
		t.Errorf("a row without a side still names one: %s", all)
	}
}

// The stripe down the edge of the row, which is what makes a screen full of
// matches readable at a glance.
func TestWonAndLostRowsAreMarked(t *testing.T) {
	h, store, anna, bodo := twoBrowsers(t)
	settledMatch(t, h, store, anna, bodo)

	if lost := fragment(t, h, "/matches", bodo).Body.String(); !strings.Contains(lost, `class="match-lost"`) {
		t.Errorf("the row Bodo lost is not marked: %s", lost)
	}
	if won := fragment(t, h, "/matches", anna).Body.String(); !strings.Contains(won, `class="match-won"`) {
		t.Errorf("the row Anna won is not marked: %s", won)
	}
	// No side, no mark.
	if all := listBody(t, h, "/matches?spieler=alle"); strings.Contains(all, "match-won") || strings.Contains(all, "match-lost") {
		t.Errorf("a row belonging to nobody in particular is marked: %s", all)
	}
}

// A pending result has a winner on the table and has counted for nothing. The
// badge beside the score says "offen" and the TTR cell says nothing at all —
// a green edge would contradict both.
func TestAnUnsettledRowIsNotMarked(t *testing.T) {
	h, store, anna, bodo := twoBrowsers(t)
	reportedByAnna(t, h, store, anna)

	for who, cookie := range map[string]*http.Cookie{"Anna": anna, "Bodo": bodo} {
		body := fragment(t, h, "/matches", cookie).Body.String()
		if strings.Contains(body, "match-won") || strings.Contains(body, "match-lost") {
			t.Errorf("%s's pending row is marked as settled: %s", who, body)
		}
		if !strings.Contains(body, "offen") {
			t.Errorf("%s's row does not say it is unsettled: %s", who, body)
		}
	}
}
