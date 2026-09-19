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

// The unsettled-results metric reads the store the page reads (issue #251):
// a result recorded and not yet confirmed shows up on the next scrape, under
// the origin it came from.
func TestAnUnconfirmedResultIsCountedOnTheNextScrape(t *testing.T) {
	store := newMemStore()
	cfg := testConfig()
	cfg.SessionKey = testSessionKey
	cfg.MetricsAddr = ":9090"
	srv := server.New(cfg, store, slog.New(slog.DiscardHandler),
		auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false),
		server.Build{Version: "v9.9.9"})
	h := srv.Handler()

	anna := sessionCookie(t, join(t, h, "Anna"))
	join(t, h, "Bodo")
	if rec := recordMatch(t, h, anna, opponentID(t, store, "Bodo"), 3, 11, "11:9", "12:10"); rec.Code != http.StatusOK {
		t.Fatalf("recording: status %d: %s", rec.Code, rec.Body.String())
	}

	rec := httptest.NewRecorder()
	srv.MetricsHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := rec.Body.String()

	for _, want := range []string{
		`schmetterpause_matches_open{entered_via="player",status="pending"} 1`,
		`schmetterpause_matches_open{entered_via="scoreboard",status="pending"} 0`,
		`schmetterpause_matches_open_query_up 1`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the metrics lack %s: %s", want, body)
		}
	}
}
