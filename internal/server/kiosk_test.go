package server_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/auth"
	"github.com/stuttgart-things/schmetterpause/internal/domain"
)

const testKioskToken = "tuesday"

// kioskHandler wires a server with the kiosk unlocked by testKioskToken.
func kioskHandler(t *testing.T) (http.Handler, *memStore) {
	t.Helper()

	store := newMemStore()
	cfg := testConfig()
	cfg.KioskToken = testKioskToken
	cfg.SessionKey = testSessionKey
	h := newHandlerConfig(cfg, store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))
	return h, store
}

// unlock walks the token exchange, names an operator and returns the kiosk
// cookie — a kiosk that can actually be used.
//
// Naming is part of unlocking here because it is part of it in the
// application: since issue #90 a machine that has not said who is typing may
// not write anything, so a helper that stopped at the token would hand every
// test a kiosk that refuses everything. The tests about the unnamed state use
// unlockOnly.
func unlock(t *testing.T, h http.Handler, store *memStore) *http.Cookie {
	t.Helper()

	cookie := unlockOnly(t, h)
	nameOperator(t, h, store, cookie)
	return cookie
}

// unlockOnly walks the token exchange and stops there: unlocked, unnamed.
func unlockOnly(t *testing.T, h http.Handler) *http.Cookie {
	t.Helper()

	rec := get(t, h, "/kiosk?token="+testKioskToken)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("unlocking = %d, want 303: %s", rec.Code, rec.Body.String())
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == "schmetterpause_kiosk" {
			return c
		}
	}
	t.Fatal("unlocking set no kiosk cookie")
	return nil
}

// scorekeeperName is who tests put behind the kiosk. Somebody who plays no
// matches, because the operator may not be one of the two players and a
// helper that picked a competitor would fail half the tests for the right
// reason at the wrong moment.
const scorekeeperName = "Schiri"

// nameOperator answers the "wer trägt ein?" question for a machine, creating
// the scorekeeper the first time it is asked.
func nameOperator(t *testing.T, h http.Handler, store *memStore, cookie *http.Cookie) uuid.UUID {
	t.Helper()

	id := scorekeeper(t, store)
	rec := kioskPost(t, h, "/kiosk/operator", cookie, url.Values{
		"operator_id": {id.String()},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("naming the operator = %d, want 303: %s", rec.Code, rec.Body.String())
	}
	return id
}

// scorekeeper is the operator player, created once per store.
func scorekeeper(t *testing.T, store *memStore) uuid.UUID {
	t.Helper()

	players, err := store.Players().List(t.Context())
	if err != nil {
		t.Fatalf("listing the players: %v", err)
	}
	for _, p := range players {
		if p.DisplayName == scorekeeperName {
			return p.ID
		}
	}

	p, err := store.Players().Create(t.Context(), scorekeeperName, domain.DefaultTTR)
	if err != nil {
		t.Fatalf("creating the scorekeeper: %v", err)
	}
	return p.ID
}

func kioskPost(t *testing.T, h http.Handler, path string, cookie *http.Cookie, form url.Values) *httptest.ResponseRecorder {
	t.Helper()

	r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if cookie != nil {
		r.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

// TestTheKioskDoesNotExistWithoutAToken: an unlocked kiosk on a laptop in an
// office is worse than none, so an unset token removes the routes rather than
// leaving them open.
func TestTheKioskDoesNotExistWithoutAToken(t *testing.T) {
	h := newHandler(newMemStore()) // testConfig leaves KioskToken empty

	for _, path := range []string{"/kiosk", "/kiosk?token=tuesday"} {
		if rec := get(t, h, path); rec.Code != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", path, rec.Code)
		}
	}
	if rec := kioskPost(t, h, "/kiosk/matches", nil, url.Values{}); rec.Code != http.StatusNotFound {
		t.Errorf("POST /kiosk/matches = %d, want 404", rec.Code)
	}
}

func TestTheKioskWantsTheToken(t *testing.T) {
	h, store := kioskHandler(t)

	// Without a grant the door is a form, not a refusal: the address alone
	// tells nobody anything, and whoever sets up the table was told the code
	// rather than guessing the path. What it must not do is say what is being
	// played behind it.
	rec := get(t, h, "/kiosk")
	if rec.Code != http.StatusOK {
		t.Errorf("without a cookie = %d, want 200 and the code form", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, `name="code"`) {
		t.Error("the door offers no way in")
	}
	if rec := get(t, h, "/kiosk?token=wednesday"); rec.Code != http.StatusUnauthorized {
		t.Errorf("with the wrong token = %d, want 401", rec.Code)
	}
	if rec := kioskPost(t, h, "/kiosk/players", nil, url.Values{"display_name": {"Anna"}}); rec.Code != http.StatusForbidden {
		t.Errorf("posting without a cookie = %d, want 403", rec.Code)
	}

	kiosk := unlock(t, h, store)

	r := httptest.NewRequest(http.MethodGet, "/kiosk", nil)
	r.AddCookie(kiosk)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	if rec.Code != http.StatusOK {
		t.Fatalf("with the cookie = %d, want 200", rec.Code)
	}
	// The token is swapped for a cookie and does not come back in the page.
	if strings.Contains(rec.Body.String(), testKioskToken) {
		t.Error("the page carries the token where the next borrower can read it")
	}
}

// TestTheKioskLeavesItsOwnSessionAlone is the reason a kiosk needs its own
// route at all: joining normally sets the cookie, so eight players entered
// from one laptop would leave the laptop signed in as the eighth.
func TestTheKioskLeavesItsOwnSessionAlone(t *testing.T) {
	h, store := kioskHandler(t)
	kiosk := unlock(t, h, store)

	for _, name := range []string{"Anna", "Bodo", "Cara"} {
		rec := kioskPost(t, h, "/kiosk/players", kiosk, url.Values{"display_name": {name}})
		if rec.Code != http.StatusOK {
			t.Fatalf("adding %s = %d: %s", name, rec.Code, rec.Body.String())
		}
		for _, c := range rec.Result().Cookies() {
			if c.Name == auth.SessionCookieName {
				t.Fatalf("adding %s signed the kiosk in as them", name)
			}
		}
	}

	n, err := store.Players().Count(t.Context())
	if err != nil {
		t.Fatalf("Count(): %v", err)
	}
	// Three added here, plus the scorekeeper the kiosk names as operator.
	if n != 4 {
		t.Errorf("%d players exist, want 4", n)
	}
}

func TestTheKioskRefusesATakenName(t *testing.T) {
	h, store := kioskHandler(t)
	kiosk := unlock(t, h, store)

	kioskPost(t, h, "/kiosk/players", kiosk, url.Values{"display_name": {"Anna"}})
	rec := kioskPost(t, h, "/kiosk/players", kiosk, url.Values{"display_name": {"anna"}})

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "gibt es schon") {
		t.Errorf("the refusal does not say why: %s", rec.Body.String())
	}
}

// kioskPostAs is kioskPost with a player session alongside the kiosk cookie —
// one browser that is both, which is the situation issue #90 is about.
func kioskPostAs(t *testing.T, h http.Handler, path string,
	kiosk, session *http.Cookie, form url.Values) *httptest.ResponseRecorder {
	t.Helper()

	r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.AddCookie(kiosk)
	r.AddCookie(session)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

// TestTheKioskRefusesYourOwnMatch is issue #90: a kiosk result settles at
// once, so entering your own game there removes the one check the application
// has — the opponent agreeing with it.
//
// The guard can only see a player signed in on this very browser, which is
// exactly what this sets up. Somebody using a private window is invisible to
// it; that limit is recorded in the handler rather than pretended away here.
func TestTheKioskRefusesYourOwnMatch(t *testing.T) {
	h, store := kioskHandler(t)
	kiosk := unlock(t, h, store)

	// Anna joins normally, so this browser holds a session of her own.
	session := sessionCookie(t, join(t, h, "Anna"))
	kioskPost(t, h, "/kiosk/players", kiosk, url.Values{"display_name": {"Bodo"}})
	ids := kioskPlayers(t, store, "Anna", "Bodo")

	rec := kioskPostAs(t, h, "/kiosk/matches", kiosk, session,
		kioskResult(ids[0], ids[1], 3, 11, "11:9", "12:10"))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Startseite") {
		t.Errorf("the refusal does not point at the way that does ask the opponent: %s",
			rec.Body.String())
	}

	// Nothing was written. A refusal that still counted would be worse than
	// none, because it would look handled.
	matches, err := store.Matches().Recent(t.Context(), 10)
	if err != nil {
		t.Fatalf("Recent(): %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("%d matches were recorded, want 0", len(matches))
	}
}

// TestTheKioskRefusesTakingBackYourOwnMatch is the same rule from the other
// side: taking back a result you played in is entering one, in reverse.
func TestTheKioskRefusesTakingBackYourOwnMatch(t *testing.T) {
	h, store := kioskHandler(t)
	kiosk := unlock(t, h, store)

	session := sessionCookie(t, join(t, h, "Anna"))
	kioskPost(t, h, "/kiosk/players", kiosk, url.Values{"display_name": {"Bodo"}})
	ids := kioskPlayers(t, store, "Anna", "Bodo")

	// Entered without the session cookie — the ordinary tournament case,
	// which has to keep working, and does: this is where the id comes from.
	id := undoLink(t, kioskEnter(t, h, kiosk, ids[1], ids[0]))

	rec := kioskPostAs(t, h, "/kiosk/matches/"+id+"/undo", kiosk, session, url.Values{})

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "selbst mitspielst") {
		t.Errorf("the refusal does not say why: %s", rec.Body.String())
	}

	matches, err := store.Matches().Recent(t.Context(), 10)
	if err != nil {
		t.Fatalf("Recent(): %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("%d matches remain, want the entered one to still be there", len(matches))
	}
	if matches[0].Status != domain.MatchConfirmed {
		t.Errorf("the match is %q, want it still confirmed", matches[0].Status)
	}
}

// TestTheKioskStillWorksForOtherPeoplesMatches guards the guard: the whole
// point of the kiosk is entering games you are not playing in, and a check
// that also stopped that would be a regression rather than a fix.
func TestTheKioskStillWorksForOtherPeoplesMatches(t *testing.T) {
	h, store := kioskHandler(t)
	kiosk := unlock(t, h, store)

	// Signed in as Anna, entering a match between two other people.
	session := sessionCookie(t, join(t, h, "Anna"))
	kioskPost(t, h, "/kiosk/players", kiosk, url.Values{"display_name": {"Bodo"}})
	kioskPost(t, h, "/kiosk/players", kiosk, url.Values{"display_name": {"Cara"}})
	ids := kioskPlayers(t, store, "Bodo", "Cara")

	rec := kioskPostAs(t, h, "/kiosk/matches", kiosk, session,
		kioskResult(ids[0], ids[1], 3, 11, "11:9", "12:10"))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	matches, err := store.Matches().Recent(t.Context(), 10)
	if err != nil {
		t.Fatalf("Recent(): %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("%d matches were recorded, want 1", len(matches))
	}
	if matches[0].Status != domain.MatchConfirmed {
		t.Errorf("the match is %q, want it settled at once", matches[0].Status)
	}
}

// kioskPlayers returns the ids of the seeded players, by name.
func kioskPlayers(t *testing.T, store *memStore, names ...string) []string {
	t.Helper()

	players, err := store.Players().List(t.Context())
	if err != nil {
		t.Fatalf("List(): %v", err)
	}

	ids := make([]string, 0, len(names))
	for _, want := range names {
		found := ""
		for _, p := range players {
			if p.DisplayName == want {
				found = p.ID.String()
			}
		}
		if found == "" {
			t.Fatalf("no player named %q", want)
		}
		ids = append(ids, found)
	}
	return ids
}

func kioskResult(home, away string, bestOf, points int, sets ...string) url.Values {
	form := url.Values{
		"home_id":       {home},
		"away_id":       {away},
		"best_of":       {strconv.Itoa(bestOf)},
		"points_to_win": {strconv.Itoa(points)},
	}
	for i, s := range sets {
		h, a, _ := strings.Cut(s, ":")
		form.Set("set_home_"+strconv.Itoa(i+1), h)
		form.Set("set_away_"+strconv.Itoa(i+1), a)
	}
	return form
}

// TestTheKioskSettlesAtOnce is the Definition of Done: somebody watched the
// match and wrote it down, so there is nobody left to ask and the rating
// moves immediately.
func TestTheKioskSettlesAtOnce(t *testing.T) {
	h, store := kioskHandler(t)
	kiosk := unlock(t, h, store)

	kioskPost(t, h, "/kiosk/players", kiosk, url.Values{"display_name": {"Anna"}})
	kioskPost(t, h, "/kiosk/players", kiosk, url.Values{"display_name": {"Bodo"}})
	ids := kioskPlayers(t, store, "Anna", "Bodo")

	rec := kioskPost(t, h, "/kiosk/matches", kiosk, kioskResult(ids[0], ids[1], 3, 11, "11:9", "12:10"))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	// Told the way somebody at the table would say it.
	if !strings.Contains(rec.Body.String(), "Anna schlägt Bodo 2:0") {
		t.Errorf("the page does not say what happened: %s", rec.Body.String())
	}

	stored := store.matches.all()
	if len(stored) != 1 {
		t.Fatalf("%d matches stored, want 1", len(stored))
	}
	if stored[0].Status != domain.MatchConfirmed {
		t.Errorf("status = %q, want confirmed — the kiosk asks nobody", stored[0].Status)
	}
	if got := ttrOfPlayer(t, store, "Anna"); got != 1008 {
		t.Errorf("Anna is on %d, want 1008 — the result did not count", got)
	}
	if got := ttrOfPlayer(t, store, "Bodo"); got != 992 {
		t.Errorf("Bodo is on %d, want 992", got)
	}
}

func TestTheKioskRefusesImpossibleInput(t *testing.T) {
	h, store := kioskHandler(t)
	kiosk := unlock(t, h, store)

	kioskPost(t, h, "/kiosk/players", kiosk, url.Values{"display_name": {"Anna"}})
	kioskPost(t, h, "/kiosk/players", kiosk, url.Values{"display_name": {"Bodo"}})
	ids := kioskPlayers(t, store, "Anna", "Bodo")

	tests := []struct {
		name string
		form url.Values
		says string
	}{
		{"the same player twice", kioskResult(ids[0], ids[0], 3, 11, "11:9", "11:7"), "verschiedene Spieler"},
		{"nobody chosen", kioskResult("", "", 3, 11, "11:9", "11:7"), "beide Spieler"},
		{"one clear point", kioskResult(ids[0], ids[1], 3, 11, "11:10", "11:9"), "zwei Punkte Vorsprung"},
		{"undecided", kioskResult(ids[0], ids[1], 5, 11, "11:9"), "noch nicht entschieden"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := kioskPost(t, h, "/kiosk/matches", kiosk, tc.form)

			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want 422", rec.Code)
			}
			if !strings.Contains(rec.Body.String(), tc.says) {
				t.Errorf("the refusal does not say why: %s", rec.Body.String())
			}
			if n := len(store.matches.all()); n != 0 {
				t.Errorf("%d matches were stored anyway", n)
			}
			if got := ttrOfPlayer(t, store, "Anna"); got != domain.DefaultTTR {
				t.Errorf("Anna moved to %d on a refused entry", got)
			}
		})
	}
}

func TestTheKioskLeadsWithTheThingThatHappensAllEvening(t *testing.T) {
	// Adding a player happens once per person, entering a result happens
	// after every match. The order on the page follows the frequency.
	h, store := kioskHandler(t)
	cookie := unlock(t, h, store)

	body := fragment(t, h, "/kiosk", cookie).Body.String()

	enter := strings.Index(body, "Ergebnis eintragen")
	create := strings.Index(body, "Spieler anlegen")

	if enter < 0 || create < 0 {
		t.Fatalf("the kiosk is missing one of its two forms: %s", body)
	}
	if enter > create {
		t.Error("adding a player comes before entering a result")
	}
}

func TestTheKioskCannotPitchAPlayerAgainstThemselves(t *testing.T) {
	// The server refuses it, but only after the whole match has been typed
	// in. The name comes out of the other list instead, so the mistake is
	// unavailable rather than punished.
	h, store := kioskHandler(t)
	cookie := unlock(t, h, store)

	for _, name := range []string{"Anna", "Bodo"} {
		if _, err := store.Players().Create(t.Context(), name, domain.DefaultTTR); err != nil {
			t.Fatalf("seeding %s: %v", name, err)
		}
	}
	anna := opponentID(t, store, "Anna")

	r := httptest.NewRequest(http.MethodGet,
		"/fragments/sets?sets_prefix=kiosk&home_id="+anna+"&best_of=3", nil)
	r.Header.Set("HX-Trigger", "kiosk-home")
	r.AddCookie(cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)

	body := rec.Body.String()

	if !strings.Contains(body, `id="kiosk-away"`) {
		t.Fatalf("the opposite picker did not come back: %s", body)
	}
	if !strings.Contains(body, `hx-swap-oob="true"`) {
		t.Errorf("the picker is not marked for an out-of-band swap: %s", body)
	}
	if strings.Contains(body, `value="`+anna+`"`) {
		t.Errorf("the opponent list still offers the player who is already home: %s", body)
	}
	if !strings.Contains(body, "Bodo") {
		t.Errorf("the opponent list lost everybody else too: %s", body)
	}
}

func TestAModeChangeLeavesThePickersAlone(t *testing.T) {
	// The two selects already agree with each other then, and replacing a
	// select somebody is not touching is a change they did not ask for.
	h, store := kioskHandler(t)
	cookie := unlock(t, h, store)
	if _, err := store.Players().Create(t.Context(), "Anna", domain.DefaultTTR); err != nil {
		t.Fatalf("seeding: %v", err)
	}

	r := httptest.NewRequest(http.MethodGet, "/fragments/sets?sets_prefix=kiosk&best_of=3", nil)
	r.Header.Set("HX-Trigger", "kiosk-best-of")
	r.AddCookie(cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)

	if strings.Contains(rec.Body.String(), "hx-swap-oob") {
		t.Errorf("a mode change replaced a picker: %s", rec.Body.String())
	}
}

func TestBothPagesGreetWithTheMascot(t *testing.T) {
	// One drawing, at the top of both pages, and hidden in print.
	h, store := kioskHandler(t)
	cookie := unlock(t, h, store)

	kiosk := fragment(t, h, "/kiosk", cookie).Body.String()
	// Beside the heading rather than above it: in the same row, and before
	// the first card either way. Document order puts it after the heading
	// now, which is what makes it sit on the right.
	if !strings.Contains(kiosk, `class="page-head"`) {
		t.Errorf("the kiosk has no heading row: %s", kiosk)
	}
	if !strings.Contains(kiosk, `class="page-mascot`) {
		t.Errorf("the kiosk has no mascot: %s", kiosk)
	}
	if strings.Index(kiosk, "page-mascot") > strings.Index(kiosk, `<section class="match">`) {
		t.Error("the kiosk mascot sits below the first card")
	}

	if _, err := store.Players().Create(t.Context(), "Anna", domain.DefaultTTR); err != nil {
		t.Fatalf("seeding: %v", err)
	}
	start := get(t, h, "/").Body.String()
	if !strings.Contains(start, `class="page-mascot`) {
		t.Errorf("the start page has no mascot: %s", start)
	}
	if !strings.Contains(start, `class="page-head"`) {
		t.Errorf("the start page has no heading row: %s", start)
	}
	if strings.Index(start, "page-mascot") > strings.Index(start, `id="match"`) {
		t.Error("the start page mascot sits below the entry form")
	}
}

// kioskEnter records a result at the kiosk and returns the page it answered
// with, which is where the undo is offered.
func kioskEnter(t *testing.T, h http.Handler, cookie *http.Cookie, home, away string) string {
	t.Helper()

	rec := kioskPost(t, h, "/kiosk/matches", cookie, url.Values{
		"home_id": {home}, "away_id": {away},
		"best_of": {"3"}, "points_to_win": {"11"},
		"set_home_1": {"11"}, "set_away_1": {"7"},
		"set_home_2": {"11"}, "set_away_2": {"9"},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("entering: status %d: %s", rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

// undoLink pulls the match id out of the form the kiosk offers after an entry.
func undoLink(t *testing.T, body string) string {
	t.Helper()

	const marker = `action="/kiosk/matches/`
	i := strings.Index(body, marker)
	if i < 0 {
		t.Fatalf("no undo offered: %s", body)
	}
	rest := body[i+len(marker):]
	return rest[:strings.Index(rest, "/undo")]
}

func TestTheKioskCanTakeTheLastResultBack(t *testing.T) {
	// A kiosk result counts at once, so there is nothing to dispute and
	// nothing to correct. Without this a typo stands for good.
	h, store := kioskHandler(t)
	cookie := unlock(t, h, store)
	for _, name := range []string{"Anna", "Bodo"} {
		if _, err := store.Players().Create(t.Context(), name, domain.DefaultTTR); err != nil {
			t.Fatalf("seeding %s: %v", name, err)
		}
	}
	anna, bodo := opponentID(t, store, "Anna"), opponentID(t, store, "Bodo")

	body := kioskEnter(t, h, cookie, anna, bodo)
	if !strings.Contains(body, "Zurücknehmen") {
		t.Fatalf("no way back was offered: %s", body)
	}

	// The rating moved — for the two who played. The scorekeeper who typed it
	// in is a player too and stays exactly where they were, which is the
	// point of naming an operator rather than crediting the home player.
	moved, err := store.Players().List(t.Context())
	if err != nil {
		t.Fatalf("List(): %v", err)
	}
	for _, p := range moved {
		played := p.ID.String() == anna || p.ID.String() == bodo
		switch {
		case played && p.TTR == domain.DefaultTTR:
			t.Fatalf("%s did not move at all: %d", p.DisplayName, p.TTR)
		case !played && p.TTR != domain.DefaultTTR:
			t.Fatalf("%s moved without playing: %d", p.DisplayName, p.TTR)
		}
	}

	rec := kioskPost(t, h, "/kiosk/matches/"+undoLink(t, body)+"/undo", cookie, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("undo: status %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Zurückgenommen") {
		t.Errorf("the undo did not say what happened: %s", rec.Body.String())
	}

	// Both ratings are back, and the match is gone from the ranking.
	back, err := store.Players().List(t.Context())
	if err != nil {
		t.Fatalf("List(): %v", err)
	}
	for _, p := range back {
		if p.TTR != domain.DefaultTTR {
			t.Errorf("%s came back to %d, want %d", p.DisplayName, p.TTR, domain.DefaultTTR)
		}
	}
	if !strings.Contains(rec.Body.String(), `class="rank rank-none"`) {
		t.Errorf("the ranking still counts the taken-back match: %s", rec.Body.String())
	}
}

func TestASecondResultBlocksTheUndo(t *testing.T) {
	// Putting the ratings back means writing the ones from before the match
	// straight back, and that is only right while nothing has counted since.
	h, store := kioskHandler(t)
	cookie := unlock(t, h, store)
	for _, name := range []string{"Anna", "Bodo"} {
		if _, err := store.Players().Create(t.Context(), name, domain.DefaultTTR); err != nil {
			t.Fatalf("seeding %s: %v", name, err)
		}
	}
	anna, bodo := opponentID(t, store, "Anna"), opponentID(t, store, "Bodo")

	first := undoLink(t, kioskEnter(t, h, cookie, anna, bodo))
	kioskEnter(t, h, cookie, bodo, anna)

	rec := kioskPost(t, h, "/kiosk/matches/"+first+"/undo", cookie, nil)

	if !strings.Contains(rec.Body.String(), "weiteres gewertet") {
		t.Errorf("the older result was taken back anyway: %s", rec.Body.String())
	}
}

func TestOnlyTheKioskMayTakeAResultBack(t *testing.T) {
	h, store := kioskHandler(t)
	cookie := unlock(t, h, store)
	for _, name := range []string{"Anna", "Bodo"} {
		if _, err := store.Players().Create(t.Context(), name, domain.DefaultTTR); err != nil {
			t.Fatalf("seeding %s: %v", name, err)
		}
	}
	id := undoLink(t, kioskEnter(t, h, cookie,
		opponentID(t, store, "Anna"), opponentID(t, store, "Bodo")))

	rec := kioskPost(t, h, "/kiosk/matches/"+id+"/undo", nil, nil)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}
