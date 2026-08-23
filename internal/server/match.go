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

// parseMatchForm reads result entry: the opponent, plus everything
// parseResultForm reads. The opponent is checked first so that a form with
// nothing filled in says the most useful thing about itself.
func parseMatchForm(r *http.Request) (matchForm, uuid.UUID, string) {
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

	form, setsMessage := parseResultForm(r)
	if message == "" {
		message = setsMessage
	}
	return form, opponentID, message
}

// parseResultForm reads the mode and the sets, shared by result entry and the
// correction of a contested match. The returned message is empty when the
// input could be read at all — whether the result is *possible* is
// match.Validate's question, not this one's.
func parseResultForm(r *http.Request) (matchForm, string) {
	form := matchForm{
		bestOf:      match.DefaultBestOf,
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

	var message string

	// Rows are read in order and the first empty one ends the match. A row
	// filled in after that is a gap, which almost always means a typo in the
	// wrong box rather than a genuinely skipped set.
	ended := false
	for i := range templates.MaxSetRows {
		home := strings.TrimSpace(r.FormValue(fmt.Sprintf("set_home_%d", i+1)))
		away := strings.TrimSpace(r.FormValue(fmt.Sprintf("set_away_%d", i+1)))
		form.typed[i] = templates.SetInput{Home: home, Away: away}

		switch {
		// Empty or 0:0 both mean "not played". The boxes come up with a zero
		// in them so the slider beside each one has something to point at,
		// and a set that ended 0:0 does not exist — table tennis has no
		// draws, which is why the domain rejects one. Reading the two the
		// same way is what lets the form have a default at all.
		case unplayed(home, away):
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

	return form, message
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

// unplayed reports whether a row carries no result: both boxes empty, both
// zero, or one of each.
func unplayed(home, away string) bool {
	blank := func(v string) bool {
		if v == "" {
			return true
		}
		n, err := strconv.Atoi(v)
		return err == nil && n == 0
	}
	return blank(home) && blank(away)
}

// setsPrefixFallback is the form the fragment renders into when the request
// names one that cannot be a form on any page.
const setsPrefixFallback = "entry"

// handleSetsFragment re-renders the set rows for the mode currently picked.
//
// One endpoint for all three forms — entry, kiosk and correction — because
// they differ in nothing the rows care about. The form says which one it is
// through the hidden sets_prefix field that setRows puts there, and hands its
// own contents along, so changing the mode never empties a box somebody has
// already filled in.
func (s *Server) handleSetsFragment(w http.ResponseWriter, r *http.Request) {
	form, _ := parseResultForm(r)
	home, away := s.setsColumns(r)

	view := templates.SetsView{
		Prefix:      setsPrefix(r.FormValue("sets_prefix")),
		BestOf:      form.bestOf,
		PointsToWin: form.pointsToWin,
		Sets:        form.typed,
		HomeLabel:   home,
		AwayLabel:   away,
	}
	if view.Prefix == kioskSetsPrefix {
		view.Picker = s.kioskPicker(r)
	}

	s.render(w, r, templates.SetsFragment(view))
}

// kioskSetsPrefix names the kiosk's copy of the set rows.
const kioskSetsPrefix = "kiosk"

// kioskPicker rebuilds the select the scorekeeper did not just touch, without
// the player the other one now holds.
//
// Nobody plays themselves, and the server says so — but only after the whole
// match has been typed in, which is too late to be useful. Taking the name out
// of the other list makes the mistake unavailable instead of punished.
//
// If the choice that is kept is the player just taken by the other side, it
// simply is not in the list any more and the select falls back to "bitte
// wählen". Better than silently keeping an impossible pair.
func (s *Server) kioskPicker(r *http.Request) *templates.KioskPicker {
	const (
		homeField = "kiosk-home"
		awayField = "kiosk-away"
	)

	var target, name, taken, keep string
	switch r.Header.Get("HX-Trigger") {
	case homeField:
		target, name = awayField, "away_id"
		taken, keep = r.FormValue("home_id"), r.FormValue("away_id")
	case awayField:
		target, name = homeField, "home_id"
		taken, keep = r.FormValue("away_id"), r.FormValue("home_id")
	default:
		// A mode change rather than a player change. The two selects already
		// agree with each other, so leave them alone.
		return nil
	}

	players, err := s.store.Players().List(r.Context())
	if err != nil {
		s.log.ErrorContext(r.Context(), "rebuilding the kiosk picker failed", "error", err)
		return nil
	}

	options := make([]templates.OpponentOption, 0, len(players))
	for _, player := range players {
		id := player.ID.String()
		if id == taken {
			continue
		}
		options = append(options, templates.OpponentOption{
			ID:          id,
			DisplayName: player.DisplayName,
			Selected:    id == keep,
		})
	}

	return &templates.KioskPicker{ID: target, Name: name, Players: options}
}

// setsColumns names the two score columns for whichever form asked.
//
// The kiosk enters for two other people and sends both ids; everywhere else
// the left column is the reader. A name that cannot be resolved falls back to
// the generic word rather than to an error: a heading is worth having even
// when the picker is still empty, and this endpoint draws boxes, it does not
// decide anything.
func (s *Server) setsColumns(r *http.Request) (home, away string) {
	name := func(raw string) string {
		id, err := uuid.Parse(strings.TrimSpace(raw))
		if err != nil {
			return ""
		}
		player, err := s.store.Players().ByID(r.Context(), id)
		if err != nil {
			return ""
		}
		return player.DisplayName
	}

	if raw := r.FormValue("home_id"); raw != "" {
		home, away = templates.SideHome, templates.SideAway
		if n := name(raw); n != "" {
			home = n
		}
		if n := name(r.FormValue("away_id")); n != "" {
			away = n
		}
		return home, away
	}

	away = templates.SideAway
	if n := name(r.FormValue("opponent_id")); n != "" {
		away = n
	}
	return templates.SideYourself, away
}

// setsPrefix keeps the prefix to what the templates can produce. It ends up in
// an id and in a target selector, and while templ escapes the attribute, a
// selector built from arbitrary input is a sharp edge to leave lying around.
func setsPrefix(raw string) string {
	if raw == "" || len(raw) > 40 {
		return setsPrefixFallback
	}

	for _, c := range raw {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '-':
		default:
			return setsPrefixFallback
		}
	}
	return raw
}
