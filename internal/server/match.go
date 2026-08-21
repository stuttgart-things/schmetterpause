package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/auth"
	"github.com/stuttgart-things/schmetterpause/internal/domain"
	"github.com/stuttgart-things/schmetterpause/internal/match"
	"github.com/stuttgart-things/schmetterpause/internal/templates"
)

// handleMatchForm serves a fresh entry form.
func (s *Server) handleMatchForm(w http.ResponseWriter, r *http.Request) {
	self, ok := auth.PlayerID(r.Context())
	if !ok {
		// User-facing text stays German; see CLAUDE.md.
		http.Error(w, "Erst mitspielen, dann eintragen", http.StatusUnauthorized)
		return
	}

	opponents, err := s.opponentOptions(r.Context(), self, uuid.Nil)
	if err != nil {
		s.log.ErrorContext(r.Context(), "loading the opponents failed", "error", err)
		http.Error(w, "Gegnerliste nicht verfügbar", http.StatusInternalServerError)
		return
	}

	s.render(w, r, templates.MatchForm(templates.NewMatchFormView(opponents)))
}

// handleRecordMatch validates a submitted result and stores it as pending.
//
// Nothing is scored here. The rating waits for the opponent to confirm (AP5),
// which is also the only real defence against a joke result — see the
// threat-model note in docs/adr/0004.
func (s *Server) handleRecordMatch(w http.ResponseWriter, r *http.Request) {
	self, ok := auth.PlayerID(r.Context())
	if !ok {
		http.Error(w, "Erst mitspielen, dann eintragen", http.StatusUnauthorized)
		return
	}

	form, opponentID, msg := parseMatchForm(r)
	if msg == "" {
		if _, err := match.Validate(form.result); err != nil {
			msg = describeRejection(err)
		}
	}

	if msg != "" {
		s.rejectMatch(w, r, self, opponentID, form, msg)
		return
	}

	opponent, err := s.store.Players().ByID(r.Context(), opponentID)
	if err != nil {
		s.log.WarnContext(r.Context(), "the chosen opponent does not exist",
			"opponent_id", opponentID, "error", err)
		s.rejectMatch(w, r, self, uuid.Nil, form, "Diesen Gegner gibt es nicht.")
		return
	}

	sets := make([]domain.MatchSet, 0, len(form.result.Sets))
	for i, set := range form.result.Sets {
		sets = append(sets, domain.MatchSet{
			SetNo: i + 1, HomePoints: set.Home, AwayPoints: set.Away,
		})
	}

	created, err := s.store.Matches().Create(r.Context(), domain.Match{
		HomeID:      self,
		AwayID:      opponent.ID,
		BestOf:      form.result.Mode.BestOf,
		PointsToWin: form.result.Mode.PointsToWin,
		Status:      domain.MatchPending,
		ReportedBy:  self,
		PlayedAt:    time.Now(),
		Sets:        sets,
	})
	if err != nil {
		s.log.ErrorContext(r.Context(), "recording the match failed", "error", err)
		s.rejectMatch(w, r, self, opponentID, form,
			"Das hat gerade nicht geklappt. Versuch es noch einmal.")
		return
	}

	outcome, _ := match.Validate(form.result)

	s.log.InfoContext(r.Context(), "match recorded",
		"match_id", created.ID, "home", self, "away", opponent.ID,
		"sets", fmt.Sprintf("%d:%d", outcome.HomeSets, outcome.AwaySets))

	s.render(w, r, templates.MatchRecorded(templates.MatchRecordedView{
		OpponentName: opponent.DisplayName,
		OwnSets:      outcome.HomeSets,
		OpponentSets: outcome.AwaySets,
		Won:          outcome.HomeWon,
	}))
}

// rejectMatch re-renders the form with the reason and everything still filled in.
func (s *Server) rejectMatch(
	w http.ResponseWriter, r *http.Request,
	self, opponentID uuid.UUID, form matchForm, msg string,
) {
	opponents, err := s.opponentOptions(r.Context(), self, opponentID)
	if err != nil {
		s.log.ErrorContext(r.Context(), "loading the opponents failed", "error", err)
		http.Error(w, "Gegnerliste nicht verfügbar", http.StatusInternalServerError)
		return
	}

	view := templates.MatchFormView{
		Opponents:   opponents,
		BestOf:      form.bestOf,
		PointsToWin: form.pointsToWin,
		Sets:        form.typed,
		Error:       msg,
	}

	// 422: the request was well formed, the result in it was not.
	w.WriteHeader(http.StatusUnprocessableEntity)
	s.render(w, r, templates.MatchForm(view))
}

// opponentOptions lists everyone except the player themselves.
func (s *Server) opponentOptions(ctx context.Context, self, selected uuid.UUID) ([]templates.OpponentOption, error) {
	players, err := s.store.Players().List(ctx)
	if err != nil {
		return nil, err
	}

	options := make([]templates.OpponentOption, 0, len(players))
	for _, p := range players {
		if p.ID == self {
			// The schema rejects a match against yourself
			// (matches_players_differ); not offering it is friendlier than
			// explaining it afterwards.
			continue
		}
		options = append(options, templates.OpponentOption{
			ID:          p.ID.String(),
			DisplayName: p.DisplayName,
			Selected:    p.ID == selected,
		})
	}
	return options, nil
}

// matchForm is a submitted form, both as typed and as parsed.
type matchForm struct {
	bestOf      int
	pointsToWin int
	// typed keeps every row as entered so a rejection can hand it back.
	typed  []templates.SetInput
	result match.Result
}

// parseMatchForm reads the form. The returned message is empty when the input
// could be read at all — whether the result is *possible* is match.Validate's
// question, not this one's.
func parseMatchForm(r *http.Request) (matchForm, uuid.UUID, string) {
	form := matchForm{
		bestOf:      5,
		pointsToWin: match.PointsToEleven,
		typed:       make([]templates.SetInput, templates.MaxSetRows),
	}

	if v, err := strconv.Atoi(r.FormValue("best_of")); err == nil {
		form.bestOf = v
	}
	if v, err := strconv.Atoi(r.FormValue("points_to_win")); err == nil {
		form.pointsToWin = v
	}
	form.result.Mode = match.Mode{BestOf: form.bestOf, PointsToWin: form.pointsToWin}

	var (
		opponentID uuid.UUID
		message    string
	)

	if raw := strings.TrimSpace(r.FormValue("opponent_id")); raw == "" {
		message = "Wähle einen Gegner."
	} else if id, err := uuid.Parse(raw); err != nil {
		message = "Diesen Gegner gibt es nicht."
	} else {
		opponentID = id
	}

	// Rows are read in order and the first empty one ends the match. A row
	// filled in after that is a gap, which almost always means a typo in the
	// wrong box rather than a genuinely skipped set.
	ended := false
	for i := range templates.MaxSetRows {
		home := strings.TrimSpace(r.FormValue(fmt.Sprintf("set_home_%d", i+1)))
		away := strings.TrimSpace(r.FormValue(fmt.Sprintf("set_away_%d", i+1)))
		form.typed[i] = templates.SetInput{Home: home, Away: away}

		switch {
		case home == "" && away == "":
			ended = true
			continue
		case ended:
			if message == "" {
				message = fmt.Sprintf("Satz %d steht ausgefüllt hinter einem leeren Satz.", i+1)
			}
			continue
		case home == "" || away == "":
			if message == "" {
				message = fmt.Sprintf("Satz %d ist nur halb ausgefüllt.", i+1)
			}
			continue
		}

		homePoints, homeErr := strconv.Atoi(home)
		awayPoints, awayErr := strconv.Atoi(away)
		if homeErr != nil || awayErr != nil {
			if message == "" {
				message = fmt.Sprintf("Satz %d enthält keine Zahl.", i+1)
			}
			continue
		}

		form.result.Sets = append(form.result.Sets, match.Set{Home: homePoints, Away: awayPoints})
	}

	return form, opponentID, message
}

// describeRejection turns a domain rejection into a sentence a player can act
// on. The Definition of Done for this package is that a rejected result says
// why, so "ungültige Eingabe" is not an acceptable answer to any of these.
func describeRejection(err error) string {
	var rejection *match.Rejection
	if !errors.As(err, &rejection) {
		return "Das Ergebnis ist so nicht möglich."
	}

	switch rejection.Kind {
	case match.KindUnknownMode:
		return "Diesen Modus gibt es nicht."
	case match.KindNoSets:
		return "Trag mindestens einen Satz ein."
	case match.KindTooManySets:
		return fmt.Sprintf("Bei Best of %d sind höchstens %d Sätze möglich, eingetragen sind %d.",
			rejection.Want, rejection.Want, rejection.Got)
	case match.KindNegativePoints:
		return fmt.Sprintf("Satz %d hat negative Punkte.", rejection.SetNo)
	case match.KindDraw:
		return fmt.Sprintf("Satz %d steht unentschieden — im Tischtennis gewinnt immer jemand.",
			rejection.SetNo)
	case match.KindSetNotFinished:
		return fmt.Sprintf("Satz %d ist noch nicht zu Ende: es braucht mindestens %d Punkte.",
			rejection.SetNo, rejection.Want)
	case match.KindMarginTooSmall:
		return fmt.Sprintf("Satz %d ist zu knapp: es braucht zwei Punkte Vorsprung.",
			rejection.SetNo)
	case match.KindOvershoot:
		return fmt.Sprintf(
			"Satz %d kann so nicht ausgegangen sein: ab %d Punkten endet der Satz, sobald jemand zwei Punkte vorn liegt.",
			rejection.SetNo, rejection.Want)
	case match.KindNoWinner:
		return fmt.Sprintf("Das Match ist noch nicht entschieden: es braucht %d gewonnene Sätze.",
			rejection.Want)
	case match.KindSetsAfterDecision:
		return fmt.Sprintf("Nach Satz %d war das Match schon entschieden.", rejection.SetNo-1)
	}
	return "Das Ergebnis ist so nicht möglich."
}
