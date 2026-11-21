package server_test

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
)

// counterHidden reads the counter's own fields back out of a drawn counter:
// the options and the event list, which is what the page sends with the next
// tap.
var counterHidden = regexp.MustCompile(`<input type="hidden" name="(counter_[a-z_]+)" value="([^"]*)"`)

// tap sends one action to the counter the way the page does: the options and
// the log as they stand, plus the button that was pressed.
func tap(t *testing.T, h http.Handler, cookie *http.Cookie, form url.Values, action string) string {
	t.Helper()

	f := url.Values{}
	for k, v := range form {
		f[k] = v
	}
	f.Set("counter_action", action)
	rec := kioskPost(t, h, "/kiosk/counter", cookie, f)
	if rec.Code != http.StatusOK {
		t.Fatalf("tap %q = %d, want 200: %s", action, rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	// The page swaps the whole counter, so what it sends next is exactly
	// what this answer holds — the setup questions are gone once it runs.
	if found := counterHidden.FindAllStringSubmatch(body, -1); len(found) > 0 {
		for k := range form {
			if strings.HasPrefix(k, "counter_") {
				delete(form, k)
			}
		}
		for _, m := range found {
			form.Set(m[1], m[2])
		}
	}
	return body
}

// The counter is the kiosk's: the cookie that opens the entry opens this, and
// nothing else does (docs/adr/0034).
func TestOnlyTheKioskCounts(t *testing.T) {
	h, _ := kioskHandler(t)

	rec := kioskPost(t, h, "/kiosk/counter", nil, url.Values{
		"counter_prefix": {"kiosk"}, "counter_action": {"start"},
	})
	if rec.Code != http.StatusForbidden {
		t.Errorf("counting without the kiosk cookie = %d, want 403", rec.Code)
	}
}

// A prefix ends up in an id and a selector; one the server did not draw is
// refused rather than printed into either.
func TestTheCounterRefusesAStrangePrefix(t *testing.T) {
	h, store := kioskHandler(t)
	cookie := unlock(t, h, store)

	rec := kioskPost(t, h, "/kiosk/counter", cookie, url.Values{
		"counter_prefix": {`kiosk"><script>`}, "counter_action": {"start"},
	})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("a strange prefix = %d, want 400", rec.Code)
	}
}

// The kiosk entry offers the counter, folded, between the mode and the boxes.
func TestTheKioskEntryOffersTheCounter(t *testing.T) {
	h, store := kioskHandler(t)
	cookie := unlock(t, h, store)
	seedField(t, store)

	body := kioskBody(t, h, cookie)
	if !strings.Contains(body, "Punkt für Punkt zählen") || !strings.Contains(body, `id="counter-kiosk"`) {
		t.Error("the kiosk entry has no counter")
	}
	// Outside a tournament the first serve is asked, played for by default.
	if !strings.Contains(body, "Ausspielen") {
		t.Error("the kiosk counter does not offer to play for the serve")
	}
}

// A whole match: played for the serve, counted, and the boxes filled at the
// end — out of band, into the form the result is entered with.
func TestTheCounterFillsTheBoxesWhenTheMatchIsOver(t *testing.T) {
	h, store := kioskHandler(t)
	cookie := unlock(t, h, store)
	field := seedField(t, store)

	form := url.Values{
		"counter_prefix": {"kiosk"},
		"home_id":        {field[0].String()},
		"away_id":        {field[1].String()},
		"best_of":        {"1"},
		"points_to_win":  {"11"},
		"counter_first":  {"rally"},
	}
	body := tap(t, h, cookie, form, "start")
	if !strings.Contains(body, "Um die Angabe") {
		t.Fatalf("the serve is not played for: %s", body)
	}
	if !strings.Contains(body, "Anna") || !strings.Contains(body, "Bodo") {
		t.Error("the sides are not headed with the players' names")
	}

	body = tap(t, h, cookie, form, "b")
	if !strings.Contains(body, "Aufschlag Bodo, noch 2") {
		t.Errorf("the rally winner does not serve: %s", body)
	}

	for range 10 {
		tap(t, h, cookie, form, "a")
	}
	tap(t, h, cookie, form, "b")
	body = tap(t, h, cookie, form, "undo")
	if !strings.Contains(body, `<span class="counter-points">10</span>`) {
		t.Errorf("undo did not take the last point back: %s", body)
	}

	body = tap(t, h, cookie, form, "a")
	if !strings.Contains(body, "Anna gewinnt 1:0") {
		t.Fatalf("the match did not end at 11:0: %s", body)
	}
	if !strings.Contains(body, `id="sets-kiosk"`) || !strings.Contains(body, `hx-swap-oob="true"`) {
		t.Error("the set rows did not come back out of band")
	}
	if !strings.Contains(body, `name="set_home_1" value="11"`) {
		t.Errorf("the first set's box was not filled with 11: %s", body)
	}
}

// In a tournament the draw says who serves (docs/adr/0013): side A, whatever
// the request asks for, and no rally.
func TestATournamentCounterServesFromSideA(t *testing.T) {
	h, store := kioskHandler(t)
	cookie := unlock(t, h, store)
	field := seedField(t, store)

	form := url.Values{
		"counter_prefix":     {"t1-" + field[0].String()},
		"counter_tournament": {"1"},
		"counter_first":      {"rally"},
		"home_id":            {field[0].String()},
		"away_id":            {field[1].String()},
		"best_of":            {"3"},
		"points_to_win":      {"11"},
	}
	body := tap(t, h, cookie, form, "start")
	if !strings.Contains(body, "Aufschlag Anna, noch 2") {
		t.Errorf("side A does not serve first in a tournament: %s", body)
	}
}

// The tournament page at the table offers the counter per pairing, without
// the question who serves.
func TestTheKioskTournamentOffersTheCounter(t *testing.T) {
	h, store := kioskHandler(t)
	cookie := unlock(t, h, store)
	field := seedField(t, store)
	id := seedTournament(t, store, "Freitag", domain.TournamentOpen, field)

	rec := fragment(t, h, "/kiosk/tournaments/"+id, cookie)
	body := rec.Body.String()
	if !strings.Contains(body, "Punkt für Punkt zählen") {
		t.Fatal("the kiosk tournament page has no counter")
	}
	if strings.Contains(body, "Ausspielen") {
		t.Error("the tournament counter asks who serves; the draw says it")
	}
	if !strings.Contains(body, "Aufschlag hat Anna") {
		t.Error("the tournament counter does not say who serves")
	}

	// The public page is read on phones and has no counter (docs/adr/0034).
	pub := fragment(t, h, "/tournaments/"+id, nil).Body.String()
	if strings.Contains(pub, "Punkt für Punkt zählen") {
		t.Error("the public tournament page offers the counter")
	}
}
