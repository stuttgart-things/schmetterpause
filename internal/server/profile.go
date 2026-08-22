package server

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strconv"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/auth"
	"github.com/stuttgart-things/schmetterpause/internal/domain"
	"github.com/stuttgart-things/schmetterpause/internal/standings"
	"github.com/stuttgart-things/schmetterpause/internal/templates"
)

const (
	// profileMatches and profileHistory bound what a profile loads. Nobody
	// scrolls past twenty matches, and the chart stops being readable long
	// before fifty points.
	profileMatches = 20
	profileHistory = 50

	// The sparkline's box. Wide enough for a shape, short enough to sit
	// beside a number rather than under a heading.
	sparkWidth  = 240
	sparkHeight = 48
	// sparkPad keeps the 2px line and the 8px endpoint marker inside the box.
	sparkPad = 6
)

// handleStandingsFragment serves the ranking on its own.
func (s *Server) handleStandingsFragment(w http.ResponseWriter, r *http.Request) {
	view, err := s.standingsView(r.Context())
	if err != nil {
		s.log.ErrorContext(r.Context(), "loading the standings failed", "error", err)
		// User-facing text stays German; see CLAUDE.md.
		http.Error(w, "Rangliste nicht verfügbar", http.StatusInternalServerError)
		return
	}
	s.render(w, r, templates.Standings(view))
}

// handleProfile renders one player's page.
func (s *Server) handleProfile(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Diesen Spieler gibt es nicht", http.StatusNotFound)
		return
	}

	view, err := s.profileView(r.Context(), id)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		http.Error(w, "Diesen Spieler gibt es nicht", http.StatusNotFound)
		return
	case err != nil:
		s.log.ErrorContext(r.Context(), "loading the profile failed", "player_id", id, "error", err)
		http.Error(w, "Profil nicht verfügbar", http.StatusInternalServerError)
		return
	}

	s.render(w, r, templates.Profile(view))
}

// standingsView builds the ranking, marking the reader's own row.
func (s *Server) standingsView(ctx context.Context) (templates.StandingsView, error) {
	records, err := s.store.Players().Records(ctx)
	if err != nil {
		return templates.StandingsView{}, err
	}

	self, _ := auth.PlayerID(ctx)

	rows := standings.Build(records)
	view := templates.StandingsView{Rows: make([]templates.StandingRow, 0, len(rows))}
	for _, row := range rows {
		view.Rows = append(view.Rows, templates.StandingRow{
			ID:          row.Record.Player.ID.String(),
			Rank:        row.Rank,
			Shared:      row.Shared,
			DisplayName: row.Record.Player.DisplayName,
			TTR:         row.Record.Player.TTR,
			Played:      row.Record.Played,
			Won:         row.Record.Won,
			Lost:        row.Record.Lost,
			IsSelf:      row.Record.Player.ID == self && self != uuid.Nil,
		})
	}
	return view, nil
}

// profileView gathers everything one player's page shows.
func (s *Server) profileView(ctx context.Context, id uuid.UUID) (templates.ProfileView, error) {
	records, err := s.store.Players().Records(ctx)
	if err != nil {
		return templates.ProfileView{}, err
	}

	view := templates.ProfileView{Header: s.headerView(ctx)}
	found := false
	for _, row := range standings.Build(records) {
		if row.Record.Player.ID != id {
			continue
		}
		view.DisplayName = row.Record.Player.DisplayName
		view.TTR = row.Record.Player.TTR
		view.Rank, view.Shared = row.Rank, row.Shared
		view.Played, view.Won, view.Lost = row.Record.Played, row.Record.Won, row.Record.Lost
		found = true
		break
	}
	if !found {
		return templates.ProfileView{}, domain.ErrNotFound
	}

	history, err := s.store.TTRHistory().ForPlayer(ctx, id, profileHistory)
	if err != nil {
		return templates.ProfileView{}, err
	}
	// ForPlayer returns newest first; a chart and a "last change" both read
	// the other way round.
	oldestFirst := slices.Clone(history)
	slices.Reverse(oldestFirst)

	view.Spark = buildSpark(oldestFirst)
	if len(history) > 0 {
		view.Delta, view.HasDelta = history[0].Delta(), true
	}

	matches, err := s.store.Matches().RecentFor(ctx, id, profileMatches)
	if err != nil {
		return templates.ProfileView{}, err
	}

	deltas := make(map[uuid.UUID]int, len(history))
	for _, change := range history {
		deltas[change.MatchID] = change.Delta()
	}

	names := map[uuid.UUID]string{}
	for _, record := range records {
		names[record.Player.ID] = record.Player.DisplayName
	}

	view.Matches = make([]templates.ProfileMatchView, 0, len(matches))
	for _, m := range matches {
		atHome := m.HomeID == id
		opponentID := m.AwayID
		if !atHome {
			opponentID = m.HomeID
		}

		entry := templates.ProfileMatchView{
			// Date only: the day is what people remember, and a timestamp
			// would push the table into a horizontal scroll on a phone.
			PlayedAt:     m.PlayedAt.Format("02.01.2006"),
			OpponentName: names[opponentID],
			Sets:         make([]templates.SetScore, 0, len(m.Sets)),
			Pending:      m.Status == domain.MatchPending,
			Disputed:     m.Status == domain.MatchDisputed,
		}

		for _, set := range m.Sets {
			own, opponent := set.HomePoints, set.AwayPoints
			if !atHome {
				own, opponent = opponent, own
			}
			entry.Sets = append(entry.Sets, templates.SetScore{Own: own, Opponent: opponent})
			if own > opponent {
				entry.OwnSets++
			} else {
				entry.OpponentSets++
			}
		}
		entry.Won = entry.OwnSets > entry.OpponentSets

		if delta, ok := deltas[m.ID]; ok {
			entry.Delta, entry.HasDelta = delta, true
		}

		view.Matches = append(view.Matches, entry)
	}

	return view, nil
}

// buildSpark turns a rating history, oldest first, into polyline geometry.
//
// The series is the rating *before* the first match followed by the rating
// after each one, so the line starts where the player started rather than
// after their first result.
//
// The vertical range is the data's own, not zero to something: ratings sit
// around a thousand, and a zero baseline would flatten every match ever
// played into one straight line. That is only honest because the range is
// labelled next to the chart.
func buildSpark(oldestFirst []domain.TTRChange) templates.SparkView {
	if len(oldestFirst) == 0 {
		return templates.SparkView{}
	}

	values := make([]int, 0, len(oldestFirst)+1)
	values = append(values, oldestFirst[0].TTRBefore)
	for _, change := range oldestFirst {
		values = append(values, change.TTRAfter)
	}
	if len(values) < 2 {
		// A single point is a dot pretending to be a trend.
		return templates.SparkView{}
	}

	low, high := slices.Min(values), slices.Max(values)

	var (
		usableW = float64(sparkWidth - 2*sparkPad)
		usableH = float64(sparkHeight - 2*sparkPad)
		points  = make([]byte, 0, len(values)*12)
		lastX   string
		lastY   string
	)

	for i, v := range values {
		x := float64(sparkPad) + usableW*float64(i)/float64(len(values)-1)

		// A flat history has no range to scale against, so it sits in the
		// middle rather than dividing by zero or hugging an edge.
		y := float64(sparkHeight) / 2
		if high != low {
			y = float64(sparkPad) + usableH*float64(high-v)/float64(high-low)
		}

		if i > 0 {
			points = append(points, ' ')
		}
		lastX, lastY = coord(x), coord(y)
		points = append(points, lastX...)
		points = append(points, ',')
		points = append(points, lastY...)
	}

	return templates.SparkView{
		Show:   true,
		Points: string(points),
		LastX:  lastX,
		LastY:  lastY,
		Width:  sparkWidth,
		Height: sparkHeight,
		Low:    low,
		High:   high,
	}
}

func coord(v float64) string {
	return strconv.FormatFloat(v, 'f', 1, 64)
}
