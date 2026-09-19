package metrics_test

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
	"github.com/stuttgart-things/schmetterpause/internal/metrics"
)

// value reads one sample off a scrape, failing when the series is absent.
func value(t *testing.T, body, series string) float64 {
	t.Helper()
	for _, line := range strings.Split(body, "\n") {
		if rest, ok := strings.CutPrefix(line, series+" "); ok {
			v, err := strconv.ParseFloat(rest, 64)
			if err != nil {
				t.Fatalf("%s: %v", series, err)
			}
			return v
		}
	}
	t.Fatalf("no series %s in: %s", series, body)
	return 0
}

func fixed(groups []domain.UnsettledGroup, err error) metrics.UnsettledSource {
	return func(context.Context) ([]domain.UnsettledGroup, error) { return groups, err }
}

// Every combination is exported, zero included. A series that vanishes when
// the last result of its kind is confirmed reads differently in PromQL from
// one that says 0, and a rule written against one misses the other.
func TestEveryStatusAndOriginIsExportedEvenAtZero(t *testing.T) {
	body := scrape(t, metrics.New("v1", metrics.WithUnsettled(fixed(nil, nil), time.Second)))

	if got := value(t, body, "schmetterpause_matches_open_query_up"); got != 1 {
		t.Errorf("query_up = %v, want 1", got)
	}
	for _, status := range []string{"pending", "disputed"} {
		for _, via := range []string{"player", "kiosk", "scoreboard"} {
			labels := `{entered_via="` + via + `",status="` + status + `"}`
			if got := value(t, body, "schmetterpause_matches_open"+labels); got != 0 {
				t.Errorf("open%s = %v, want 0", labels, got)
			}
			if got := value(t, body, "schmetterpause_matches_oldest_open_seconds"+labels); got != 0 {
				t.Errorf("oldest%s = %v, want 0", labels, got)
			}
		}
	}
}

// The count and the age come from the group they belong to, and only from
// it: the Zählwerk's evening must not age the players' own results.
func TestCountAndAgeArePerGroup(t *testing.T) {
	now := time.Now()
	source := fixed([]domain.UnsettledGroup{
		{Status: domain.MatchPending, EnteredVia: domain.EnteredViaScoreboard, Count: 4, Oldest: now.Add(-3 * time.Hour)},
		{Status: domain.MatchDisputed, EnteredVia: domain.EnteredViaPlayer, Count: 1, Oldest: now.Add(-30 * time.Hour)},
	}, nil)
	body := scrape(t, metrics.New("v1", metrics.WithUnsettled(source, time.Second)))

	if got := value(t, body, `schmetterpause_matches_open{entered_via="scoreboard",status="pending"}`); got != 4 {
		t.Errorf("pending from the Zählwerk = %v, want 4", got)
	}
	age := value(t, body, `schmetterpause_matches_oldest_open_seconds{entered_via="scoreboard",status="pending"}`)
	if age < 3*3600 || age > 3*3600+60 {
		t.Errorf("oldest pending from the Zählwerk = %vs, want about three hours", age)
	}
	age = value(t, body, `schmetterpause_matches_oldest_open_seconds{entered_via="player",status="disputed"}`)
	if age < 30*3600 || age > 30*3600+60 {
		t.Errorf("oldest disputed = %vs, want about thirty hours", age)
	}
	if got := value(t, body, `schmetterpause_matches_oldest_open_seconds{entered_via="player",status="pending"}`); got != 0 {
		t.Errorf("a group with nothing waiting has an age: %v", got)
	}
}

// A failed query exports its failure and nothing else. Zeros would read as
// "nothing waits" and resolve the alert this exists for; and the rest of the
// scrape — requests, runtime, build — must survive it.
func TestAFailedQueryIsReportedNotZeroed(t *testing.T) {
	body := scrape(t, metrics.New("v1",
		metrics.WithUnsettled(fixed(nil, errors.New("connection refused")), time.Second)))

	if got := value(t, body, "schmetterpause_matches_open_query_up"); got != 0 {
		t.Errorf("query_up = %v, want 0", got)
	}
	if strings.Contains(body, "schmetterpause_matches_open{") {
		t.Errorf("a failed query still exported counts: %s", body)
	}
	if !strings.Contains(body, `schmetterpause_build_info{version="v1"} 1`) {
		t.Errorf("the failure took the rest of the scrape with it: %s", body)
	}
}

// A database that hangs costs this series, not the scrape.
func TestAHangingQueryIsCutOff(t *testing.T) {
	hang := func(ctx context.Context) ([]domain.UnsettledGroup, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}

	start := time.Now()
	body := scrape(t, metrics.New("v1", metrics.WithUnsettled(hang, 50*time.Millisecond)))
	if took := time.Since(start); took > 2*time.Second {
		t.Errorf("the scrape waited %v for a hanging query", took)
	}
	if got := value(t, body, "schmetterpause_matches_open_query_up"); got != 0 {
		t.Errorf("query_up = %v, want 0", got)
	}
}
