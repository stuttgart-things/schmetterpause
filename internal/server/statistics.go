package server

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/stats"
	"github.com/stuttgart-things/schmetterpause/internal/templates"
)

// statisticsLimit bounds how many matches the page reads.
//
// The same cap the match list uses, and for the same reason: a year is a few
// thousand rows, and the number is stated on the page when it bites. A total
// that silently stopped counting would be worse than no total — it would read
// as the whole history.
const statisticsLimit = matchListLimit

// handleStatistics serves what the results already know.
//
// Counts only. Issue #121 records why: below about a hundred matches a
// percentage is noise wearing a percent sign, and it gets believed because of
// the formatting. Three wins to one is three wins to one at any volume.
func (s *Server) handleStatistics(w http.ResponseWriter, r *http.Request) {
	view, err := s.statisticsView(r.Context())
	if err != nil {
		s.log.ErrorContext(r.Context(), "loading the statistics failed", "error", err)
		http.Error(w, "Statistik nicht verfügbar", http.StatusInternalServerError)
		return
	}
	s.render(w, r, templates.Statistics(view))
}

func (s *Server) statisticsView(ctx context.Context) (templates.StatisticsView, error) {
	matches, err := s.store.Matches().Recent(ctx, statisticsLimit)
	if err != nil {
		return templates.StatisticsView{}, err
	}

	// In ranking order, so the matrix reads top to bottom the way the table
	// on the front page does. List already returns them by rating.
	players, err := s.store.Players().List(ctx)
	if err != nil {
		return templates.StatisticsView{}, err
	}

	ids := make([]uuid.UUID, 0, len(players))
	names := make([]string, 0, len(players))
	for _, p := range players {
		ids = append(ids, p.ID)
		names = append(names, p.DisplayName)
	}

	totals := stats.Compute(matches)
	view := templates.StatisticsView{
		Header:    s.headerView(ctx),
		Names:     names,
		Limit:     statisticsLimit,
		Truncated: len(matches) == statisticsLimit,
		Totals: templates.StatisticsTotals{
			Matches:     totals.Matches,
			Sets:        totals.Sets,
			Points:      totals.Points,
			Deuce:       totals.Deuce,
			Whitewashes: totals.Whitewashes,
		},
	}
	if totals.LongestSet > 0 {
		view.Totals.LongestSet = itoa(totals.LongestSetHome) + ":" + itoa(totals.LongestSetAway)
	}

	for i, row := range stats.Matrix(ids, matches) {
		out := templates.StatisticsRow{
			ID:          row.PlayerID.String(),
			DisplayName: names[i],
			Won:         row.Won,
			Lost:        row.Lost,
			Cells:       make([]templates.StatisticsCell, 0, len(row.Cells)),
		}
		for j, cell := range row.Cells {
			out.Cells = append(out.Cells, templates.StatisticsCell{
				Record:   record(cell),
				Self:     cell.Self,
				Opponent: names[j],
			})
		}
		view.Matrix = append(view.Matrix, out)
	}
	return view, nil
}

// record states a cell, or nothing at all when the two have never met.
//
// Empty rather than "0:0": a pairing that never happened and one that somehow
// ended without a winner are different things, and only the first exists.
func record(c stats.Cell) string {
	if !c.Played() {
		return ""
	}
	return itoa(c.Won) + ":" + itoa(c.Lost)
}
