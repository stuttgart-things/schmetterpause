package server

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/auth"
	"github.com/stuttgart-things/schmetterpause/internal/domain"
	"github.com/stuttgart-things/schmetterpause/internal/scoring"
	"github.com/stuttgart-things/schmetterpause/internal/templates"
)

// handlePendingFragment lists the results waiting on the player.
func (s *Server) handlePendingFragment(w http.ResponseWriter, r *http.Request) {
	self, ok := auth.PlayerID(r.Context())
	if !ok {
		// User-facing text stays German; see CLAUDE.md.
		http.Error(w, "Erst mitspielen", http.StatusUnauthorized)
		return
	}

	view, err := s.pendingListView(r.Context(), self)
	if err != nil {
		s.log.ErrorContext(r.Context(), "loading the pending matches failed", "error", err)
		http.Error(w, "Offene Ergebnisse nicht verfügbar", http.StatusInternalServerError)
		return
	}
	s.render(w, r, templates.PendingList(view))
}

// handleConfirmMatch settles a match the player agrees with.
func (s *Server) handleConfirmMatch(w http.ResponseWriter, r *http.Request) {
	self, matchID, ok := s.rulingRequest(w, r)
	if !ok {
		return
	}

	settlement, err := scoring.Confirm(r.Context(), s.store, matchID, self, time.Now())
	if err != nil {
		s.reportRulingError(w, r, err, "confirming the match failed")
		return
	}

	// The settlement is told from the home player's side; the reader is on
	// whichever side they played.
	view := templates.SettledView{ID: matchID.String()}
	if settlement.Home.ID == self {
		view.OpponentName = settlement.Away.DisplayName
		view.Won = settlement.HomeWon
		view.OwnSets, view.OpponentSets = settlement.HomeSets, settlement.AwaySets
		view.Delta, view.NewTTR = settlement.HomeChange.Delta(), settlement.HomeChange.After
	} else {
		view.OpponentName = settlement.Home.DisplayName
		view.Won = !settlement.HomeWon
		view.OwnSets, view.OpponentSets = settlement.AwaySets, settlement.HomeSets
		view.Delta, view.NewTTR = settlement.AwayChange.Delta(), settlement.AwayChange.After
	}

	s.render(w, r, templates.Settled(view))
	s.refreshAfterRuling(w, r, self)
}

// handleDisputeMatch contests a match the player does not agree with.
func (s *Server) handleDisputeMatch(w http.ResponseWriter, r *http.Request) {
	self, matchID, ok := s.rulingRequest(w, r)
	if !ok {
		return
	}

	// Read before the status changes, so the message can name whoever
	// entered it.
	m, err := s.store.Matches().ByID(r.Context(), matchID)
	if err != nil {
		s.reportRulingError(w, r, err, "loading the match failed")
		return
	}

	if err := scoring.Dispute(r.Context(), s.store, matchID, self); err != nil {
		s.reportRulingError(w, r, err, "disputing the match failed")
		return
	}

	reporter, err := s.store.Players().ByID(r.Context(), m.ReportedBy)
	if err != nil {
		s.log.WarnContext(r.Context(), "loading the reporter failed", "error", err)
	}

	s.render(w, r, templates.Disputed(templates.DisputedView{
		ID:           matchID.String(),
		ReporterName: reporter.DisplayName,
	}))
	s.refreshAfterRuling(w, r, self)
}

// rulingRequest resolves the player and the match id, answering the request
// itself when either is missing.
func (s *Server) rulingRequest(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	self, ok := auth.PlayerID(r.Context())
	if !ok {
		http.Error(w, "Erst mitspielen", http.StatusUnauthorized)
		return uuid.Nil, uuid.Nil, false
	}

	matchID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Dieses Match gibt es nicht", http.StatusNotFound)
		return uuid.Nil, uuid.Nil, false
	}
	return self, matchID, true
}

// reportRulingError maps a scoring failure onto a status and a sentence.
func (s *Server) reportRulingError(w http.ResponseWriter, r *http.Request, err error, msg string) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		http.Error(w, "Dieses Match gibt es nicht", http.StatusNotFound)
	case errors.Is(err, scoring.ErrNotYours):
		// Also the case where the reporter tries to confirm their own
		// result, which is the one this step exists to prevent.
		http.Error(w, "Darüber entscheidet dein Gegner", http.StatusForbidden)
	case errors.Is(err, scoring.ErrNotPending):
		http.Error(w, "Das ist schon entschieden", http.StatusConflict)
	default:
		s.log.ErrorContext(r.Context(), msg, "error", err)
		http.Error(w, "Das hat gerade nicht geklappt", http.StatusInternalServerError)
	}
}

// refreshAfterRuling swaps the roster and the pending list out of band, since
// a ruling changes both.
func (s *Server) refreshAfterRuling(w http.ResponseWriter, r *http.Request, self uuid.UUID) {
	players, err := s.playerListView(r.Context())
	if err != nil {
		s.log.ErrorContext(r.Context(), "loading the player list failed", "error", err)
		return
	}
	s.render(w, r, templates.PlayerListOOB(players))

	pending, err := s.pendingListView(r.Context(), self)
	if err != nil {
		s.log.ErrorContext(r.Context(), "loading the pending matches failed", "error", err)
		return
	}
	s.render(w, r, templates.PendingListOOB(pending))
}

// pendingListView describes the results waiting on this player, told from
// their side of the table.
func (s *Server) pendingListView(ctx context.Context, self uuid.UUID) (templates.PendingListView, error) {
	matches, err := s.store.Matches().PendingFor(ctx, self)
	if err != nil {
		return templates.PendingListView{}, err
	}

	names := make(map[uuid.UUID]string, len(matches))
	view := templates.PendingListView{Matches: make([]templates.PendingMatchView, 0, len(matches))}

	for _, m := range matches {
		name, ok := names[m.ReportedBy]
		if !ok {
			reporter, err := s.store.Players().ByID(ctx, m.ReportedBy)
			if err != nil {
				return templates.PendingListView{}, err
			}
			name = reporter.DisplayName
			names[m.ReportedBy] = name
		}

		atHome := m.HomeID == self
		entry := templates.PendingMatchView{
			ID:           m.ID.String(),
			ReporterName: name,
			Sets:         make([]templates.SetScore, 0, len(m.Sets)),
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

		view.Matches = append(view.Matches, entry)
	}
	return view, nil
}
