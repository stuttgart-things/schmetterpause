// Package metrics is what the application measures about itself, for a
// scraper that reaches the pod directly (issue #175, level 2).
//
// It is served on a listener of its own and never on the application's port.
// The HTTPRoute in front of the application forwards everything under "/"
// (kcl/httproute.k), so a /metrics path there would be public the moment it
// existed. A port the Service does not name is one no gateway can reach.
//
// A registry of its own rather than the global one, so nothing a dependency
// registers on import turns up on the page by accident, and a test can build
// as many as it likes.
package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// unmatched is the route label of a request no pattern took. One value for
// every unknown path, so somebody probing random URLs cannot grow the series
// count with them.
const unmatched = "unmatched"

// Metrics holds the registry and the instruments on it.
type Metrics struct {
	handler  http.Handler
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

// New builds the registry for a binary of the given version, with whatever
// the options add to it.
func New(version string, opts ...Option) *Metrics {
	registry := prometheus.NewRegistry()

	// Which build is answering, as a metric: the same fact /version gives a
	// probe, in the shape an operator expects (issue #229).
	build := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "schmetterpause_build_info",
		Help: "The running build, as a label. Always 1.",
	}, []string{"version"})
	build.WithLabelValues(version).Set(1)

	requests := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "schmetterpause_http_requests_total",
		Help: "HTTP requests by method, route pattern and status code.",
	}, []string{"method", "route", "code"})

	duration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "schmetterpause_http_request_duration_seconds",
		Help:    "Time to answer an HTTP request, by route pattern.",
		Buckets: prometheus.DefBuckets,
	}, []string{"route"})

	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		build, requests, duration,
	)
	for _, opt := range opts {
		opt(registry)
	}

	// /metrics and nothing else, so the second port is not a second way into
	// anything the first one serves.
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))

	return &Metrics{handler: mux, requests: requests, duration: duration}
}

// Handler is what the metrics listener serves.
func (m *Metrics) Handler() http.Handler { return m.handler }

// Middleware counts and times every request under the route pattern that
// routeOf names for it.
//
// The pattern — "GET /players/{id}" — rather than the path, because the path
// carries ids and would make a series per player. routeOf answering "" means
// no pattern matched, and all of those share one label.
func (m *Metrics) Middleware(routeOf func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			route := routeOf(r)
			if route == "" {
				route = unmatched
			}

			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w}
			next.ServeHTTP(rec, r)

			m.requests.WithLabelValues(method(r.Method), route, strconv.Itoa(rec.code())).Inc()
			m.duration.WithLabelValues(route).Observe(time.Since(start).Seconds())
		})
	}
}

// method bounds the method label to the ones HTTP defines. Anything else is
// "other", for the same reason unknown paths share a label.
func method(m string) string {
	switch m {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch,
		http.MethodDelete, http.MethodOptions, http.MethodConnect, http.MethodTrace:
		return m
	}
	return "other"
}

// statusRecorder remembers the status code a handler wrote.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (w *statusRecorder) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusRecorder) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

// code is what was written, and 200 for a handler that wrote nothing — which
// is what net/http sends in that case.
func (w *statusRecorder) code() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}
