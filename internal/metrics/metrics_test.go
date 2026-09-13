package metrics_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stuttgart-things/schmetterpause/internal/metrics"
)

func scrape(t *testing.T, m *metrics.Metrics) string {
	t.Helper()
	rec := httptest.NewRecorder()
	m.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /metrics = %d, want %d", rec.Code, http.StatusOK)
	}
	return rec.Body.String()
}

func TestTheBuildIsALabel(t *testing.T) {
	body := scrape(t, metrics.New("v1.2.3"))

	if !strings.Contains(body, `schmetterpause_build_info{version="v1.2.3"} 1`) {
		t.Errorf("build_info does not name the version: %s", body)
	}
	// The runtime comes along, so a leak shows before the pod is killed for it.
	if !strings.Contains(body, "go_goroutines") {
		t.Errorf("the Go collector is missing: %s", body)
	}
}

// The listener is a second port, not a second way into the application.
func TestTheHandlerServesMetricsAndNothingElse(t *testing.T) {
	m := metrics.New("v1.2.3")

	for _, path := range []string{"/", "/healthz", "/players", "/metrics/extra"} {
		rec := httptest.NewRecorder()
		m.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("GET %s on the metrics listener = %d, want %d", path, rec.Code, http.StatusNotFound)
		}
	}
}

// By pattern, not by path: a path carries a player's id, and a series per
// player is a series count nobody chose.
func TestRequestsAreCountedByPattern(t *testing.T) {
	m := metrics.New("v1.2.3")

	mux := http.NewServeMux()
	mux.HandleFunc("GET /players/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
	routeOf := func(r *http.Request) string {
		_, pattern := mux.Handler(r)
		return pattern
	}
	h := m.Middleware(routeOf)(mux)

	for _, path := range []string{"/players/anna", "/players/bodo", "/nowhere", "/somewhere-else"} {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, path, nil))
	}

	body := scrape(t, m)
	for _, want := range []string{
		`schmetterpause_http_requests_total{code="418",method="GET",route="GET /players/{id}"} 2`,
		`schmetterpause_http_requests_total{code="404",method="GET",route="unmatched"} 2`,
		`schmetterpause_http_request_duration_seconds_count{route="GET /players/{id}"} 2`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %s in: %s", want, body)
		}
	}
	for _, leaked := range []string{"anna", "bodo", "nowhere"} {
		if strings.Contains(body, leaked) {
			t.Errorf("a path segment %q became a label: %s", leaked, body)
		}
	}
}

// A method nobody defined shares a label too, or it is another way to grow
// the series count from outside.
func TestAnInventedMethodIsOther(t *testing.T) {
	m := metrics.New("v1.2.3")
	h := m.Middleware(func(*http.Request) string { return "" })(http.NotFoundHandler())

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("BREW", "/", nil))

	if body := scrape(t, m); !strings.Contains(body, `method="other"`) || strings.Contains(body, "BREW") {
		t.Errorf("an invented method was not folded into other: %s", body)
	}
}
