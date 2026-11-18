package server

import (
	"crypto/subtle"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/stats"
)

// GET /api/statistics: what the statistics page counts, as JSON, for a reader
// that is not a browser (docs/adr/0032). The first is homerun2-radio-pitcher,
// which airs the numbers on the LED panel between notifications.
//
// Counts only, for the reason the page gives (issue #121), and nothing that
// lets the caller change anything. Behind STATISTICS_TOKEN, not the
// scoreboard's: reading the counts is not entering a result.

// maxStatisticsDays bounds ?days=. A year; the match cap bites long before.
const maxStatisticsDays = 366

// statisticsLeaders is how many names most_wins lists at most.
const statisticsLeaders = 3

type apiStatistics struct {
	// From is the start of the window, or absent for everything on record.
	From  *time.Time `json:"from,omitempty"`
	Until time.Time  `json:"until"`
	// Truncated reports that the match cap cut into the window, so every
	// count below is too low. The page says the same in words.
	Truncated   bool             `json:"truncated"`
	Matches     int              `json:"matches"`
	Sets        int              `json:"sets"`
	Points      int              `json:"points"`
	Deuce       int              `json:"deuce"`
	Whitewashes int              `json:"whitewashes"`
	LongestSet  *apiSetScore     `json:"longest_set,omitempty"`
	MostWins    []apiPlayerScore `json:"most_wins"`
}

// apiSetScore is one set as it was played, home first.
type apiSetScore struct {
	Home int `json:"home"`
	Away int `json:"away"`
}

// apiPlayerScore is one player's record in the window: confirmed matches
// only, against players who are still playing.
type apiPlayerScore struct {
	ID          uuid.UUID `json:"id"`
	DisplayName string    `json:"display_name"`
	Won         int       `json:"won"`
	Lost        int       `json:"lost"`
}

// statisticsAuthorized is scoreboardAuthorized for the reader's token.
func (s *Server) statisticsAuthorized(r *http.Request) bool {
	token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(token), []byte(s.cfg.StatisticsToken)) == 1
}

// handleAPIStatistics answers ?days=N (1..366) for the last N days, or
// everything on record without it.
func (s *Server) handleAPIStatistics(w http.ResponseWriter, r *http.Request) {
	if !s.statisticsAuthorized(r) {
		s.apiRefuse(w, r, http.StatusUnauthorized, "a bearer token is required")
		return
	}

	until := time.Now()
	var from time.Time
	if raw := r.URL.Query().Get("days"); raw != "" {
		days, err := strconv.Atoi(raw)
		if err != nil || days < 1 || days > maxStatisticsDays {
			s.apiRefuse(w, r, http.StatusBadRequest, "days must be a whole number from 1 to 366")
			return
		}
		from = until.AddDate(0, 0, -days)
	}

	ctx := r.Context()
	recent, err := s.store.Matches().Recent(ctx, statisticsLimit)
	if err != nil {
		s.log.ErrorContext(ctx, "loading matches for the statistics api failed", "error", err)
		s.apiRefuse(w, r, http.StatusInternalServerError, "the statistics are not available")
		return
	}
	players, err := s.store.Players().Playing(ctx)
	if err != nil {
		s.log.ErrorContext(ctx, "loading players for the statistics api failed", "error", err)
		s.apiRefuse(w, r, http.StatusInternalServerError, "the statistics are not available")
		return
	}

	matches := stats.Since(recent, from)
	totals := stats.Compute(matches)
	out := apiStatistics{
		Until: until,
		// Recent is newest first: the cap cut into the window only when the
		// last match it returned is still inside it.
		Truncated:   len(recent) == statisticsLimit && len(matches) == len(recent),
		Matches:     totals.Matches,
		Sets:        totals.Sets,
		Points:      totals.Points,
		Deuce:       totals.Deuce,
		Whitewashes: totals.Whitewashes,
		MostWins:    []apiPlayerScore{},
	}
	if !from.IsZero() {
		out.From = &from
	}
	if totals.LongestSet > 0 {
		out.LongestSet = &apiSetScore{Home: totals.LongestSetHome, Away: totals.LongestSetAway}
	}

	ids := make([]uuid.UUID, 0, len(players))
	names := make(map[uuid.UUID]string, len(players))
	for _, p := range players {
		ids = append(ids, p.ID)
		names[p.ID] = p.DisplayName
	}
	for _, row := range stats.MostWins(stats.Matrix(ids, matches), statisticsLeaders) {
		out.MostWins = append(out.MostWins, apiPlayerScore{
			ID: row.PlayerID, DisplayName: names[row.PlayerID], Won: row.Won, Lost: row.Lost,
		})
	}
	s.writeJSON(w, r, http.StatusOK, out)
}
