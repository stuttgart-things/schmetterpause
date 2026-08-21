package server_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stuttgart-things/schmetterpause/internal/auth"
	"github.com/stuttgart-things/schmetterpause/internal/config"
	"github.com/stuttgart-things/schmetterpause/internal/repository"
	"github.com/stuttgart-things/schmetterpause/internal/server"
)

// fakeStore erfuellt repository.Store, ohne eine Datenbank zu brauchen.
// Genau dafuer existieren die Repository-Interfaces (Invariante 5): Handler
// sind ohne Postgres testbar. Nicht ueberschriebene Methoden sind nil und
// wuerden beim Aufruf panicken — das ist gewollt, es faellt sofort auf.
type fakeStore struct {
	repository.Store
	players repository.PlayerRepository
	pingErr error
}

func (f fakeStore) Ping(context.Context) error           { return f.pingErr }
func (f fakeStore) Players() repository.PlayerRepository { return f.players }

type fakePlayers struct {
	repository.PlayerRepository
	count int
	err   error
}

func (f fakePlayers) Count(context.Context) (int, error) { return f.count, f.err }

func newHandler(store repository.Store) http.Handler {
	cfg := config.Config{
		HTTPAddr:         ":0",
		DatabaseURL:      "postgres://test",
		ShutdownTimeout:  time.Second,
		ReadinessTimeout: time.Second,
	}
	log := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	return server.New(cfg, store, log, auth.Anonymous{}, "test").Handler()
}

func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestHealthzIgnoresDatabase(t *testing.T) {
	// Liveness darf nicht von der Datenbank abhaengen: ein DB-Ausfall soll
	// keinen Container-Neustart ausloesen.
	h := newHandler(fakeStore{pingErr: errors.New("db weg")})

	rec := get(t, h, "/healthz")

	if rec.Code != http.StatusOK {
		t.Fatalf("Status = %d, erwartet 200", rec.Code)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != "ok" {
		t.Errorf("Body = %q, erwartet \"ok\"", got)
	}
}

func TestReadyz(t *testing.T) {
	tests := []struct {
		name    string
		pingErr error
		want    int
	}{
		{"datenbank erreichbar", nil, http.StatusOK},
		{"datenbank weg", errors.New("db weg"), http.StatusServiceUnavailable},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := get(t, newHandler(fakeStore{pingErr: tc.pingErr}), "/readyz")

			if rec.Code != tc.want {
				t.Errorf("Status = %d, erwartet %d", rec.Code, tc.want)
			}
		})
	}
}

func TestIndexRendersLayout(t *testing.T) {
	h := newHandler(fakeStore{players: fakePlayers{}})

	rec := get(t, h, "/")

	if rec.Code != http.StatusOK {
		t.Fatalf("Status = %d, erwartet 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"Schmetterpause", "/static/js/htmx.min.js", `hx-get="/fragments/status"`} {
		if !strings.Contains(body, want) {
			t.Errorf("Seite enthaelt %q nicht", want)
		}
	}
}

func TestStatusFragmentZeigtSpielerzahl(t *testing.T) {
	h := newHandler(fakeStore{players: fakePlayers{count: 7}})

	rec := get(t, h, "/fragments/status")

	if rec.Code != http.StatusOK {
		t.Fatalf("Status = %d, erwartet 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, ">7<") {
		t.Errorf("Fragment nennt die Spielerzahl nicht: %s", body)
	}
	if !strings.Contains(body, "erreichbar") {
		t.Errorf("Fragment nennt den Datenbankstatus nicht: %s", body)
	}
}

func TestStatusFragmentOhneDatenbank(t *testing.T) {
	// Ohne Datenbank bleibt die Seite bedienbar und sagt, was fehlt.
	h := newHandler(fakeStore{pingErr: errors.New("db weg")})

	rec := get(t, h, "/fragments/status")

	if rec.Code != http.StatusOK {
		t.Fatalf("Status = %d, erwartet 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "nicht erreichbar") {
		t.Errorf("Fragment meldet den Ausfall nicht: %s", rec.Body.String())
	}
}

func TestStaticAssetsSindEingebettet(t *testing.T) {
	// Faengt genau den Fehler, den der Verify-Schritt spaeter im Image sucht:
	// Assets, die im Container nicht mitkommen.
	h := newHandler(fakeStore{})

	rec := get(t, h, "/static/js/htmx.min.js")

	if rec.Code != http.StatusOK {
		t.Fatalf("Status = %d, erwartet 200", rec.Code)
	}
	if rec.Body.Len() == 0 {
		t.Error("htmx.min.js ist leer")
	}
}
