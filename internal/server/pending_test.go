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

// twoBrowsers signs both players in through the same handler, so each has a
// cookie of their own. Confirmation is the one step that genuinely needs two
// people, and a test that fakes the second one proves less than it looks.
func twoBrowsers(t *testing.T) (h http.Handler, store *memStore, anna, bodo *http.Cookie) {
	t.Helper()

	store = newMemStore()
	h = newHandlerWith(store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))

	anna = sessionCookie(t, join(t, h, "Anna"))
	bodo = sessionCookie(t, join(t, h, "Bodo"))
	return h, store, anna, bodo
}

// reportedByAnna records a match Anna claims she won 2:0, leaving Bodo to
// rule on it.
func reportedByAnna(t *testing.T, h http.Handler, store *memStore, anna *http.Cookie) string {
	t.Helper()

	rec := recordMatch(t, h, anna, opponentID(t, store, "Bodo"), 3, 11, "11:9", "12:10")
	if rec.Code != http.StatusOK {
		t.Fatalf("recording the match: status %d: %s", rec.Code, rec.Body.String())
	}

	stored := store.matches.all()
	if len(stored) != 1 {
		t.Fatalf("%d matches stored, want 1", len(stored))
	}
	return stored[0].ID.String()
}

func post(t *testing.T, h http.Handler, path string, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()

	r := httptest.NewRequest(http.MethodPost, path, nil)
	if cookie != nil {
		r.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

func fragment(t *testing.T, h http.Handler, path string, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()

	r := httptest.NewRequest(http.MethodGet, path, nil)
	if cookie != nil {
		r.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

func ttrOfPlayer(t *testing.T, store *memStore, name string) int {
	t.Helper()

	players, err := store.Players().List(t.Context())
	if err != nil {
		t.Fatalf("List(): %v", err)
	}
	for _, p := range players {
		if p.DisplayName == name {
			return p.TTR
		}
	}
	t.Fatalf("no player named %q", name)
	return 0
}

func TestThePendingListOnlyReachesTheOpponent(t *testing.T) {
	h, store, anna, bodo := twoBrowsers(t)
	reportedByAnna(t, h, store, anna)

	forBodo := fragment(t, h, "/fragments/pending", bodo)
	if !strings.Contains(forBodo.Body.String(), "Anna") {
		t.Errorf("Bodo does not see the result Anna entered: %s", forBodo.Body.String())
	}

	// Anna entered it, so there is nothing for her to agree with — and
	// nothing to read either. An empty list renders an empty section, kept
	// only so the out-of-band swaps have something to aim at.
	forAnna := fragment(t, h, "/fragments/pending", anna).Body.String()
	if strings.Contains(forAnna, "/confirm") {
		t.Errorf("Anna is asked to confirm her own result: %s", forAnna)
	}
	if strings.Contains(forAnna, "Zu bestätigen") {
		t.Errorf("an empty list still costs a heading: %s", forAnna)
	}
}

// TestThePendingListReadsFromTheOpponentsSide checks the score is turned
// around: Bodo lost 0:2, and showing him "2:0 für dich" would be worse than
// showing nothing.
func TestThePendingListReadsFromTheOpponentsSide(t *testing.T) {
	h, store, anna, bodo := twoBrowsers(t)
	reportedByAnna(t, h, store, anna)

	body := fragment(t, h, "/fragments/pending", bodo).Body.String()

	if !strings.Contains(body, "2:0 für Anna") {
		t.Errorf("the result is not shown from Bodo's side: %s", body)
	}
	if !strings.Contains(body, "9:11") || !strings.Contains(body, "10:12") {
		t.Errorf("the set scores are not turned around: %s", body)
	}
}

// TestConfirmingSettlesTheMatch is the Definition of Done of AP5, from the
// other end: before the confirmation nothing has moved, after it both
// ratings have.
func TestConfirmingSettlesTheMatch(t *testing.T) {
	h, store, anna, bodo := twoBrowsers(t)
	id := reportedByAnna(t, h, store, anna)

	if got := ttrOfPlayer(t, store, "Anna"); got != domain.DefaultTTR {
		t.Fatalf("Anna is on %d before the confirmation, want %d", got, domain.DefaultTTR)
	}

	rec := post(t, h, "/matches/"+id+"/confirm", bodo)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Bestätigt") || !strings.Contains(body, "Verloren") {
		t.Errorf("the confirmation does not state the outcome for Bodo: %s", body)
	}
	// Equal ratings, Bodo lost: -8.
	if !strings.Contains(body, "-8") || !strings.Contains(body, "992") {
		t.Errorf("the confirmation does not state Bodo's rating change: %s", body)
	}
	// A confirmation changes the roster, so it comes back in the same
	// response rather than leaving a stale rating on screen.
	if !strings.Contains(body, `hx-swap-oob="true"`) {
		t.Errorf("the response does not refresh the ranking out of band: %s", body)
	}

	if got := ttrOfPlayer(t, store, "Anna"); got != 1008 {
		t.Errorf("Anna is on %d, want 1008", got)
	}
	if got := ttrOfPlayer(t, store, "Bodo"); got != 992 {
		t.Errorf("Bodo is on %d, want 992", got)
	}
	if store.matches.all()[0].Status != domain.MatchConfirmed {
		t.Errorf("status = %q, want confirmed", store.matches.all()[0].Status)
	}
}

func TestTheReporterCannotConfirmTheirOwnResult(t *testing.T) {
	h, store, anna, _ := twoBrowsers(t)
	id := reportedByAnna(t, h, store, anna)

	rec := post(t, h, "/matches/"+id+"/confirm", anna)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	if got := ttrOfPlayer(t, store, "Anna"); got != domain.DefaultTTR {
		t.Errorf("Anna is on %d, want the starting %d", got, domain.DefaultTTR)
	}
}

func TestConfirmingTwice(t *testing.T) {
	h, store, anna, bodo := twoBrowsers(t)
	id := reportedByAnna(t, h, store, anna)

	if rec := post(t, h, "/matches/"+id+"/confirm", bodo); rec.Code != http.StatusOK {
		t.Fatalf("first confirmation: status %d", rec.Code)
	}

	rec := post(t, h, "/matches/"+id+"/confirm", bodo)

	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", rec.Code)
	}
	if got := ttrOfPlayer(t, store, "Anna"); got != 1008 {
		t.Errorf("Anna is on %d after a repeated confirmation, want 1008", got)
	}
}

// correct posts a corrected result for a contested match. The sets are read
// from the correcting player's own side, exactly as the form presents them.
func correct(t *testing.T, h http.Handler, cookie *http.Cookie, id string, bestOf, points int, sets ...string) *httptest.ResponseRecorder {
	t.Helper()

	form := url.Values{
		"best_of":       {strconv.Itoa(bestOf)},
		"points_to_win": {strconv.Itoa(points)},
	}
	for i, s := range sets {
		own, opponent, _ := strings.Cut(s, ":")
		form.Set("set_home_"+strconv.Itoa(i+1), own)
		form.Set("set_away_"+strconv.Itoa(i+1), opponent)
	}

	r := httptest.NewRequest(http.MethodPost, "/matches/"+id+"/correct", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if cookie != nil {
		r.AddCookie(cookie)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

func TestDisputingBlocksTheResult(t *testing.T) {
	h, store, anna, bodo := twoBrowsers(t)
	id := reportedByAnna(t, h, store, anna)

	rec := post(t, h, "/matches/"+id+"/dispute", bodo)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Strittig") {
		t.Errorf("the response does not say the result is contested: %s", rec.Body.String())
	}

	if got := ttrOfPlayer(t, store, "Anna"); got != domain.DefaultTTR {
		t.Errorf("Anna is on %d after a dispute, want the starting %d", got, domain.DefaultTTR)
	}
	if store.matches.all()[0].Status != domain.MatchDisputed {
		t.Errorf("status = %q, want disputed", store.matches.all()[0].Status)
	}

	// A contested result is not a confirmable one. Only a correction gets it
	// moving again.
	if again := post(t, h, "/matches/"+id+"/confirm", bodo); again.Code != http.StatusConflict {
		t.Errorf("confirming after a dispute = %d, want 409", again.Code)
	}
}

// TestTheCorrectionFormOpensOnWhatWasReported is the reason a correction is
// worth using at all: a contested result is usually nearly right, and
// retyping all of it to fix one number is how people go back to shrugging.
func TestTheCorrectionFormOpensOnWhatWasReported(t *testing.T) {
	h, store, anna, bodo := twoBrowsers(t)
	id := reportedByAnna(t, h, store, anna)

	body := post(t, h, "/matches/"+id+"/dispute", bodo).Body.String()

	// Anna claimed 11:9, 12:10 for herself, so Bodo's side of it is
	// 9:11 and 10:12 — and the mode she chose comes back too.
	for _, want := range []string{
		`name="set_home_1" value="9"`, `name="set_away_1" value="11"`,
		`name="set_home_2" value="10"`, `name="set_away_2" value="12"`,
		`value="3" selected`,
		`hx-post="/matches/` + id + `/correct"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the correction form does not contain %q: %s", want, body)
		}
	}
}

// TestACorrectionSurvivesAReload covers the failure that would make the whole
// feature useless: if the form only existed in the answer to the dispute, one
// page reload would put the match back out of reach.
func TestACorrectionSurvivesAReload(t *testing.T) {
	h, store, anna, bodo := twoBrowsers(t)
	id := reportedByAnna(t, h, store, anna)
	post(t, h, "/matches/"+id+"/dispute", bodo)

	for _, who := range []struct {
		name   string
		cookie *http.Cookie
	}{{"the player who disputed", bodo}, {"the player who reported", anna}} {
		body := fragment(t, h, "/fragments/pending", who.cookie).Body.String()
		if !strings.Contains(body, "/correct") {
			t.Errorf("%s cannot reach the correction after a reload: %s", who.name, body)
		}
	}
}

// TestCorrectingHandsTheMatchBack is the Definition of Done: a contested
// result becomes a confirmed one without SQL.
func TestCorrectingHandsTheMatchBack(t *testing.T) {
	h, store, anna, bodo := twoBrowsers(t)
	id := reportedByAnna(t, h, store, anna)
	post(t, h, "/matches/"+id+"/dispute", bodo)

	// Bodo says he won it 2:1 — from his side of the table.
	rec := correct(t, h, bodo, id, 3, 11, "11:9", "8:11", "11:7")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "2:1") {
		t.Errorf("the response does not name the corrected result: %s", rec.Body.String())
	}

	stored := store.matches.all()[0]
	if stored.Status != domain.MatchPending {
		t.Fatalf("status = %q, want pending", stored.Status)
	}
	// Whoever corrects becomes the reporter, which is what hands the
	// decision to the other one.
	if stored.ReportedBy != stored.AwayID {
		t.Errorf("the correction did not make Bodo the reporter")
	}
	// Bodo played away, so his 11:9 is stored as 9:11.
	if got := stored.Sets[0]; got.HomePoints != 9 || got.AwayPoints != 11 {
		t.Errorf("set 1 stored as %d:%d, want 9:11", got.HomePoints, got.AwayPoints)
	}
	if len(stored.Sets) != 3 {
		t.Errorf("%d sets stored, want 3", len(stored.Sets))
	}

	// And now the other side settles it.
	if rec := post(t, h, "/matches/"+id+"/confirm", anna); rec.Code != http.StatusOK {
		t.Fatalf("confirming the correction: status %d: %s", rec.Code, rec.Body.String())
	}
	if got := ttrOfPlayer(t, store, "Bodo"); got != 1008 {
		t.Errorf("Bodo is on %d after winning the corrected match, want 1008", got)
	}
}

// TestTheReporterMayCorrectToo: the person who mistyped it knows what they
// meant, and sending them back to the table to borrow the other phone would
// be an odd thing for the app to insist on.
func TestTheReporterMayCorrectToo(t *testing.T) {
	h, store, anna, bodo := twoBrowsers(t)
	id := reportedByAnna(t, h, store, anna)
	post(t, h, "/matches/"+id+"/dispute", bodo)

	if rec := correct(t, h, anna, id, 3, 11, "11:9", "9:11", "11:8"); rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	stored := store.matches.all()[0]
	if stored.ReportedBy != stored.HomeID {
		t.Errorf("the reporter is not the player who corrected it")
	}
	if rec := post(t, h, "/matches/"+id+"/confirm", bodo); rec.Code != http.StatusOK {
		t.Errorf("the opponent could not confirm the correction: %d", rec.Code)
	}
}

// TestACorrectionIsHeldToTheSameRules: a corrected result gets no discount on
// what counts as possible, and the refusal still says why.
func TestACorrectionIsHeldToTheSameRules(t *testing.T) {
	h, store, anna, bodo := twoBrowsers(t)
	id := reportedByAnna(t, h, store, anna)
	post(t, h, "/matches/"+id+"/dispute", bodo)

	rec := correct(t, h, bodo, id, 3, 11, "11:10", "11:9")

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "zwei Punkte Vorsprung") {
		t.Errorf("the refusal does not say why: %s", body)
	}
	// What was typed comes back, so nobody re-enters the whole match.
	if !strings.Contains(body, `name="set_home_1" value="11"`) {
		t.Errorf("the typed correction was not returned: %s", body)
	}
	if store.matches.all()[0].Status != domain.MatchDisputed {
		t.Error("an impossible correction moved the match anyway")
	}
}

func TestOnlyAContestedMatchCanBeCorrected(t *testing.T) {
	h, store, anna, bodo := twoBrowsers(t)
	id := reportedByAnna(t, h, store, anna)

	// Still pending: it is waiting for a plain yes or no, not a rewrite.
	if rec := correct(t, h, bodo, id, 3, 11, "11:9", "11:7"); rec.Code != http.StatusConflict {
		t.Errorf("correcting a pending match = %d, want 409", rec.Code)
	}

	post(t, h, "/matches/"+id+"/confirm", bodo)
	if rec := correct(t, h, bodo, id, 3, 11, "11:9", "11:7"); rec.Code != http.StatusConflict {
		t.Errorf("correcting a confirmed match = %d, want 409", rec.Code)
	}
	if got := ttrOfPlayer(t, store, "Anna"); got != 1008 {
		t.Errorf("Anna is on %d, so a settled result was rewritten", got)
	}
}

func TestABystanderCannotCorrect(t *testing.T) {
	h, store, anna, bodo := twoBrowsers(t)
	id := reportedByAnna(t, h, store, anna)
	post(t, h, "/matches/"+id+"/dispute", bodo)

	cara := sessionCookie(t, join(t, h, "Cara"))

	if rec := correct(t, h, cara, id, 3, 11, "11:9", "11:7"); rec.Code != http.StatusForbidden {
		t.Errorf("a bystander correcting = %d, want 403", rec.Code)
	}
	if store.matches.all()[0].Status != domain.MatchDisputed {
		t.Error("a bystander moved the match")
	}
}

func TestRulingRequiresRecognition(t *testing.T) {
	h, store, anna, _ := twoBrowsers(t)
	id := reportedByAnna(t, h, store, anna)

	for _, path := range []string{
		"/matches/" + id + "/confirm",
		"/matches/" + id + "/dispute",
		"/matches/" + id + "/correct",
	} {
		if rec := post(t, h, path, nil); rec.Code != http.StatusUnauthorized {
			t.Errorf("POST %s without a cookie = %d, want 401", path, rec.Code)
		}
	}
}

func TestRulingOnSomethingThatIsNotAMatch(t *testing.T) {
	h, _, _, bodo := twoBrowsers(t)

	if rec := post(t, h, "/matches/not-a-uuid/confirm", bodo); rec.Code != http.StatusNotFound {
		t.Errorf("a malformed id = %d, want 404", rec.Code)
	}
	if rec := post(t, h, "/matches/"+uuid.New().String()+"/confirm", bodo); rec.Code != http.StatusNotFound {
		t.Errorf("an unknown id = %d, want 404", rec.Code)
	}
}

func TestTheStartPageShowsWhatIsWaiting(t *testing.T) {
	h, store, anna, bodo := twoBrowsers(t)
	reportedByAnna(t, h, store, anna)

	body := fragment(t, h, "/", bodo).Body.String()

	if !strings.Contains(body, "Zu bestätigen") {
		t.Errorf("the start page does not show the pending section: %s", body)
	}
	if !strings.Contains(body, "Stimmt nicht") {
		t.Errorf("the start page offers no way to disagree: %s", body)
	}
}

// TestRefreshBringsSomebodyElsesResultOntoThePage is what the button is for.
//
// The three things that change because *somebody else* acted — the ranking,
// the results waiting on you, and the badge — never moved without a reload.
// The badge polled and the list under it did not, so the bar could say one
// result was waiting while the page below showed nothing.
func TestRefreshBringsSomebodyElsesResultOntoThePage(t *testing.T) {
	h, store, anna, bodo := twoBrowsers(t)

	if body := fragment(t, h, "/fragments/refresh", bodo).Body.String(); strings.Contains(body, "Zu bestätigen") {
		t.Fatalf("something is waiting on Bodo before anybody played: %s", body)
	}

	reportedByAnna(t, h, store, anna)

	rec := fragment(t, h, "/fragments/refresh", bodo)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()

	// All three arrive out of band, so one press catches the page up rather
	// than three presses catching up one region each.
	for _, want := range []string{`id="standings"`, `id="pending"`, `id="whoami"`} {
		if !strings.Contains(body, want) {
			t.Errorf("the refresh does not carry %s: %s", want, body)
		}
	}
	if strings.Count(body, "hx-swap-oob") != 3 {
		t.Errorf("%d out-of-band swaps, want 3: %s", strings.Count(body, "hx-swap-oob"), body)
	}

	// And the content, not just the shape: Anna's report is now on Bodo's
	// page without him having reloaded it.
	if !strings.Contains(body, "Zu bestätigen") {
		t.Errorf("the result Anna reported did not reach Bodo: %s", body)
	}
	if !strings.Contains(body, "Anna") {
		t.Errorf("the waiting result does not name who reported it: %s", body)
	}
}

// TestRefreshSignedOutOnlyBringsTheRanking: there is nothing waiting on a
// reader nobody is recognised as, and asking for it would need a player id
// that does not exist.
func TestRefreshSignedOutOnlyBringsTheRanking(t *testing.T) {
	h, store, anna, _ := twoBrowsers(t)
	reportedByAnna(t, h, store, anna)

	body := fragment(t, h, "/fragments/refresh", nil).Body.String()

	if !strings.Contains(body, `id="standings"`) {
		t.Errorf("a signed-out refresh does not carry the ranking: %s", body)
	}
	if strings.Contains(body, `id="pending"`) {
		t.Errorf("a signed-out refresh carries a pending list: %s", body)
	}
}
