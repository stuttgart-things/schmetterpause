package server_test

import (
	"net/http"
	"net/http/httptest"
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

	// Anna entered it, so there is nothing for her to agree with.
	forAnna := fragment(t, h, "/fragments/pending", anna)
	if !strings.Contains(forAnna.Body.String(), "Nichts zu bestätigen") {
		t.Errorf("Anna is asked to confirm her own result: %s", forAnna.Body.String())
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
		t.Errorf("the response does not refresh the roster out of band: %s", body)
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

func TestDisputingBlocksTheResult(t *testing.T) {
	h, store, anna, bodo := twoBrowsers(t)
	id := reportedByAnna(t, h, store, anna)

	rec := post(t, h, "/matches/"+id+"/dispute", bodo)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Widersprochen") || !strings.Contains(body, "zählt nicht") {
		t.Errorf("the response does not say the result is blocked: %s", body)
	}

	if got := ttrOfPlayer(t, store, "Anna"); got != domain.DefaultTTR {
		t.Errorf("Anna is on %d after a dispute, want the starting %d", got, domain.DefaultTTR)
	}
	if store.matches.all()[0].Status != domain.MatchDisputed {
		t.Errorf("status = %q, want disputed", store.matches.all()[0].Status)
	}

	// Resolving a dispute is a manual step in the MVP (issue #18), so a
	// second click must not undo it.
	if again := post(t, h, "/matches/"+id+"/confirm", bodo); again.Code != http.StatusConflict {
		t.Errorf("confirming after a dispute = %d, want 409", again.Code)
	}
}

func TestRulingRequiresRecognition(t *testing.T) {
	h, store, anna, _ := twoBrowsers(t)
	id := reportedByAnna(t, h, store, anna)

	for _, path := range []string{"/matches/" + id + "/confirm", "/matches/" + id + "/dispute"} {
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
