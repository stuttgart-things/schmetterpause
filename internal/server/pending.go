package server

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/auth"
	"github.com/stuttgart-things/schmetterpause/internal/domain"
	"github.com/stuttgart-things/schmetterpause/internal/match"
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
	view := templates.SettledView{ID: matchID.String(), Rated: settlement.Rated}
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
	s.refreshAfterRuling(w, r, self, settlement.Match.TournamentID)
}

// handleDisputeMatch contests a match the player does not agree with, and
// hands them the form to say what it really was.
func (s *Server) handleDisputeMatch(w http.ResponseWriter, r *http.Request) {
	self, matchID, ok := s.rulingRequest(w, r)
	if !ok {
		return
	}

	if err := scoring.Dispute(r.Context(), s.store, matchID, self); err != nil {
		s.reportRulingError(w, r, err, "disputing the match failed")
		return
	}

	// Read after the dispute, so the entry is rendered from the state that
	// now exists rather than the one that just stopped existing.
	view, err := s.pendingEntryView(r.Context(), matchID, self)
	if err != nil {
		s.log.ErrorContext(r.Context(), "loading the contested match failed", "error", err)
		http.Error(w, "Das hat gerade nicht geklappt", http.StatusInternalServerError)
		return
	}

	s.render(w, r, templates.PendingItem(view))
	// No tournament table to refresh: a disputed result was pending, and a
	// pending result was never in the table to begin with.
	s.refreshAfterRuling(w, r, self, nil)
}

// handleCorrectMatch replaces the result of a contested match and hands it
// back to the opponent for confirmation.
func (s *Server) handleCorrectMatch(w http.ResponseWriter, r *http.Request) {
	self, matchID, ok := s.rulingRequest(w, r)
	if !ok {
		return
	}

	m, err := s.store.Matches().ByID(r.Context(), matchID)
	if err != nil {
		s.reportCorrectionError(w, r, err)
		return
	}

	form, msg := parseResultForm(r)

	if msg == "" {
		correction, err := scoring.Correct(r.Context(), s.store, matchID, self,
			asPlayed(form.result, m.HomeID == self))

		var rejection *match.Rejection
		switch {
		case err == nil:
			ownSets, opponentSets := correction.AwaySets, correction.HomeSets
			if m.HomeID == self {
				ownSets, opponentSets = correction.HomeSets, correction.AwaySets
			}
			s.render(w, r, templates.Corrected(templates.CorrectedView{
				ID:           matchID.String(),
				OpponentName: correction.Opponent.DisplayName,
				OwnSets:      ownSets,
				OpponentSets: opponentSets,
			}))
			// Same as a dispute: a corrected result goes back to pending,
			// so it has not entered any table yet.
			s.refreshAfterRuling(w, r, self, nil)
			return
		case errors.As(err, &rejection):
			msg = describeRejection(err)
		default:
			s.reportCorrectionError(w, r, err)
			return
		}
	}

	view, err := s.pendingEntryView(r.Context(), matchID, self)
	if err != nil {
		s.log.ErrorContext(r.Context(), "loading the contested match failed", "error", err)
		http.Error(w, "Das hat gerade nicht geklappt", http.StatusInternalServerError)
		return
	}
	// Hand back what was typed rather than what is stored, so a correction
	// is not lost to one mistyped number.
	view.Inputs = form.typed
	view.BestOf, view.PointsToWin = form.bestOf, form.pointsToWin
	view.Error = msg

	// 422: the request was well formed, the result in it was not.
	w.WriteHeader(http.StatusUnprocessableEntity)
	s.render(w, r, templates.PendingItem(view))
}

// asPlayed turns a result typed from one player's side into the home/away
// orientation the domain stores. The form always reads "own : opponent"; who
// that is depends on which side of the table they were on.
func asPlayed(result match.Result, atHome bool) match.Result {
	if atHome {
		return result
	}

	flipped := match.Result{Mode: result.Mode, Sets: make([]match.Set, len(result.Sets))}
	for i, set := range result.Sets {
		flipped.Sets[i] = match.Set{Home: set.Away, Away: set.Home}
	}
	return flipped
}

// reportCorrectionError maps a failed correction onto a status and a
// sentence. Separate from reportRulingError because the same errors mean
// something else here: a correction is refused for being late, not for being
// somebody else's call.
func (s *Server) reportCorrectionError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		http.Error(w, "Dieses Match gibt es nicht", http.StatusNotFound)
	case errors.Is(err, scoring.ErrNotYours):
		http.Error(w, "Da hast du nicht mitgespielt", http.StatusForbidden)
	case errors.Is(err, scoring.ErrNotDisputed), errors.Is(err, domain.ErrConflict):
		http.Error(w, "Dieses Match ist nicht strittig", http.StatusConflict)
	default:
		s.log.ErrorContext(r.Context(), "correcting the match failed", "error", err)
		http.Error(w, "Das hat gerade nicht geklappt", http.StatusInternalServerError)
	}
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

// refreshAfterRuling swaps the ranking, the pending list and the badge out of
// band, since a ruling changes all three.
//
// tournamentID is the draw the match belonged to, or nil. Where there is one
// the tournament table goes with them: a confirmation given on that page has
// just put the result into it, and a table that still says otherwise reads as
// a confirmation that did not take. The swaps that find no target on the page
// the request came from simply land nowhere, which is what lets one response
// serve both pages.
func (s *Server) refreshAfterRuling(
	w http.ResponseWriter,
	r *http.Request,
	self uuid.UUID,
	tournamentID *uuid.UUID,
) {
	s.render(w, r, templates.WhoamiOOB(s.headerView(r.Context())))

	table, err := s.standingsView(r.Context())
	if err != nil {
		s.log.ErrorContext(r.Context(), "loading the standings failed", "error", err)
		return
	}
	s.render(w, r, templates.StandingsOOB(table))

	pending, err := s.pendingListView(r.Context(), self)
	if err != nil {
		s.log.ErrorContext(r.Context(), "loading the pending matches failed", "error", err)
		return
	}
	s.render(w, r, templates.PendingListOOB(pending))

	if tournamentID == nil {
		return
	}
	// kiosk=false: this is the copy a player reads, which is the only one
	// that could have carried the confirmation.
	tour, err := s.tournamentView(r.Context(), *tournamentID, false)
	if err != nil {
		s.log.ErrorContext(r.Context(), "loading the tournament failed",
			"tournament_id", *tournamentID, "error", err)
		return
	}
	s.render(w, r, templates.TournamentTableOOB(tour))
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
		entry, err := s.pendingEntry(ctx, m, self, names)
		if err != nil {
			return templates.PendingListView{}, err
		}
		view.Matches = append(view.Matches, entry)
	}
	return view, nil
}

// pendingEntryView describes a single waiting match, for the responses that
// replace one entry rather than the whole list.
func (s *Server) pendingEntryView(ctx context.Context, matchID, self uuid.UUID) (templates.PendingMatchView, error) {
	m, err := s.store.Matches().ByID(ctx, matchID)
	if err != nil {
		return templates.PendingMatchView{}, err
	}
	return s.pendingEntry(ctx, m, self, map[uuid.UUID]string{})
}

// pendingEntry turns a match into the entry this player sees. names caches
// display names across a list; pass an empty map for a single entry.
func (s *Server) pendingEntry(
	ctx context.Context,
	m domain.Match,
	self uuid.UUID,
	names map[uuid.UUID]string,
) (templates.PendingMatchView, error) {
	lookup := func(id uuid.UUID) (string, error) {
		if name, ok := names[id]; ok {
			return name, nil
		}
		player, err := s.store.Players().ByID(ctx, id)
		if err != nil {
			return "", err
		}
		names[id] = player.DisplayName
		return player.DisplayName, nil
	}

	opponentID := m.AwayID
	if m.AwayID == self {
		opponentID = m.HomeID
	}

	reporterName, err := lookup(m.ReportedBy)
	if err != nil {
		return templates.PendingMatchView{}, err
	}
	opponentName, err := lookup(opponentID)
	if err != nil {
		return templates.PendingMatchView{}, err
	}

	atHome := m.HomeID == self
	entry := templates.PendingMatchView{
		ID:           m.ID.String(),
		ReporterName: reporterName,
		OpponentName: opponentName,
		Disputed:     m.Status == domain.MatchDisputed,
		BestOf:       m.BestOf,
		PointsToWin:  m.PointsToWin,
		Sets:         make([]templates.SetScore, 0, len(m.Sets)),
		Inputs:       make([]templates.SetInput, templates.MaxSetRows),
	}

	for i, set := range m.Sets {
		own, opponent := set.HomePoints, set.AwayPoints
		if !atHome {
			own, opponent = opponent, own
		}
		entry.Sets = append(entry.Sets, templates.SetScore{Own: own, Opponent: opponent})
		// The correction form opens on what was reported, so fixing one
		// number does not mean retyping the other seven.
		if i < templates.MaxSetRows {
			entry.Inputs[i] = templates.SetInput{
				Home: strconv.Itoa(own), Away: strconv.Itoa(opponent),
			}
		}
		if own > opponent {
			entry.OwnSets++
		} else {
			entry.OpponentSets++
		}
	}
	entry.Won = entry.OwnSets > entry.OpponentSets

	return entry, nil
}
