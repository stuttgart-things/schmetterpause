package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stuttgart-things/schmetterpause/internal/auth"
	"github.com/stuttgart-things/schmetterpause/internal/domain"
)

const testStatisticsToken = "radio-reads-counts"

func statisticsHandler(t *testing.T, statsToken string) (http.Handler, *memStore) {
	t.Helper()

	store := newMemStore()
	cfg := testConfig()
	cfg.ScoreboardToken = testScoreboardToken
	cfg.StatisticsToken = statsToken
	cfg.SessionKey = testSessionKey
	h := newHandlerConfig(cfg, store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))
	return h, store
}

func getStatistics(t *testing.T, h http.Handler, token, query string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/api/statistics"+query, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// storeResult writes a match straight into the fake, played at the given time.
func storeResult(t *testing.T, store *memStore, home, away domain.Player, status domain.MatchStatus, at time.Time, sets ...[2]int) {
	t.Helper()

	m := domain.Match{HomeID: home.ID, AwayID: away.ID, BestOf: 3, PointsToWin: 11, Status: status, PlayedAt: at}
	for i, s := range sets {
		m.Sets = append(m.Sets, domain.MatchSet{SetNo: i + 1, HomePoints: s[0], AwayPoints: s[1]})
	}
	if _, err := store.Matches().Create(t.Context(), m); err != nil {
		t.Fatalf("storing a match: %v", err)
	}
}

type statisticsBody struct {
	From        *time.Time `json:"from"`
	Truncated   bool       `json:"truncated"`
	Matches     int        `json:"matches"`
	Sets        int        `json:"sets"`
	Points      int        `json:"points"`
	Deuce       int        `json:"deuce"`
	Whitewashes int        `json:"whitewashes"`
	LongestSet  *struct {
		Home int `json:"home"`
		Away int `json:"away"`
	} `json:"longest_set"`
	MostWins []struct {
		DisplayName string `json:"display_name"`
		Won         int    `json:"won"`
		Lost        int    `json:"lost"`
	} `json:"most_wins"`
}

// Unset means absent, as for every other token on this surface.
func TestStatisticsRouteDoesNotExistWithoutItsToken(t *testing.T) {
	h, _ := statisticsHandler(t, "")
	if rec := getStatistics(t, h, testScoreboardToken, ""); rec.Code != http.StatusNotFound {
		t.Errorf("GET /api/statistics without STATISTICS_TOKEN: got %d, want 404", rec.Code)
	}
}

// The scoreboard token may enter results; it does not double as the reader's.
func TestStatisticsRefusesOtherTokens(t *testing.T) {
	h, _ := statisticsHandler(t, testStatisticsToken)
	for _, token := range []string{"", "wrong", testScoreboardToken} {
		if rec := getStatistics(t, h, token, ""); rec.Code != http.StatusUnauthorized {
			t.Errorf("token %q: got %d, want 401", token, rec.Code)
		}
	}
	// And the reader's token opens nothing else.
	req := httptest.NewRequest(http.MethodGet, "/api/players", nil)
	req.Header.Set("Authorization", "Bearer "+testStatisticsToken)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("GET /api/players with the statistics token: got %d, want 401", rec.Code)
	}
}

func TestStatisticsCountsConfirmedMatchesInTheWindow(t *testing.T) {
	h, store := statisticsHandler(t, testStatisticsToken)
	anna, bea, cem := threePlayers(t, store)
	now := time.Now()

	// Oldest first: the fake's Recent goes by insertion order.
	storeResult(t, store, anna, bea, domain.MatchConfirmed, now.AddDate(0, 0, -30), [2]int{11, 0}, [2]int{11, 0})
	storeResult(t, store, anna, bea, domain.MatchConfirmed, now.Add(-48*time.Hour), [2]int{11, 9}, [2]int{14, 12})
	storeResult(t, store, cem, anna, domain.MatchConfirmed, now.Add(-24*time.Hour), [2]int{11, 5}, [2]int{8, 11}, [2]int{11, 7})
	storeResult(t, store, bea, cem, domain.MatchPending, now.Add(-time.Hour), [2]int{11, 1}, [2]int{11, 1})

	rec := getStatistics(t, h, testStatisticsToken, "?days=7")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/statistics?days=7: got %d (%s)", rec.Code, rec.Body)
	}
	var got statisticsBody
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decoding: %v", err)
	}

	if got.From == nil {
		t.Error("from is missing for a window")
	}
	if got.Matches != 2 || got.Sets != 5 || got.Deuce != 1 || got.Whitewashes != 1 || got.Truncated {
		t.Errorf("totals: got %+v, want 2 matches, 5 sets, 1 deuce, 1 whitewash — the month-old and the pending match left out", got)
	}
	if wantPoints := 20 + 26 + 16 + 19 + 18; got.Points != wantPoints {
		t.Errorf("points: got %d, want %d", got.Points, wantPoints)
	}
	if got.LongestSet == nil || got.LongestSet.Home != 14 || got.LongestSet.Away != 12 {
		t.Errorf("longest set: got %+v, want 14:12", got.LongestSet)
	}
	if len(got.MostWins) != 2 || got.MostWins[0].DisplayName != "Cem" || got.MostWins[0].Won != 1 || got.MostWins[0].Lost != 0 ||
		got.MostWins[1].DisplayName != "Anna" || got.MostWins[1].Lost != 1 {
		t.Errorf("most wins: got %+v, want Cem 1:0 then Anna 1:1 — equal wins, fewer losses first; Bea without a win absent", got.MostWins)
	}

	// Without days: everything on record, the month-old whitewash included.
	rec = getStatistics(t, h, testStatisticsToken, "")
	var all statisticsBody
	if err := json.Unmarshal(rec.Body.Bytes(), &all); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if all.From != nil || all.Matches != 3 || all.Whitewashes != 2 {
		t.Errorf("everything: got %+v, want no from, 3 matches, 2 whitewashes", all)
	}
}

func TestStatisticsWithNothingPlayed(t *testing.T) {
	h, _ := statisticsHandler(t, testStatisticsToken)
	rec := getStatistics(t, h, testStatisticsToken, "?days=1")
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d (%s)", rec.Code, rec.Body)
	}
	var raw map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if _, ok := raw["longest_set"]; ok {
		t.Error("longest_set is present before any set was played")
	}
	if mw, ok := raw["most_wins"].([]any); !ok || len(mw) != 0 {
		t.Errorf("most_wins: got %v, want an empty list, not null", raw["most_wins"])
	}
}

func TestStatisticsRefusesABadWindow(t *testing.T) {
	h, _ := statisticsHandler(t, testStatisticsToken)
	for _, q := range []string{"?days=0", "?days=-1", "?days=367", "?days=week", "?days=1.5"} {
		if rec := getStatistics(t, h, testStatisticsToken, q); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: got %d, want 400", q, rec.Code)
		}
	}
}
