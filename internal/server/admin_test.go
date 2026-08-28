package server_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stuttgart-things/schmetterpause/internal/auth"
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
		auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false), "test")
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
