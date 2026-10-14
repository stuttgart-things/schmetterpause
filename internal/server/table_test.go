package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/auth"
	"github.com/stuttgart-things/schmetterpause/internal/domain"
)

// fakeZaehlwerk plays the Zählwerk's POST /checkin/claim
// (stuttgart-things/zaehlwerk#116) and keeps every claim it got.
type fakeZaehlwerk struct {
	mu     sync.Mutex
	claims []fakeClaim
	// status and body are what it answers. 200 with a "claimed" body when
	// unset.
	status int
	body   string
}

type fakeClaim struct {
	Auth      string
	Half      domain.Half `json:"half"`
	PlayerID  uuid.UUID   `json:"player_id"`
	CheckinID uuid.UUID   `json:"checkin_id"`
}

func (f *fakeZaehlwerk) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || r.URL.Path != "/checkin/claim" {
		http.NotFound(w, r)
		return
	}
	var c fakeClaim
	_ = json.NewDecoder(r.Body).Decode(&c)
	c.Auth = r.Header.Get("Authorization")

	f.mu.Lock()
	f.claims = append(f.claims, c)
	status, body := f.status, f.body
	f.mu.Unlock()

	if status == 0 {
		status, body = http.StatusOK, `{"outcome":"claimed","half":"`+string(c.Half)+`","left":"Anna"}`
	}
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}

func (f *fakeZaehlwerk) got() []fakeClaim {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]fakeClaim(nil), f.claims...)
}

// tableServer wires a server whose Zählwerk is zw.
func tableServer(t *testing.T, zw *fakeZaehlwerk) (http.Handler, *memStore) {
	t.Helper()

	srv := httptest.NewServer(zw)
	t.Cleanup(srv.Close)

	store := newMemStore()
	cfg := testConfig()
	cfg.ScoreboardToken = testScoreboardToken
	cfg.SessionKey = testSessionKey
	cfg.ZaehlwerkURL = srv.URL
	h := newHandlerConfig(cfg, store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))
	return h, store
}

func getAs(t *testing.T, h http.Handler, path string, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	return fragment(t, h, path, cookie)
}

// No Zählwerk address, or no token: the pages do not exist, and /qr does not
// point at them (docs/adr/0026).
func TestTablePagesDoNotExistWithoutTheZaehlwerk(t *testing.T) {
	for _, tc := range []struct {
		name       string
		url, token string
	}{
		{"neither", "", ""},
		{"no token", "http://zaehlwerk.example", ""},
		{"no address", "", testScoreboardToken},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := newMemStore()
			cfg := testConfig()
			cfg.SessionKey = testSessionKey
			cfg.ZaehlwerkURL, cfg.ScoreboardToken = tc.url, tc.token
			h := newHandlerConfig(cfg, store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))

			for _, path := range []string{"/table/left", "/qr/table"} {
				if rec := get(t, h, path); rec.Code != http.StatusNotFound {
					t.Errorf("GET %s = %d, want 404", path, rec.Code)
				}
			}
			if body := get(t, h, "/qr").Body.String(); strings.Contains(body, "/qr/table") {
				t.Error("/qr links the table sheet while table check-in is off")
			}
		})
	}
}

func TestTheTablePageChecksNobodyIn(t *testing.T) {
	zw := &fakeZaehlwerk{}
	h, store := tableServer(t, zw)

	// A stranger is told to sign in, and gets no button.
	body := get(t, h, "/table/left").Body.String()
	if !strings.Contains(body, "Zur Anmeldung") || strings.Contains(body, `hx-post="/table/left"`) {
		t.Errorf("signed-out page:\n%s", body)
	}

	cookie, _ := joined(t, h, store, "Anna")
	rec := getAs(t, h, "/table/left", cookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `hx-post="/table/left"`) {
		t.Fatalf("signed-in page = %d:\n%s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "linke Seite") {
		t.Error("the page does not say which half")
	}

	// Opening the link is not checking in: a preview or a scan in passing
	// must leave nothing behind.
	if n := len(store.checkins.all()); n != 0 {
		t.Errorf("GET created %d check-ins, want 0", n)
	}
	if n := len(zw.got()); n != 0 {
		t.Errorf("GET called the Zählwerk %d times, want 0", n)
	}

	if rec := get(t, h, "/table/middle"); rec.Code != http.StatusNotFound {
		t.Errorf("GET /table/middle = %d, want 404", rec.Code)
	}
}

func TestCheckingInWithThePhone(t *testing.T) {
	zw := &fakeZaehlwerk{}
	h, store := tableServer(t, zw)
	cookie, anna := joined(t, h, store, "Anna")

	rec := post(t, h, "/table/right", cookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Du bist angemeldet") {
		t.Fatalf("POST /table/right = %d:\n%s", rec.Code, rec.Body)
	}

	rows := store.checkins.all()
	if len(rows) != 1 {
		t.Fatalf("recorded %d check-ins, want 1", len(rows))
	}
	c := rows[0]
	if c.PlayerID != anna || c.Half != domain.HalfRight || c.Method != domain.CheckinPhone {
		t.Errorf("check-in = %+v, want Anna at right by phone", c)
	}

	claims := zw.got()
	if len(claims) != 1 {
		t.Fatalf("the Zählwerk got %d claims, want 1", len(claims))
	}
	got := claims[0]
	if got.Half != domain.HalfRight || got.PlayerID != anna || got.CheckinID != c.ID {
		t.Errorf("claim = %+v, want right, Anna, %s", got, c.ID)
	}
	if got.Auth != "Bearer "+testScoreboardToken {
		t.Errorf("Authorization = %q, want the scoreboard token", got.Auth)
	}
}

func TestTheTablePageSaysWhatTheZaehlwerkAnswered(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"both halves taken", http.StatusOK,
			`{"outcome":"started","left":"Bea","right":"Anna","match_id":"m"}`, "Das Match startet"},
		{"a match is running", http.StatusConflict,
			`{"outcome":"refused","error":"a match is running"}`, "läuft gerade ein Match"},
		{"check-in off over there", http.StatusServiceUnavailable, `{"outcome":"unavailable"}`, "nicht erreichbar"},
		{"wrong token", http.StatusUnauthorized, `{"error":"no"}`, "nicht erreichbar"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			zw := &fakeZaehlwerk{status: tc.status, body: tc.body}
			h, store := tableServer(t, zw)
			cookie, _ := joined(t, h, store, "Anna")

			body := post(t, h, "/table/right", cookie).Body.String()
			if !strings.Contains(body, tc.want) {
				t.Errorf("answer does not say %q:\n%s", tc.want, body)
			}
			// The row stays either way. It proves nothing alone and
			// expires by itself (docs/adr/0026).
			if n := len(store.checkins.all()); n != 1 {
				t.Errorf("%d check-ins, want 1", n)
			}
		})
	}
}

func TestAnObserverDoesNotCheckIn(t *testing.T) {
	zw := &fakeZaehlwerk{}
	h, store := tableServer(t, zw)
	cookie, id := joined(t, h, store, "Olga")
	if err := store.Players().SetObserver(t.Context(), id, true); err != nil {
		t.Fatal(err)
	}

	if body := getAs(t, h, "/table/left", cookie).Body.String(); !strings.Contains(body, "Beobachter") ||
		strings.Contains(body, `hx-post="/table/left"`) {
		t.Errorf("observer's page:\n%s", body)
	}
	post(t, h, "/table/left", cookie)
	if n := len(store.checkins.all()); n != 0 {
		t.Errorf("an observer's press created %d check-ins", n)
	}
	if n := len(zw.got()); n != 0 {
		t.Errorf("an observer's press reached the Zählwerk %d times", n)
	}
}

func TestCheckingInNeedsAPlayer(t *testing.T) {
	zw := &fakeZaehlwerk{}
	h, store := tableServer(t, zw)

	if rec := post(t, h, "/table/left", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("POST signed out = %d, want 401", rec.Code)
	}
	if len(store.checkins.all()) != 0 || len(zw.got()) != 0 {
		t.Error("a signed-out press left a trace")
	}
}

// Every press is a row and a call. The sixth in a row is not somebody waiting
// for an opponent, and it is answered without either.
func TestPressingOverAndOverIsBraked(t *testing.T) {
	zw := &fakeZaehlwerk{}
	h, store := tableServer(t, zw)
	cookie, _ := joined(t, h, store, "Anna")

	for range 5 {
		post(t, h, "/table/left", cookie)
	}
	body := post(t, h, "/table/left", cookie).Body.String()
	if !strings.Contains(body, "Einen Moment") {
		t.Errorf("sixth press:\n%s", body)
	}
	if n := len(zw.got()); n != 5 {
		t.Errorf("the Zählwerk got %d claims, want 5", n)
	}
}

// Two phone check-ins are proof like two fobs: they carry a result without an
// operator (docs/adr/0024).
func TestPhoneCheckinsCarryAResult(t *testing.T) {
	zw := &fakeZaehlwerk{}
	h, store := tableServer(t, zw)
	annaCookie, anna := joined(t, h, store, "Anna")
	beaCookie, bea := joined(t, h, store, "Bea")

	post(t, h, "/table/left", annaCookie)
	post(t, h, "/table/right", beaCookie)
	claims := zw.got()
	if len(claims) != 2 {
		t.Fatalf("%d claims, want 2", len(claims))
	}

	annaP, _ := store.Players().ByID(t.Context(), anna)
	beaP, _ := store.Players().ByID(t.Context(), bea)
	rec := apiPost(t, h, testScoreboardToken, checkedInResult(annaP, beaP, claims[0].CheckinID, claims[1].CheckinID))
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /api/results with phone check-ins = %d %s", rec.Code, rec.Body)
	}
}

func TestTheTableSheetHasACodePerHalf(t *testing.T) {
	h, _ := tableServer(t, &fakeZaehlwerk{})

	body := get(t, h, "/qr/table").Body.String()
	for _, want := range []string{"Linke Seite", "Rechte Seite", "/table/left", "/table/right"} {
		if !strings.Contains(body, want) {
			t.Errorf("table sheet lacks %q", want)
		}
	}
	if !strings.Contains(get(t, h, "/qr").Body.String(), `href="/qr/table"`) {
		t.Error("/qr does not link the table sheet")
	}
}
