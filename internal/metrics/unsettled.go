package metrics

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
)

// UnsettledSource answers how many results are waiting, grouped by status and
// origin. The server hands in its repository call; this package never sees a
// store (invariant 5).
type UnsettledSource func(context.Context) ([]domain.UnsettledGroup, error)

// Option adds something to the registry New builds.
type Option func(*prometheus.Registry)

// WithUnsettled measures results still waiting on somebody (issue #251): the
// one signal that says the loop of enter, confirm, rate has stopped while
// every technical one stays green.
//
// Asked at scrape time rather than kept up to date by the handlers, because a
// confirmation is not the only way a result stops waiting — an admin removal,
// a correction, a restore all change the answer, and a gauge the handlers had
// to remember to move would be wrong after the first one they forgot.
//
// timeout bounds the query. A scrape waits for Collect, so a database that
// hangs would otherwise hang /metrics with it and take the request and
// runtime series down as well.
func WithUnsettled(source UnsettledSource, timeout time.Duration) Option {
	return func(r *prometheus.Registry) {
		r.MustRegister(&unsettledCollector{source: source, timeout: timeout, now: time.Now})
	}
}

var (
	unsettledOpenDesc = prometheus.NewDesc(
		"schmetterpause_matches_open",
		"Results still waiting on a player, by status and origin.",
		[]string{"status", "entered_via"}, nil)
	unsettledOldestDesc = prometheus.NewDesc(
		"schmetterpause_matches_oldest_open_seconds",
		"How long the longest-waiting result has been waiting, by status and origin. 0 when none waits.",
		[]string{"status", "entered_via"}, nil)
	unsettledUpDesc = prometheus.NewDesc(
		"schmetterpause_matches_open_query_up",
		"Whether the query behind schmetterpause_matches_open answered on this scrape.",
		nil, nil)
)

// The label values, fixed. Every combination is exported on every successful
// scrape, zero included, so a series does not vanish when the last result of
// its kind is confirmed — an absent series and a zero read differently in
// PromQL, and a rule written against one misses the other.
var (
	unsettledStatuses = []domain.MatchStatus{domain.MatchPending, domain.MatchDisputed}
	unsettledOrigins  = []domain.EnteredVia{
		domain.EnteredViaPlayer, domain.EnteredViaKiosk, domain.EnteredViaScoreboard,
	}
)

type unsettledCollector struct {
	source  UnsettledSource
	timeout time.Duration
	now     func() time.Time
}

func (c *unsettledCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- unsettledOpenDesc
	ch <- unsettledOldestDesc
	ch <- unsettledUpDesc
}

func (c *unsettledCollector) Collect(ch chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	groups, err := c.source(ctx)
	if err != nil {
		// Nothing but the failure. Zeros here would read as "nothing waits"
		// and silently resolve the very alert this exists for; missing series
		// plus up = 0 is a state a rule can name.
		ch <- prometheus.MustNewConstMetric(unsettledUpDesc, prometheus.GaugeValue, 0)
		return
	}
	ch <- prometheus.MustNewConstMetric(unsettledUpDesc, prometheus.GaugeValue, 1)

	type key struct {
		status domain.MatchStatus
		via    domain.EnteredVia
	}
	byKey := make(map[key]domain.UnsettledGroup, len(groups))
	for _, g := range groups {
		byKey[key{g.Status, g.EnteredVia}] = g
	}

	now := c.now()
	for _, status := range unsettledStatuses {
		for _, via := range unsettledOrigins {
			g := byKey[key{status, via}]
			var oldest float64
			if g.Count > 0 {
				oldest = now.Sub(g.Oldest).Seconds()
			}
			ch <- prometheus.MustNewConstMetric(unsettledOpenDesc, prometheus.GaugeValue,
				float64(g.Count), string(status), string(via))
			ch <- prometheus.MustNewConstMetric(unsettledOldestDesc, prometheus.GaugeValue,
				oldest, string(status), string(via))
		}
	}
}
