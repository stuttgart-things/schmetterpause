package server_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/auth"
	"github.com/stuttgart-things/schmetterpause/internal/domain"
	"github.com/stuttgart-things/schmetterpause/internal/server"
)

// adminHandler wires a server whose SP_BOOTSTRAP_ADMIN names bootstrap, and
// runs the grant the way serve does.
func adminHandler(t *testing.T, bootstrap string) (*server.Server, *memStore) {
	t.Helper()

	store := newMemStore()
	cfg := testConfig()
	cfg.SessionKey = testSessionKey
	cfg.BootstrapAdmin = bootstrap

	srv := server.New(cfg, store, discardLogger(),
		auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false),
		server.Build{Version: "test"})
	return srv, store
}

// discardLogger keeps the test output about the tests.
func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
}

func getWith(t *testing.T, h http.Handler, path string, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()

	r := httptest.NewRequest(http.MethodGet, path, nil)
	if cookie != nil {
		r.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

// The first admin comes from the environment. Issue #73 names "a way to grant
// it that is not psql" as the price of a flag on the player, and ADR-0008
// answers with a variable — the only form that fits invariant 2.
func TestTheBootstrapVariableGrantsTheFlag(t *testing.T) {
	srv, store := adminHandler(t, "Anna")
	h := srv.Handler()

	join(t, h, "Anna")
	join(t, h, "Bodo")

	srv.GrantBootstrapAdmin(t.Context())

	admins, err := store.Players().Admins(t.Context())
	if err != nil {
		t.Fatalf("Admins(): %v", err)
	}
	if len(admins) != 1 || admins[0].DisplayName != "Anna" {
		t.Fatalf("Admins() = %v, want only Anna", admins)
	}
}

// The variable names somebody by the name people call them, so it must not
// depend on the casing whoever typed it into the join form happened to use.
func TestTheBootstrapVariableIgnoresCaseAndSpace(t *testing.T) {
	srv, store := adminHandler(t, "  anna  ")
	h := srv.Handler()

	join(t, h, "Anna")
	srv.GrantBootstrapAdmin(t.Context())

	admins, _ := store.Players().Admins(t.Context())
	if len(admins) != 1 {
		t.Errorf("Admins() = %v, want Anna", admins)
	}
}

// Set before the person has joined is the ordinary case, not a failure: the
// variable goes into .env at setup time and the people arrive later.
func TestABootstrapNameNobodyHasIsNotFatal(t *testing.T) {
	srv, store := adminHandler(t, "Niemand")

	srv.GrantBootstrapAdmin(t.Context())

	admins, err := store.Players().Admins(t.Context())
	if err != nil {
		t.Fatalf("Admins(): %v", err)
	}
	if len(admins) != 0 {
		t.Errorf("Admins() = %v, want nobody", admins)
	}
}

func TestNoBootstrapVariableGrantsNothing(t *testing.T) {
	srv, store := adminHandler(t, "")
	h := srv.Handler()

	join(t, h, "Anna")
	srv.GrantBootstrapAdmin(t.Context())

	admins, _ := store.Players().Admins(t.Context())
	if len(admins) != 0 {
		t.Errorf("Admins() = %v, want nobody", admins)
	}
}

// Running it again is a no-op. It is called on every start, so it has to be.
func TestTheBootstrapIsIdempotent(t *testing.T) {
	srv, store := adminHandler(t, "Anna")
	h := srv.Handler()

	join(t, h, "Anna")
	srv.GrantBootstrapAdmin(t.Context())
	srv.GrantBootstrapAdmin(t.Context())

	admins, _ := store.Players().Admins(t.Context())
	if len(admins) != 1 {
		t.Errorf("Admins() = %v, want exactly one", admins)
	}
}

// The record of who may act for other people is itself not public.
func TestTheAdminPageIsBehindTheFlag(t *testing.T) {
	srv, store := adminHandler(t, "Anna")
	h := srv.Handler()

	annaCookie := sessionCookie(t, join(t, h, "Anna"))
	bodoCookie := sessionCookie(t, join(t, h, "Bodo"))
	srv.GrantBootstrapAdmin(t.Context())

	if got := getWith(t, h, "/admin", nil).Code; got != http.StatusUnauthorized {
		t.Errorf("a stranger gets %d, want %d", got, http.StatusUnauthorized)
	}
	if got := getWith(t, h, "/admin", bodoCookie).Code; got != http.StatusForbidden {
		t.Errorf("a plain player gets %d, want %d", got, http.StatusForbidden)
	}

	rec := getWith(t, h, "/admin", annaCookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("the admin gets %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Anna") {
		t.Errorf("the page does not list the admin: %s", rec.Body.String())
	}

	// Withdrawing it takes the page away again. That is the property the
	// kiosk's constant cookie does not have (#77).
	players, _ := store.Players().List(t.Context())
	for _, p := range players {
		if p.DisplayName == "Anna" {
			if err := store.Players().SetAdmin(t.Context(), p.ID, false); err != nil {
				t.Fatalf("SetAdmin(): %v", err)
			}
		}
	}
	if got := getWith(t, h, "/admin", annaCookie).Code; got != http.StatusForbidden {
		t.Errorf("after withdrawing the flag: %d, want %d", got, http.StatusForbidden)
	}
}

// A link everybody sees to a page only some may open is a link that mostly
// produces a refusal.
func TestOnlyAnAdminSeesTheLink(t *testing.T) {
	srv, _ := adminHandler(t, "Anna")
	h := srv.Handler()

	annaCookie := sessionCookie(t, join(t, h, "Anna"))
	bodoCookie := sessionCookie(t, join(t, h, "Bodo"))
	srv.GrantBootstrapAdmin(t.Context())

	if !strings.Contains(getWith(t, h, "/", annaCookie).Body.String(), `href="/admin"`) {
		t.Error("the admin is not offered the link")
	}
	for name, cookie := range map[string]*http.Cookie{"a plain player": bodoCookie, "a stranger": nil} {
		if strings.Contains(getWith(t, h, "/", cookie).Body.String(), `href="/admin"`) {
			t.Errorf("%s is offered the link", name)
		}
	}
}

// The boundary is on the page, because the person who wonders whether the
// laptop at the table may delete a result is standing in front of the
// application and not in front of the repository.
func TestTheAdminPageStatesTheBoundary(t *testing.T) {
	srv, _ := adminHandler(t, "Anna")
	h := srv.Handler()

	annaCookie := sessionCookie(t, join(t, h, "Anna"))
	srv.GrantBootstrapAdmin(t.Context())

	body := getWith(t, h, "/admin", annaCookie).Body.String()

	for _, want := range []string{
		"Gewertetes Ergebnis korrigieren oder entfernen",
		"Zwei Spieler zusammenführen",
		"Ein Turnier abrechnen",
		// The one nobody may do, ADR-0007: a PIN somebody else knows is not
		// a PIN.
		"Eine PIN für jemand anderen setzen",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the boundary table does not mention %q", want)
		}
	}
}

// A joined player is not an admin. The flag is granted, never inherited.
func TestJoiningDoesNotMakeAnAdmin(t *testing.T) {
	store := newMemStore()
	h := newHandlerWith(store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))

	join(t, h, "Anna")

	players, _ := store.Players().List(t.Context())
	if len(players) != 1 {
		t.Fatalf("List() = %d players, want 1", len(players))
	}
	if players[0].IsAdmin {
		t.Error("joining handed out the admin flag")
	}
}

// countedMatch puts one settled result in the store: Anna reports, Bodo
// confirms, both ratings move. The state a wrong result is actually in when
// somebody asks for it to be fixed.
func countedMatch(t *testing.T, h http.Handler, store *memStore, anna, bodo *http.Cookie) string {
	t.Helper()

	rec := recordMatch(t, h, anna, opponentID(t, store, "Bodo"), 3, 11, "11:9", "12:10")
	if rec.Code != http.StatusOK {
		t.Fatalf("recording: status %d: %s", rec.Code, rec.Body.String())
	}
	stored := store.matches.all()
	id := stored[len(stored)-1].ID.String()

	if rec := post(t, h, "/matches/"+id+"/confirm", bodo); rec.Code != http.StatusOK {
		t.Fatalf("confirming: status %d: %s", rec.Code, rec.Body.String())
	}
	return id
}

// twoPlayersAndAnAdmin is Anna with the flag, Bodo without, and a browser
// each.
func twoPlayersAndAnAdmin(t *testing.T) (http.Handler, *memStore, *http.Cookie, *http.Cookie) {
	t.Helper()

	srv, store := adminHandler(t, "Anna")
	h := srv.Handler()

	anna := sessionCookie(t, join(t, h, "Anna"))
	bodo := sessionCookie(t, join(t, h, "Bodo"))
	srv.GrantBootstrapAdmin(t.Context())

	return h, store, anna, bodo
}

// TestAnAdminTakesBackACountedResult is issue #105's first action and the
// phase's Definition of Done point 6: until this existed, the only way to fix
// a wrong result was SQL against the database the office plays on.
func TestAnAdminTakesBackACountedResult(t *testing.T) {
	h, store, anna, bodo := twoPlayersAndAnAdmin(t)

	id := countedMatch(t, h, store, anna, bodo)
	if ttrOfPlayer(t, store, "Anna") == domain.DefaultTTR {
		t.Fatal("the rating did not move, so there is nothing to take back")
	}

	// The page offers it, with the result readable enough to tell it from
	// the right one: a mistyped match is usually mistyped in the points.
	page := getWith(t, h, "/admin", anna).Body.String()
	for _, want := range []string{"Gewertete Ergebnisse", "11:9", "/admin/matches/" + id + "/remove"} {
		if !strings.Contains(page, want) {
			t.Errorf("the page does not offer %q: %s", want, page)
		}
	}

	rec := post(t, h, "/admin/matches/"+id+"/remove", anna)
	if rec.Code != http.StatusOK {
		t.Fatalf("removing: status %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Entfernt") {
		t.Errorf("the page does not say what happened: %s", rec.Body.String())
	}

	if got := ttrOfPlayer(t, store, "Anna"); got != domain.DefaultTTR {
		t.Errorf("Anna is on %d, want %d", got, domain.DefaultTTR)
	}
	if got := ttrOfPlayer(t, store, "Bodo"); got != domain.DefaultTTR {
		t.Errorf("Bodo is on %d, want %d", got, domain.DefaultTTR)
	}
	if len(store.matches.all()) != 0 {
		t.Errorf("the match survived the removal: %+v", store.matches.all())
	}
}

// TestTakingBackIsRefusedOnceSomethingElseHasCounted: the guard that stays
// after the clock is dropped. Restoring the ratings writes ttr_before back,
// which is only right while nothing has counted since — and the refusal has
// to say what to do rather than only that it will not.
func TestTakingBackIsRefusedOnceSomethingElseHasCounted(t *testing.T) {
	h, store, anna, bodo := twoPlayersAndAnAdmin(t)

	first := countedMatch(t, h, store, anna, bodo)
	countedMatch(t, h, store, anna, bodo)

	before := ttrOfPlayer(t, store, "Anna")

	rec := post(t, h, "/admin/matches/"+first+"/remove", anna)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("removing an overtaken result: status %d, want %d",
			rec.Code, http.StatusUnprocessableEntity)
	}
	if !strings.Contains(rec.Body.String(), "Erst das neuere entfernen") {
		t.Errorf("the refusal does not say what to do instead: %s", rec.Body.String())
	}
	if got := ttrOfPlayer(t, store, "Anna"); got != before {
		t.Errorf("the refused removal moved the rating anyway: %d, want %d", got, before)
	}
	if len(store.matches.all()) != 2 {
		t.Errorf("the refused removal deleted something: %+v", store.matches.all())
	}
}

// TestTakingBackACountedResultIsBehindTheFlag: nobody confirms this action,
// which is the price docs/adr/0008 pays for it — so the flag is the whole
// guard and it has to hold on the route, not only on the page that links it.
func TestTakingBackACountedResultIsBehindTheFlag(t *testing.T) {
	h, store, anna, bodo := twoPlayersAndAnAdmin(t)

	id := countedMatch(t, h, store, anna, bodo)
	before := ttrOfPlayer(t, store, "Anna")

	for name, tc := range map[string]struct {
		cookie *http.Cookie
		want   int
	}{
		"a stranger":     {nil, http.StatusUnauthorized},
		"a plain player": {bodo, http.StatusForbidden},
	} {
		if got := post(t, h, "/admin/matches/"+id+"/remove", tc.cookie).Code; got != tc.want {
			t.Errorf("%s gets %d, want %d", name, got, tc.want)
		}
	}

	if len(store.matches.all()) != 1 {
		t.Errorf("a refused caller removed the match anyway: %+v", store.matches.all())
	}
	if got := ttrOfPlayer(t, store, "Anna"); got != before {
		t.Errorf("a refused caller moved the rating: %d, want %d", got, before)
	}
	// And the one who may, still may — the guard is the flag and not the
	// route being unreachable.
	if got := post(t, h, "/admin/matches/"+id+"/remove", anna).Code; got != http.StatusOK {
		t.Errorf("the admin gets %d, want %d", got, http.StatusOK)
	}
}

// TestTheAdminRemovalHasNoClockOnIt is the one thing that separates this from
// the kiosk's undo (issue #49), and therefore the thing worth a test of its
// own: the kiosk may take back what it is still looking at, an admin may take
// back an evening later. Everything else about the two is the same act.
func TestTheAdminRemovalHasNoClockOnIt(t *testing.T) {
	h, store, anna, bodo := twoPlayersAndAnAdmin(t)

	id := countedMatch(t, h, store, anna, bodo)

	// Backdate the confirmation past the ten-minute window the kiosk undo
	// refuses beyond. Reaching into the store rather than waiting: the clock
	// is what is under test, and a test that sleeps ten minutes is not one.
	anHourAgo := time.Now().Add(-time.Hour)
	matchID, err := uuid.Parse(id)
	if err != nil {
		t.Fatalf("Parse(): %v", err)
	}
	if err := store.Matches().SetStatus(
		t.Context(), matchID, domain.MatchConfirmed, &anHourAgo,
	); err != nil {
		t.Fatalf("SetStatus(): %v", err)
	}

	rec := post(t, h, "/admin/matches/"+id+"/remove", anna)
	if rec.Code != http.StatusOK {
		t.Fatalf("removing an hour-old result: status %d, want %d: %s",
			rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := ttrOfPlayer(t, store, "Anna"); got != domain.DefaultTTR {
		t.Errorf("Anna is on %d, want %d", got, domain.DefaultTTR)
	}
}

// TestAnAdminRemovesAPlayerWhoNeverPlayed is the small action of issue #105:
// the joke entry and the duplicate created before anybody played.
func TestAnAdminRemovesAPlayerWhoNeverPlayed(t *testing.T) {
	h, store, anna, _ := twoPlayersAndAnAdmin(t)

	// A third browser, so the player being removed is the one holding the
	// session — their sign-in proof has to go with them.
	ella := sessionCookie(t, join(t, h, "Ella"))
	id := opponentID(t, store, "Ella")

	page := getWith(t, h, "/admin", anna).Body.String()
	for _, want := range []string{"Spieler ohne Ergebnis", "Ella", "/admin/players/" + id + "/remove"} {
		if !strings.Contains(page, want) {
			t.Errorf("the page does not offer %q", want)
		}
	}

	rec := post(t, h, "/admin/players/"+id+"/remove", anna)
	if rec.Code != http.StatusOK {
		t.Fatalf("removing: status %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Entfernt: Ella") {
		t.Errorf("the page does not name who went: %s", rec.Body.String())
	}

	players, err := store.Players().List(t.Context())
	if err != nil {
		t.Fatalf("List(): %v", err)
	}
	for _, p := range players {
		if p.DisplayName == "Ella" {
			t.Fatal("Ella is still in the roster")
		}
	}

	// The cascade: their cookie stops being recognised, because the identity
	// behind it went with the row. Anything else would leave a browser signed
	// in as somebody who no longer exists.
	if body := getWith(t, h, "/", ella).Body.String(); strings.Contains(body, "Hallo, Ella") {
		t.Errorf("the removed player's browser is still recognised: %s", body)
	}
}

// TestRemovingAPlayerWhoPlayedIsRefused: the schema is the authority here,
// and the refusal has to be a sentence rather than a 500.
func TestRemovingAPlayerWhoPlayedIsRefused(t *testing.T) {
	h, store, anna, bodo := twoPlayersAndAnAdmin(t)

	countedMatch(t, h, store, anna, bodo)
	id := opponentID(t, store, "Bodo")

	// Somebody with a result is not even offered.
	if page := getWith(t, h, "/admin", anna).Body.String(); strings.Contains(page, "/admin/players/"+id+"/remove") {
		t.Error("a player who has played is offered for removal")
	}

	// And asking anyway is refused rather than obeyed: the button is a
	// shortlist, the foreign keys are the rule.
	rec := post(t, h, "/admin/players/"+id+"/remove", anna)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status %d, want %d: %s", rec.Code, http.StatusUnprocessableEntity, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Wer einmal gespielt hat, bleibt") {
		t.Errorf("the refusal does not say why: %s", rec.Body.String())
	}
	if ttrOfPlayer(t, store, "Bodo") == 0 {
		t.Error("Bodo lost his rating to a refused removal")
	}
}

// TestAnAdminCannotRemoveThemselves: a legitimate delete by the schema's
// rules — an admin who never played — and a footgun, because the session
// would outlive the player it names.
func TestAnAdminCannotRemoveThemselves(t *testing.T) {
	h, store, anna, _ := twoPlayersAndAnAdmin(t)
	id := opponentID(t, store, "Anna")

	if page := getWith(t, h, "/admin", anna).Body.String(); strings.Contains(page, "/admin/players/"+id+"/remove") {
		t.Error("the admin's own row carries a remove button")
	}

	rec := post(t, h, "/admin/players/"+id+"/remove", anna)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	players, _ := store.Players().List(t.Context())
	if len(players) != 2 {
		t.Errorf("%d players left, want 2", len(players))
	}
}

// TestRemovingAPlayerIsBehindTheFlag: nobody confirms this either.
func TestRemovingAPlayerIsBehindTheFlag(t *testing.T) {
	h, store, anna, bodo := twoPlayersAndAnAdmin(t)
	join(t, h, "Ella")
	id := opponentID(t, store, "Ella")

	for name, tc := range map[string]struct {
		cookie *http.Cookie
		want   int
	}{
		"a stranger":     {nil, http.StatusUnauthorized},
		"a plain player": {bodo, http.StatusForbidden},
	} {
		if got := post(t, h, "/admin/players/"+id+"/remove", tc.cookie).Code; got != tc.want {
			t.Errorf("%s gets %d, want %d", name, got, tc.want)
		}
	}

	players, _ := store.Players().List(t.Context())
	if len(players) != 3 {
		t.Errorf("a refused caller removed somebody: %d players left, want 3", len(players))
	}
	if got := post(t, h, "/admin/players/"+id+"/remove", anna).Code; got != http.StatusOK {
		t.Errorf("the admin gets %d, want 200", got)
	}
}
