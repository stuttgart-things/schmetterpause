package server

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
	"github.com/stuttgart-things/schmetterpause/internal/templates"
)

// matchListLimit bounds the list. A year is a few thousand rows and nobody
// scrolls that far, but the cap is stated on the page when it bites: a list
// that silently stops reads as the whole history.
//
// Paging is cheap to add later — matches_status_idx makes keyset paging on
// played_at free — and is not worth building for a volume nobody has yet.
const matchListLimit = 200

// handleMatchList serves every match the office has played.
//
// The ranking says who is ahead; this says what happened. It reads only what
// is already stored — matches, their sets, and what each one was worth — so
// there is nothing here a migration had to make room for.
func (s *Server) handleMatchList(w http.ResponseWriter, r *http.Request) {
	view, err := s.matchListView(r.Context())
	if err != nil {
		s.log.ErrorContext(r.Context(), "loading the match list failed", "error", err)
		http.Error(w, "Matches nicht verfügbar", http.StatusInternalServerError)
		return
	}
	s.render(w, r, templates.MatchList(view))
}

func (s *Server) matchListView(ctx context.Context) (templates.MatchListView, error) {
	matches, err := s.store.Matches().Recent(ctx, matchListLimit)
	if err != nil {
		return templates.MatchListView{}, err
	}

	players, err := s.store.Players().List(ctx)
	if err != nil {
		return templates.MatchListView{}, err
	}
	names := make(map[uuid.UUID]string, len(players))
	for _, p := range players {
		names[p.ID] = p.DisplayName
	}

	ids := make([]uuid.UUID, 0, len(matches))
	for _, m := range matches {
		ids = append(ids, m.ID)
	}
	// One query for all of them. Asking per match would turn one page into
	// one round trip per row.
	changes, err := s.store.TTRHistory().ForMatches(ctx, ids)
	if err != nil {
		return templates.MatchListView{}, err
	}
	// Keyed by match and player: the winner's change is the one to show, and
	// which player that is only becomes clear from the sets below.
	deltas := make(map[uuid.UUID]map[uuid.UUID]int, len(changes))
	for _, c := range changes {
		if deltas[c.MatchID] == nil {
			deltas[c.MatchID] = map[uuid.UUID]int{}
		}
		deltas[c.MatchID][c.PlayerID] = c.Delta()
	}

	view := templates.MatchListView{
		Header:    s.headerView(ctx),
		Matches:   make([]templates.MatchListRow, 0, len(matches)),
		Limit:     matchListLimit,
		Truncated: len(matches) == matchListLimit,
	}
	for _, m := range matches {
		view.Matches = append(view.Matches, matchListRow(m, names, deltas[m.ID]))
	}
	return view, nil
}

// matchListRow turns one stored match into a row read from the winner's side.
//
// The winner comes from the set scores, not from the rating change: a match
// that is still waiting for its opponent has no rating change at all, and it
// still has a winner on the table.
func matchListRow(m domain.Match, names map[uuid.UUID]string, deltas map[uuid.UUID]int) templates.MatchListRow {
	var homeSets, awaySets int
	for _, set := range m.Sets {
		if set.HomePoints > set.AwayPoints {
			homeSets++
		} else {
			awaySets++
		}
	}

	winner, loser := m.HomeID, m.AwayID
	homeWon := homeSets > awaySets
	if !homeWon {
		winner, loser = m.AwayID, m.HomeID
	}

	row := templates.MatchListRow{
		// Date only: the day is what people remember, and a timestamp would
		// push the table into a horizontal scroll on a phone.
		PlayedAt:   m.PlayedAt.Format("02.01.2006"),
		WinnerName: names[winner],
		LoserName:  names[loser],
		WinnerID:   winner.String(),
		LoserID:    loser.String(),
		Sets:       make([]templates.SetScore, 0, len(m.Sets)),
		Pending:    m.Status == domain.MatchPending,
		Disputed:   m.Status == domain.MatchDisputed,
	}
	row.WinnerSets, row.LoserSets = homeSets, awaySets
	if !homeWon {
		row.WinnerSets, row.LoserSets = awaySets, homeSets
	}

	for _, set := range m.Sets {
		own, other := set.HomePoints, set.AwayPoints
		if !homeWon {
			own, other = other, own
		}
		row.Sets = append(row.Sets, templates.SetScore{Own: own, Opponent: other})
	}

	if delta, ok := deltas[winner]; ok {
		row.Delta, row.HasDelta = delta, true
	}
	return row
}
