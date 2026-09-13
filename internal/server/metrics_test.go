package server_test

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stuttgart-things/schmetterpause/internal/auth"
	"github.com/stuttgart-things/schmetterpause/internal/config"
	"github.com/stuttgart-things/schmetterpause/internal/server"
)

func newMeasuredServer(t *testing.T, metricsAddr string) *server.Server {
	t.Helper()
	store := newMemStore()
	cfg := config.Config{HTTPAddr: ":8080", MetricsAddr: metricsAddr}
	return server.New(cfg, store, slog.New(slog.DiscardHandler),
		auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false),
		server.Build{Version: "v9.9.9"})
}

// #194's Definition of Done, point 2, from the application's side: the port
// players reach has no /metrics, whatever the metrics listener is doing.
func TestMetricsAreNotOnTheApplicationPort(t *testing.T) {
	srv := newMeasuredServer(t, ":9090")

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	if rec.Code == http.StatusOK || strings.Contains(rec.Body.String(), "schmetterpause_build_info") {
		t.Errorf("GET /metrics on the application port = %d: %s", rec.Code, rec.Body.String())
	}
}

// Off means off: no listener to start, nothing wrapped around every request.
func TestMetricsAreOffWithoutAnAddress(t *testing.T) {
	if h := newMeasuredServer(t, "").MetricsHandler(); h != nil {
		t.Error("a server without SP_METRICS_ADDR has a metrics handler")
	}
}

// Both muxes name their routes: the probes on the outer one, the pages behind
// the auth middleware on the inner one. Without asking the inner mux every
// page would be counted as "/".
func TestRequestsAreCountedUnderTheirPattern(t *testing.T) {
	srv := newMeasuredServer(t, ":9090")

	for _, path := range []string{"/version", "/standings", "/no-such-page"} {
		srv.Handler().ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, path, nil))
	}

	rec := httptest.NewRecorder()
	srv.MetricsHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := rec.Body.String()

	for _, want := range []string{
		`schmetterpause_build_info{version="v9.9.9"} 1`,
		`route="GET /version"`,
		`route="GET /standings"`,
		`route="unmatched"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the metrics lack %s: %s", want, body)
		}
	}
	if strings.Contains(body, "no-such-page") {
		t.Error("an unknown path became a label")
	}
}
