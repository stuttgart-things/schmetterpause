package server_test

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/stuttgart-things/schmetterpause/internal/auth"
)

// Without SP_ZAEHLWERK_URL the start page carries neither the running score
// nor the script that fills it (#188) — off means absent, the kiosk's
// posture, not an element that waits for a stream that does not exist.
func TestStartPageHasNoLiveScoreWithoutAZaehlwerk(t *testing.T) {
	body := get(t, newHandler(newMemStore()), "/").Body.String()

	for _, absent := range []string{`id="live"`, "/static/js/live.js", "live-ratings"} {
		if strings.Contains(body, absent) {
			t.Errorf("the start page carries %q without SP_ZAEHLWERK_URL", absent)
		}
	}
}

// With it, the page points the element at the Zählwerk's table stream and
// hands the script every player's stored rating by id, so the script has
// nothing to look up and nothing to compute.
func TestStartPageCarriesTheLiveScoreAndTheRatings(t *testing.T) {
	store := newMemStore()
	ids := seedPlayers(t, store, "Anna", "Ben")

	cfg := testConfig()
	cfg.ZaehlwerkURL = "https://zaehlwerk.example.org"
	h := newHandlerConfig(cfg, store, auth.Anonymous{})

	body := get(t, h, "/").Body.String()

	if !strings.Contains(body, `data-stream="https://zaehlwerk.example.org/live/stream"`) {
		t.Error("the element does not point at the table stream")
	}
	if !strings.Contains(body, `<script src="/static/js/live.js" defer>`) {
		t.Error("the page does not load live.js")
	}
	// Hidden until an event says a match is running: a page opened between
	// matches must look like one without the element.
	if !regexp.MustCompile(`<section[^>]*id="live"[^>]*\shidden`).MatchString(body) {
		t.Error("the element is not rendered hidden")
	}

	m := regexp.MustCompile(`(?s)<script id="live-ratings" type="application/json">(.*?)</script>`).FindStringSubmatch(body)
	if m == nil {
		t.Fatal("no live-ratings script in the page")
	}
	var ratings map[string]int
	if err := json.Unmarshal([]byte(m[1]), &ratings); err != nil {
		t.Fatalf("live-ratings is not a JSON object of ratings: %v", err)
	}
	for _, id := range ids {
		if ratings[id] == 0 {
			t.Errorf("no rating for player %s in %v", id, ratings)
		}
	}
}
