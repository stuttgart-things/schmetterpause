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

// fakeStore satisfies repository.Store without needing a database. This is
// what the repository interfaces are for (invariant 5): handlers are testable
// without Postgres. Methods that are not overridden stay nil and would panic
// when called — which is intended, it surfaces immediately.
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
	// Liveness must not depend on the database: an outage should not trigger
	// a container restart.
	h := newHandler(fakeStore{pingErr: errors.New("database gone")})

	rec := get(t, h, "/healthz")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != "ok" {
		t.Errorf("body = %q, want \"ok\"", got)
	}
}

func TestReadyz(t *testing.T) {
	tests := []struct {
		name    string
		pingErr error
		want    int
	}{
		{"database reachable", nil, http.StatusOK},
		{"database gone", errors.New("database gone"), http.StatusServiceUnavailable},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := get(t, newHandler(fakeStore{pingErr: tc.pingErr}), "/readyz")

			if rec.Code != tc.want {
				t.Errorf("status = %d, want %d", rec.Code, tc.want)
			}
		})
	}
}

func TestIndexRendersLayout(t *testing.T) {
	h := newHandler(fakeStore{players: fakePlayers{}})

	rec := get(t, h, "/")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"Schmetterpause", "/static/js/htmx.min.js", `hx-get="/fragments/status"`} {
		if !strings.Contains(body, want) {
			t.Errorf("page does not contain %q", want)
		}
	}
}

func TestStatusFragmentShowsPlayerCount(t *testing.T) {
	h := newHandler(fakeStore{players: fakePlayers{count: 7}})

	rec := get(t, h, "/fragments/status")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, ">7<") {
		t.Errorf("fragment does not state the player count: %s", body)
	}
	if !strings.Contains(body, "erreichbar") {
		t.Errorf("fragment does not state the database status: %s", body)
	}
}

func TestStatusFragmentWithoutDatabase(t *testing.T) {
	// Without a database the page stays usable and says what is missing.
	h := newHandler(fakeStore{pingErr: errors.New("database gone")})

	rec := get(t, h, "/fragments/status")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "nicht erreichbar") {
		t.Errorf("fragment does not report the outage: %s", rec.Body.String())
	}
}

func TestStaticAssetsAreEmbedded(t *testing.T) {
	// Catches exactly the fault the verify step later looks for in the image:
	// assets that did not make it into the container.
	h := newHandler(fakeStore{})

	rec := get(t, h, "/static/js/htmx.min.js")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Body.Len() == 0 {
		t.Error("htmx.min.js is empty")
	}
}
