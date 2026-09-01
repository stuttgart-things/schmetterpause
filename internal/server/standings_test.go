package server_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
)

// settledMatch has Anna report a 2:0 over Bodo and Bodo confirm it, so there
// is something for the ranking to count.
func settledMatch(t *testing.T, h http.Handler, store *memStore, anna, bodo *http.Cookie) {
	t.Helper()

	id := reportedByAnna(t, h, store, anna)
	if rec := post(t, h, "/matches/"+id+"/confirm", bodo); rec.Code != http.StatusOK {
		t.Fatalf("confirming: status %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTheRankingCountsOnlyConfirmedMatches(t *testing.T) {
	h, store, anna, bodo := twoBrowsers(t)

	// The same match, before and after the confirmation. Only the confirmed
	// state may reach the ranking.
	id := reportedByAnna(t, h, store, anna)

	before := fragment(t, h, "/fragments/standings", anna).Body.String()
	if !strings.Contains(before, `<td class="num">0</td>`) {
		t.Errorf("a pending match was already counted: %s", before)
	}

	if rec := post(t, h, "/matches/"+id+"/confirm", bodo); rec.Code != http.StatusOK {
		t.Fatalf("confirming: status %d", rec.Code)
	}

	after := fragment(t, h, "/fragments/standings", anna).Body.String()
	for _, want := range []string{"1008", "1:0", "0:1", "992"} {
		if !strings.Contains(after, want) {
			t.Errorf("the ranking does not contain %q: %s", want, after)
		}
	}
}

func TestTheRankingIsOrderedAndNumbered(t *testing.T) {
	h, store, anna, bodo := twoBrowsers(t)
	settledMatch(t, h, store, anna, bodo)

	body := fragment(t, h, "/fragments/standings", anna).Body.String()

	// Anna won, so she is first and Bodo second.
	annaAt := strings.Index(body, "Anna")
	bodoAt := strings.Index(body, "Bodo")
	if annaAt < 0 || bodoAt < 0 || annaAt > bodoAt {
		t.Errorf("the winner is not listed first: %s", body)
	}
	// The place is a chip, so the assertion is on the chip rather than on a
	// digit that could come from any column.
	if !strings.Contains(body, `class="rank rank-1"`) || !strings.Contains(body, `class="rank rank-2"`) {
		t.Errorf("the ranking is not numbered: %s", body)
	}
}

// TestAFreshTableRanksNobody is the state every evening starts in: names on
// the board, no matches. Four players all on the starting rating used to come
// out as four number ones, which reads as a defect rather than as a tie.
func TestAFreshTableRanksNobody(t *testing.T) {
	h, _, anna, _ := twoBrowsers(t)

	body := fragment(t, h, "/fragments/standings", anna).Body.String()

	if strings.Contains(body, `class="rank rank-1"`) {
		t.Errorf("somebody without a confirmed match was given a position: %s", body)
	}
	if !strings.Contains(body, `class="rank rank-none"`) {
		t.Errorf("an unplayed row is not marked as unranked: %s", body)
	}
	// The reason, not just the absence: "nobody has a position" is confusing,
	// "nobody has played" is not.
	if !strings.Contains(body, "Noch kein gewertetes Spiel") {
		t.Errorf("the empty table does not say why it is empty: %s", body)
	}
}

// TestLevelPlayersShareARank is the tie that can actually happen: both have
// played, and they are level.
func TestLevelPlayersShareARank(t *testing.T) {
	h, store, anna, bodo := twoBrowsers(t)
	settledMatch(t, h, store, anna, bodo)

	// The match moved them apart. Put them back on one rating, because the
	// tie is what is under test and not the arithmetic that produced it.
	players, err := store.Players().List(t.Context())
	if err != nil {
		t.Fatalf("List(): %v", err)
	}
	for _, p := range players {
		if err := store.Players().UpdateTTR(t.Context(), p.ID, domain.DefaultTTR); err != nil {
			t.Fatalf("UpdateTTR(): %v", err)
		}
	}

	body := fragment(t, h, "/fragments/standings", anna).Body.String()

	if strings.Contains(body, "rank-2") {
		t.Errorf("two players on the same rating were ranked apart: %s", body)
	}
	if !strings.Contains(body, "geteilter Platz") {
		t.Errorf("a shared rank is not marked as one: %s", body)
	}
}

func TestTheRankingLinksToProfiles(t *testing.T) {
	h, store, anna, _ := twoBrowsers(t)
	id := opponentID(t, store, "Bodo")

	body := fragment(t, h, "/fragments/standings", anna).Body.String()

	if !strings.Contains(body, `href="/players/`+id+`"`) {
		t.Errorf("the ranking does not link to Bodo's profile: %s", body)
	}
}

func TestProfile(t *testing.T) {
	h, store, anna, bodo := twoBrowsers(t)
	settledMatch(t, h, store, anna, bodo)

	rec := fragment(t, h, "/players/"+opponentID(t, store, "Anna"), anna)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()

	for _, want := range []string{
		"Anna",         // the heading
		"1008",         // the current rating
		"+8",           // what the last match did
		"Platz 1",      // where that puts her
		"1:0",          // the record
		"Bodo",         // the opponent in the match table
		"11:9",         // the set scores
		"<polyline",    // the history chart
		"Verlauf 1000", // and its range, since the baseline is not zero
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the profile does not contain %q", want)
		}
	}
}

// TestAProfileWithoutMatches is the state every player starts in, and the one
// where a chart would be a dot pretending to be a trend.
func TestAProfileWithoutMatches(t *testing.T) {
	h, store, anna, _ := twoBrowsers(t)

	rec := fragment(t, h, "/players/"+opponentID(t, store, "Anna"), anna)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Noch keine Matches") {
		t.Errorf("the profile does not say the player has not played: %s", body)
	}
	if strings.Contains(body, "<polyline") {
		t.Error("a player with no history got a chart")
	}
}

// TestAProfileMarksWhatDoesNotCount keeps an unconfirmed match visible but
// clearly outside the rating, rather than hiding it or letting it look
// settled.
func TestAProfileMarksWhatDoesNotCount(t *testing.T) {
	h, store, anna, _ := twoBrowsers(t)
	reportedByAnna(t, h, store, anna)

	body := fragment(t, h, "/players/"+opponentID(t, store, "Anna"), anna).Body.String()

	if !strings.Contains(body, "offen") {
		t.Errorf("a pending match is not marked as pending: %s", body)
	}
	// Listed, but it must not have moved anything: still the starting rating
	// and still no chart.
	if !strings.Contains(body, ">1000<") {
		t.Errorf("a pending match appears to have changed the rating: %s", body)
	}
	if strings.Contains(body, "<polyline") {
		t.Errorf("a pending match produced a rating history: %s", body)
	}
}

func TestProfileOfSomebodyWhoDoesNotExist(t *testing.T) {
	h, _, anna, _ := twoBrowsers(t)

	if rec := fragment(t, h, "/players/not-a-uuid", anna); rec.Code != http.StatusNotFound {
		t.Errorf("a malformed id = %d, want 404", rec.Code)
	}
	if rec := fragment(t, h, "/players/"+uuid.New().String(), anna); rec.Code != http.StatusNotFound {
		t.Errorf("an unknown id = %d, want 404", rec.Code)
	}
}

// The ranking has a page of its own. It used to sit under the entry form,
// which made the start page two things at once: enter what you played, and
// read where everybody stands.
func TestTheRankingHasItsOwnPage(t *testing.T) {
	h, store, anna, bodo := twoBrowsers(t)
	settledMatch(t, h, store, anna, bodo)

	body := fragment(t, h, "/standings", anna).Body.String()
	if !strings.Contains(body, "Rangliste") || !strings.Contains(body, "Bilanz") {
		t.Errorf("the ranking page does not show the ranking: %s", body)
	}
	if !strings.Contains(body, `href="/standings"`) {
		t.Error("the navigation does not name it")
	}

	start := fragment(t, h, "/", anna).Body.String()
	if strings.Contains(start, `id="standings"`) {
		t.Error("the start page still carries the ranking")
	}
}

func TestBothTablesSitInAScrollBox(t *testing.T) {
	// A table cannot shrink below the width its content needs, so without the
	// box a long name takes the whole page sideways with it. The box has to be
	// focusable: a region that scrolls and cannot be reached by keyboard is a
	// region a keyboard cannot read.
	h, store, anna, bodo := twoBrowsers(t)
	settledMatch(t, h, store, anna, bodo)

	for _, page := range []string{"/standings", "/players/" + opponentID(t, store, "Anna")} {
		body := fragment(t, h, page, anna).Body.String()

		for _, want := range []string{`class="table-scroll"`, `tabindex="0"`, `role="region"`} {
			if !strings.Contains(body, want) {
				t.Errorf("%s: the table is not in a scroll box, missing %q", page, want)
			}
		}
	}
}
