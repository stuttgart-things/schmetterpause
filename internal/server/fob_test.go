package server_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/auth"
	"github.com/stuttgart-things/schmetterpause/internal/domain"
	"github.com/stuttgart-things/schmetterpause/internal/server"
)

// testFobKey keys the fob hash in these tests; its content does not matter.
var testFobKey = []byte("fedcba9876543210fedcba9876543210")

// A UID as a reader prints it, and the same one as firmware might.
const (
	testUID        = "04:A3:1B:22:5C:80:01"
	testUIDCompact = "04a31b225c8001"
	strangerUID    = "DE:AD:BE:EF"
)

// fobServer wires a server with the Zählwerk's surface and fobs on, and keeps
// every log line at debug level so a test can look for what must not be in it.
func fobServer(t *testing.T) (http.Handler, *memStore, *bytes.Buffer) {
	t.Helper()

	store := newMemStore()
	cfg := testConfig()
	cfg.ScoreboardToken = testScoreboardToken
	cfg.SessionKey = testSessionKey
	cfg.RFIDUIDKey = testFobKey

	var logs bytes.Buffer
	log := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	h := server.New(cfg, store, log,
		auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false),
		server.Build{Version: "test"}).Handler()
	return h, store, &logs
}

func resolveFob(t *testing.T, h http.Handler, token, uid, half string) *httptest.ResponseRecorder {
	t.Helper()

	raw, err := json.Marshal(map[string]string{"uid": uid, "half": half})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/identities/rfid/resolve", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

type resolved struct {
	PlayerID  uuid.UUID `json:"player_id"`
	CheckinID uuid.UUID `json:"checkin_id"`
}

func decodeResolved(t *testing.T, rec *httptest.ResponseRecorder) resolved {
	t.Helper()

	if rec.Code != http.StatusOK {
		t.Fatalf("resolve = %d %s, want 200", rec.Code, rec.Body)
	}
	var out resolved
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decoding the answer: %v", err)
	}
	return out
}

// joined creates a signed-in player and returns their cookie and id.
func joined(t *testing.T, h http.Handler, store *memStore, name string) (*http.Cookie, uuid.UUID) {
	t.Helper()

	cookie := sessionCookie(t, join(t, h, name))
	p, err := store.Players().ByDisplayName(t.Context(), name)
	if err != nil {
		t.Fatalf("finding %s: %v", name, err)
	}
	return cookie, p.ID
}

// The property the feature rests on: no key, no route — even with the
// Zählwerk's token set. A 404 like any unknown path, the same posture as /api
// without SP_SCOREBOARD_TOKEN (docs/adr/0025).
func TestFobRoutesDoNotExistWithoutTheKey(t *testing.T) {
	store := newMemStore()
	cfg := testConfig()
	cfg.ScoreboardToken = testScoreboardToken
	cfg.SessionKey = testSessionKey
	h := newHandlerConfig(cfg, store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))

	if rec := resolveFob(t, h, testScoreboardToken, testUID, "left"); rec.Code != http.StatusNotFound {
		t.Errorf("resolve without SP_RFID_UID_KEY = %d, want 404", rec.Code)
	}

	cookie := sessionCookie(t, join(t, h, "Anna"))
	if rec := post(t, h, "/fobs/window", cookie); rec.Code != http.StatusNotFound {
		t.Errorf("POST /fobs/window without SP_RFID_UID_KEY = %d, want 404", rec.Code)
	}
}

func TestFobsAreOffSaysSoInTheLog(t *testing.T) {
	var logs bytes.Buffer
	cfg := testConfig()
	server.New(cfg, newMemStore(), slog.New(slog.NewTextHandler(&logs, nil)),
		auth.Anonymous{}, server.Build{Version: "test"})

	if !strings.Contains(logs.String(), "SP_RFID_UID_KEY is not set") {
		t.Errorf("starting without a fob key logged %q, want a line naming SP_RFID_UID_KEY", logs.String())
	}
}

func TestResolvingAFobNeedsTheToken(t *testing.T) {
	h, _, _ := fobServer(t)

	for _, token := range []string{"", "wrong"} {
		if rec := resolveFob(t, h, token, testUID, "left"); rec.Code != http.StatusUnauthorized {
			t.Errorf("resolve with token %q = %d, want 401", token, rec.Code)
		}
	}
}

// A badge held against the reader by mistake has to pass through without a
// trace: 404, no row anywhere, and neither the UID nor anything derived from
// it in the log (docs/adr/0025).
func TestAnUnknownFobLeavesNothingBehind(t *testing.T) {
	h, store, logs := fobServer(t)

	rec := resolveFob(t, h, testScoreboardToken, strangerUID, "left")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown fob = %d, want 404", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("unknown fob answered %q, want an empty body", rec.Body)
	}
	if n := len(store.checkins.all()); n != 0 {
		t.Errorf("an unknown fob recorded %d check-ins, want none", n)
	}
	if n := len(store.identities.rows); n != 0 {
		t.Errorf("an unknown fob left %d identities, want none", n)
	}

	for _, trace := range []string{"DE:AD", "DEADBEEF", "deadbeef", "de:ad"} {
		if strings.Contains(logs.String(), trace) {
			t.Errorf("the log mentions the UID (%q): %s", trace, logs)
		}
	}
	if !strings.Contains(logs.String(), "unknown fob") {
		t.Errorf("the log does not say an unknown fob came by: %s", logs)
	}
}

// The whole path: open the window on the profile, hold the fob to the reader,
// and from then on that fob is this player — however the UID is written.
func TestAFobLinkedInTheWindowResolvesToItsPlayer(t *testing.T) {
	h, store, logs := fobServer(t)
	cookie, anna := joined(t, h, store, "Anna")

	if rec := post(t, h, "/fobs/window", cookie); rec.Code != http.StatusOK ||
		!strings.Contains(rec.Body.String(), "Halte jetzt deinen Anhänger") {
		t.Fatalf("opening the window = %d %s", rec.Code, rec.Body)
	}

	first := decodeResolved(t, resolveFob(t, h, testScoreboardToken, testUID, "left"))
	if first.PlayerID != anna {
		t.Errorf("linking resolved to %s, want Anna %s", first.PlayerID, anna)
	}

	// The window is used up: the next unknown fob is not Anna's.
	if rec := resolveFob(t, h, testScoreboardToken, strangerUID, "right"); rec.Code != http.StatusNotFound {
		t.Errorf("a second fob after linking = %d, want 404", rec.Code)
	}

	again := decodeResolved(t, resolveFob(t, h, testScoreboardToken, testUIDCompact, "right"))
	if again.PlayerID != anna {
		t.Errorf("the same fob written differently resolved to %s, want Anna", again.PlayerID)
	}
	if again.CheckinID == first.CheckinID {
		t.Error("two resolves returned the same check-in; each one is a check-in of its own")
	}

	checkins := store.checkins.all()
	if len(checkins) != 2 {
		t.Fatalf("recorded %d check-ins, want 2", len(checkins))
	}
	for i, want := range []domain.Half{domain.HalfLeft, domain.HalfRight} {
		c := checkins[i]
		if c.PlayerID != anna || c.Half != want || c.Method != domain.CheckinRFID {
			t.Errorf("check-in %d = %+v, want Anna at %s by rfid", i, c, want)
		}
	}

	if strings.Contains(strings.ToLower(logs.String()), "04a31b") {
		t.Errorf("the log mentions the UID: %s", logs)
	}
}

// Only one window: otherwise two people pressing "koppeln" at once could each
// end up with the other's fob.
func TestTwoWindowsCannotBeOpenAtOnce(t *testing.T) {
	h, store, _ := fobServer(t)
	annaCookie, anna := joined(t, h, store, "Anna")
	beaCookie, _ := joined(t, h, store, "Bea")

	post(t, h, "/fobs/window", annaCookie)

	rec := post(t, h, "/fobs/window", beaCookie)
	if !strings.Contains(rec.Body.String(), "Gerade koppelt jemand anderes") {
		t.Errorf("a second window answered %s, want the busy message", rec.Body)
	}

	if got := decodeResolved(t, resolveFob(t, h, testScoreboardToken, testUID, "left")); got.PlayerID != anna {
		t.Errorf("the fob went to %s, want Anna, who opened the window first", got.PlayerID)
	}
}

// A fob that belongs to somebody resolves to them and leaves another player's
// open window alone: it is not relinked, and it does not use the window up.
func TestAKnownFobDoesNotTakeSomebodyElsesWindow(t *testing.T) {
	h, store, _ := fobServer(t)
	annaCookie, anna := joined(t, h, store, "Anna")
	beaCookie, bea := joined(t, h, store, "Bea")

	post(t, h, "/fobs/window", annaCookie)
	decodeResolved(t, resolveFob(t, h, testScoreboardToken, testUID, "left"))

	post(t, h, "/fobs/window", beaCookie)
	if got := decodeResolved(t, resolveFob(t, h, testScoreboardToken, testUID, "left")); got.PlayerID != anna {
		t.Errorf("Anna's fob in Bea's window resolved to %s, want Anna", got.PlayerID)
	}
	if got := decodeResolved(t, resolveFob(t, h, testScoreboardToken, strangerUID, "right")); got.PlayerID != bea {
		t.Errorf("Bea's window was used up by a fob that was not hers: next fob went to %s", got.PlayerID)
	}
}

func TestAnExpiredWindowLinksNothing(t *testing.T) {
	h, store, _ := fobServer(t)
	cookie, _ := joined(t, h, store, "Anna")

	post(t, h, "/fobs/window", cookie)
	store.fobWindow.expire()

	if rec := resolveFob(t, h, testScoreboardToken, testUID, "left"); rec.Code != http.StatusNotFound {
		t.Errorf("a fob after the window ran out = %d, want 404", rec.Code)
	}

	rec := getWith(t, h, "/fragments/fobs?known=0", cookie)
	if !strings.Contains(rec.Body.String(), "abgelaufen") {
		t.Errorf("the card after expiry says %s, want that the window ran out", rec.Body)
	}
}

func TestCancellingTheWindow(t *testing.T) {
	h, store, _ := fobServer(t)
	cookie, _ := joined(t, h, store, "Anna")

	post(t, h, "/fobs/window", cookie)
	post(t, h, "/fobs/window/close", cookie)

	if rec := resolveFob(t, h, testScoreboardToken, testUID, "left"); rec.Code != http.StatusNotFound {
		t.Errorf("a fob after cancelling = %d, want 404", rec.Code)
	}
}

// The card polls while the window is open, and turns into "gekoppelt" once
// the fob was at the reader.
func TestThePollSeesTheFobArrive(t *testing.T) {
	h, store, _ := fobServer(t)
	cookie, _ := joined(t, h, store, "Anna")

	open := post(t, h, "/fobs/window", cookie)
	if !strings.Contains(open.Body.String(), `hx-get="/fragments/fobs?known=0"`) {
		t.Errorf("the open card does not poll: %s", open.Body)
	}

	decodeResolved(t, resolveFob(t, h, testScoreboardToken, testUID, "left"))

	rec := getWith(t, h, "/fragments/fobs?known=0", cookie)
	body := rec.Body.String()
	if !strings.Contains(body, "Anhänger gekoppelt") {
		t.Errorf("the card after linking says %s, want that it is linked", body)
	}
	if strings.Contains(body, "hx-trigger") {
		t.Error("the card keeps polling after the window closed")
	}
}

// A player sees their fobs on their own profile and can remove each one;
// a removed fob is unknown again.
func TestAPlayerSeesAndRemovesTheirFobs(t *testing.T) {
	h, store, _ := fobServer(t)
	cookie, anna := joined(t, h, store, "Anna")

	post(t, h, "/fobs/window", cookie)
	decodeResolved(t, resolveFob(t, h, testScoreboardToken, testUID, "left"))

	page := getWith(t, h, "/players/"+anna.String(), cookie).Body.String()
	if !strings.Contains(page, "Schlüsselanhänger") || !strings.Contains(page, "zuletzt benutzt") {
		t.Fatalf("the profile does not list the fob: %s", page)
	}

	fobs, err := store.identities.ForPlayer(t.Context(), anna)
	if err != nil || len(fobs) == 0 {
		t.Fatalf("no identities for Anna: %v", err)
	}
	var subject string
	for _, id := range fobs {
		if id.Provider == domain.ProviderRFID {
			subject = id.Subject
		}
	}
	if strings.Contains(strings.ToLower(subject), "04a31b") {
		t.Errorf("the fob is stored as %q, which contains the UID", subject)
	}

	postForm(t, h, "/fobs/remove", url.Values{"subject": {subject}}, cookie)

	if rec := resolveFob(t, h, testScoreboardToken, testUID, "left"); rec.Code != http.StatusNotFound {
		t.Errorf("a removed fob = %d, want 404", rec.Code)
	}
}

func TestNobodyRemovesSomebodyElsesFob(t *testing.T) {
	h, store, _ := fobServer(t)
	annaCookie, anna := joined(t, h, store, "Anna")
	beaCookie, _ := joined(t, h, store, "Bea")

	post(t, h, "/fobs/window", annaCookie)
	decodeResolved(t, resolveFob(t, h, testScoreboardToken, testUID, "left"))

	fobs, _ := store.identities.ForPlayer(t.Context(), anna)
	for _, id := range fobs {
		postForm(t, h, "/fobs/remove", url.Values{"subject": {id.Subject}}, beaCookie)
	}

	if got := decodeResolved(t, resolveFob(t, h, testScoreboardToken, testUID, "left")); got.PlayerID != anna {
		t.Errorf("after Bea tried to remove it, the fob resolved to %s, want Anna", got.PlayerID)
	}
}

func TestTheFobCardIsOnlyOnYourOwnPage(t *testing.T) {
	h, store, _ := fobServer(t)
	_, anna := joined(t, h, store, "Anna")
	beaCookie, _ := joined(t, h, store, "Bea")

	if page := getWith(t, h, "/players/"+anna.String(), beaCookie).Body.String(); strings.Contains(page, "Schlüsselanhänger") {
		t.Error("Bea sees Anna's fob card")
	}
	if rec := post(t, h, "/fobs/window", nil); rec.Code == http.StatusOK {
		t.Errorf("opening a window signed out = %d, want a refusal", rec.Code)
	}
}

// A malformed request is a bug on the sending side. The refusal says what is
// wrong without repeating the input, because the input is a card number.
func TestAMalformedResolveIsRefusedWithoutEchoingTheUID(t *testing.T) {
	h, store, _ := fobServer(t)

	for _, tc := range []struct{ name, uid, half string }{
		{"not hex", "zz:zz:zz:zz", "left"},
		{"too short", "04:A3", "left"},
		{"no half", testUID, ""},
		{"a half that is not one", testUID, "middle"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := resolveFob(t, h, testScoreboardToken, tc.uid, tc.half)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("= %d, want 400", rec.Code)
			}
			if body := rec.Body.String(); tc.uid != "" && strings.Contains(body, tc.uid) {
				t.Errorf("the refusal repeats the UID: %s", body)
			}
		})
	}

	raw := `{"uid":"04:A3:1B:22","half":"left","extra":"x"}`
	req := httptest.NewRequest(http.MethodPost, "/api/identities/rfid/resolve", strings.NewReader(raw))
	req.Header.Set("Authorization", "Bearer "+testScoreboardToken)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if body, _ := io.ReadAll(rec.Body); rec.Code != http.StatusBadRequest || strings.Contains(string(body), "04:A3") {
		t.Errorf("unknown field = %d %s, want 400 without the UID", rec.Code, body)
	}

	if n := len(store.checkins.all()); n != 0 {
		t.Errorf("malformed requests recorded %d check-ins", n)
	}
}
