package server

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/auth"
	"github.com/stuttgart-things/schmetterpause/internal/domain"
	"github.com/stuttgart-things/schmetterpause/internal/match"
	"github.com/stuttgart-things/schmetterpause/internal/scoring"
	"github.com/stuttgart-things/schmetterpause/internal/templates"
	"github.com/stuttgart-things/schmetterpause/internal/tournament"
)

// tournamentListLimit bounds the list of tournaments. An office plays a
// handful a year; the cap is here so the page cannot grow without bound
// rather than because anybody will reach it.
const tournamentListLimit = 50

// maxTournamentPlayers caps the field.
//
// Not a technical limit — the circle method does not care — but a table-time
// one. Twelve players is 66 matches, sixteen and a half hours at a quarter of
// an hour each (#41). A form that lets somebody tick twenty names produces a
// tournament nobody can finish, and the number is easier to argue with before
// the draw than after.
const maxTournamentPlayers = 12

// tournamentIDFrom reads the bracket a result belongs to out of the form.
//
// Absent, empty or unparseable all mean "no tournament" rather than an error.
// The field is optional on both entry paths — the kiosk and, since ADR-0010, a
// player's own phone — and a malformed one is a request nobody made on purpose:
// refusing the whole result over it would lose a match somebody just played.
func tournamentIDFrom(r *http.Request) *uuid.UUID {
	raw := strings.TrimSpace(r.FormValue("tournament_id"))
	if raw == "" {
		return nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return nil
	}
	return &id
}

// modeFrom reads the mode a tournament is to be played in.
//
// A missing or unreadable value is the default rather than a refusal: the
// form always sends both, so anything else is a request nobody made on
// purpose, and Known() below catches a value that is there but not allowed.
func modeFrom(r *http.Request) match.Mode {
	mode := match.Mode{
		BestOf:      match.DefaultBestOf,
		PointsToWin: match.PointsToEleven,
	}
	if v, err := strconv.Atoi(strings.TrimSpace(r.FormValue("best_of"))); err == nil {
		mode.BestOf = v
	}
	if v, err := strconv.Atoi(strings.TrimSpace(r.FormValue("points_to_win"))); err == nil {
		mode.PointsToWin = v
	}
	return mode
}

// itoa is strconv.Itoa under a shorter name, because the score strings below
// would otherwise be more package qualifier than content.
func itoa(n int) string { return strconv.Itoa(n) }

// handleTournaments serves the list and the form that starts a new one.
func (s *Server) handleTournaments(w http.ResponseWriter, r *http.Request) {
	view, err := s.tournamentListView(r.Context())
	if err != nil {
		s.log.ErrorContext(r.Context(), "loading tournaments failed", "error", err)
		http.Error(w, "Turniere nicht verfügbar", http.StatusInternalServerError)
		return
	}
	s.render(w, r, templates.TournamentList(view))
}

// handleCreateTournament starts one.
//
// The field is shuffled here rather than in tournament.Draw: the draw is pure
// and deterministic over the order it is given, which is what makes it
// testable, so the randomness lives at the edge that owns the request.
func (s *Server) handleCreateTournament(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		name = "Schnelles Turnier"
	}

	var field []uuid.UUID
	for _, raw := range r.Form["player_id"] {
		id, err := uuid.Parse(strings.TrimSpace(raw))
		if err != nil {
			continue
		}
		if !slices.Contains(field, id) {
			field = append(field, id)
		}
	}

	// The mode is settled here, once, and every pairing in the draw is played
	// under it. A control per pairing would ask the same question twenty-eight
	// times; a draw with no mode at all was the state before, and it left the
	// schedule unable to say over how many sets the evening was decided.
	mode := modeFrom(r)
	if !mode.Known() {
		s.rejectTournament(w, r, name, field, mode,
			"Diesen Modus gibt es nicht.")
		return
	}

	if msg := checkField(field); msg != "" {
		s.rejectTournament(w, r, name, field, mode, msg)
		return
	}

	// Whoever set it up. A tournament with no author is one nobody will
	// admit to having made when the pairings are wrong.
	author := field[0]
	if id, ok := auth.PlayerID(ctx); ok {
		author = id
	}

	rand.Shuffle(len(field), func(i, j int) { field[i], field[j] = field[j], field[i] })

	created, err := s.store.Tournaments().Create(ctx, domain.Tournament{
		Name:        name,
		Format:      domain.TournamentRoundRobin,
		CreatedBy:   author,
		BestOf:      mode.BestOf,
		PointsToWin: mode.PointsToWin,
		Players:     field,
	})
	if err != nil {
		s.log.ErrorContext(ctx, "creating the tournament failed", "error", err)
		s.rejectTournament(w, r, name, field, mode, "Das hat gerade nicht geklappt.")
		return
	}

	s.log.InfoContext(ctx, "tournament created",
		"tournament_id", created.ID, "players", len(created.Players),
		"best_of", created.BestOf, "points_to_win", created.PointsToWin)

	http.Redirect(w, r, "/tournaments/"+created.ID.String(), http.StatusSeeOther)
}

// checkField explains why a field cannot play, in words somebody can act on,
// or returns "" when it can.
func checkField(field []uuid.UUID) string {
	switch {
	case len(field) < 2:
		return "Mindestens zwei Spieler, sonst gibt es nichts zu spielen."
	case len(field) > maxTournamentPlayers:
		return "Höchstens " + itoa(maxTournamentPlayers) + " Spieler. " +
			"Mehr sind rechnerisch ein ganzer Tag an einer Platte."
	default:
		return ""
	}
}

// handleTournament serves one tournament: the draw, and the table so far.
func (s *Server) handleTournament(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Turnier nicht gefunden", http.StatusNotFound)
		return
	}

	// Only the copy served from under /kiosk can see the kiosk cookie at
	// all, so only that one offers entry. Asking kioskUnlocked on the public
	// path would always answer false, which is the bug this shape removes
	// rather than works around.
	view, err := s.tournamentView(r.Context(), id, s.kioskUnlocked(r))
	switch {
	case errors.Is(err, domain.ErrNotFound):
		http.Error(w, "Turnier nicht gefunden", http.StatusNotFound)
		return
	case err != nil:
		s.log.ErrorContext(r.Context(), "loading the tournament failed", "error", err)
		http.Error(w, "Turnier nicht verfügbar", http.StatusInternalServerError)
		return
	}

	// Which of the two copies this is. A reader who cannot enter here needs a
	// different sentence depending on it: the way to the entry view, or the
	// news that this device is not the one at the table.
	view.OnKioskPath = strings.HasPrefix(r.URL.Path, "/kiosk/")

	// The complaint from a refused entry, carried here through the redirect
	// rather than rendered in place, so a reload cannot re-submit a result.
	view.Error = r.URL.Query().Get("fehler")

	s.render(w, r, templates.TournamentPage(view))
}

// handleCloseTournament marks one over.
//
// Nothing about the rating hangs on this — tournament matches settle one at a
// time (docs/adr/0009) — so it takes the tournament off the list of things
// still happening and nothing else. That is also why it does not refuse an
// unfinished one: a tournament with one match nobody ever plays should not
// stay open forever.
func (s *Server) handleCloseTournament(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Turnier nicht gefunden", http.StatusNotFound)
		return
	}

	if err := s.store.Tournaments().Close(r.Context(), id, time.Now()); err != nil {
		s.log.ErrorContext(r.Context(), "closing the tournament failed", "error", err)
		http.Error(w, "Das hat gerade nicht geklappt.", http.StatusInternalServerError)
		return
	}
	s.log.InfoContext(r.Context(), "tournament closed", "tournament_id", id)

	http.Redirect(w, r, "/tournaments/"+id.String(), http.StatusSeeOther)
}

// handleTournamentRecord enters a result for one pairing of the draw.
//
// Kiosk-only, and that is the design rather than a restriction that got left
// in: a quick tournament is run by one person on the machine at the table
// (docs/turnier-vor-ort.md), and those entries settle at once. Twenty-eight
// matches each waiting on the opponent's tap would be the evening, which is
// open point 1 in #41 answered rather than deferred.
//
// It has an endpoint of its own instead of posting to /kiosk/matches, for one
// concrete reason: the kiosk answers with the kiosk page, and somebody who
// entered a result from the draw has to land back on the draw.
func (s *Server) handleTournamentRecord(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Turnier nicht gefunden", http.StatusNotFound)
		return
	}
	if !s.kioskUnlocked(r) {
		http.Error(w, "Zugang nötig", http.StatusForbidden)
		return
	}

	tour, err := s.store.Tournaments().ByID(ctx, id)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		http.Error(w, "Turnier nicht gefunden", http.StatusNotFound)
		return
	case err != nil:
		s.log.ErrorContext(ctx, "loading the tournament failed", "error", err)
		http.Error(w, "Turnier nicht verfügbar", http.StatusInternalServerError)
		return
	}
	if !tour.Open() {
		s.tournamentBack(w, r, id, true, "Das Turnier ist beendet.")
		return
	}

	homeID, awayID, msg := parseKioskPlayers(r)
	form, setsMsg := parseResultForm(r)
	if msg == "" {
		msg = setsMsg
	}
	if msg != "" {
		s.tournamentBack(w, r, id, true, msg)
		return
	}

	// Both have to be in the field. Without this the endpoint would book any
	// two players into somebody else's tournament, and the table would grow
	// a row the draw never had. Same sentence as the player path, from the
	// same check, so the two cannot drift apart.
	if !slices.Contains(tour.Players, homeID) || !slices.Contains(tour.Players, awayID) {
		s.tournamentBack(w, r, id, true, "Diese beiden spielen nicht in diesem Turnier.")
		return
	}

	// The mode comes from the tournament, not from the form. The hidden
	// fields carry it back correctly, but a result booked into a draw under a
	// mode the draw was not played in is a table that lies about itself —
	// and the form is the one part of this a caller can edit.
	form.result.Mode = match.Mode{BestOf: tour.BestOf, PointsToWin: tour.PointsToWin}

	_, err = scoring.Record(ctx, s.store, homeID, awayID, form.result,
		domain.EnteredViaKiosk, &tour.ID, time.Now())

	var rejection *match.Rejection
	switch {
	case err == nil:
	case errors.Is(err, scoring.ErrSamePlayer):
		s.tournamentBack(w, r, id, true, "Zwei verschiedene Spieler, bitte.")
		return
	case errors.Is(err, domain.ErrNotFound):
		s.tournamentBack(w, r, id, true, "Diesen Spieler gibt es nicht.")
		return
	case errors.As(err, &rejection):
		s.tournamentBack(w, r, id, true, describeRejection(err))
		return
	default:
		s.log.ErrorContext(ctx, "recording the tournament match failed", "error", err)
		s.tournamentBack(w, r, id, true, "Das hat gerade nicht geklappt.")
		return
	}

	s.log.InfoContext(ctx, "tournament match recorded",
		"tournament_id", id, "home", homeID, "away", awayID)

	s.tournamentBack(w, r, id, true, "")
}

// tournamentBack returns to the draw, carrying a complaint in the query when
// there is one.
//
// A redirect rather than rendering in place, so a reload after entering a
// result does not offer to enter it again — the mistake that a settled result
// makes expensive, because a kiosk entry counts immediately.
func (s *Server) tournamentBack(w http.ResponseWriter, r *http.Request,
	id uuid.UUID, kiosk bool, msg string,
) {
	target := tournamentPath(id, kiosk)
	if msg != "" {
		target += "?fehler=" + url.QueryEscape(msg)
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

func (s *Server) tournamentListView(ctx context.Context) (templates.TournamentListView, error) {
	tours, err := s.store.Tournaments().List(ctx, tournamentListLimit)
	if err != nil {
		return templates.TournamentListView{}, err
	}

	players, err := s.store.Players().List(ctx)
	if err != nil {
		return templates.TournamentListView{}, err
	}

	view := templates.TournamentListView{
		Header:     s.headerView(ctx),
		MaxPlayers: maxTournamentPlayers,
		Form:       templates.NewTournamentFormView(candidates(players, nil)),
	}
	for _, t := range tours {
		view.Tournaments = append(view.Tournaments, tournamentRow(t))
	}
	return view, nil
}

// openTournaments are the ones a result can still go into, newest first.
//
// The kiosk shows these because it is the only page that can reach the entry
// view: the grant cookie is scoped to /kiosk, so a page outside it cannot
// know it is the machine at the table and cannot offer the link. Without this
// list the way in is somebody copying a UUID out of the address bar (#124).
func (s *Server) openTournaments(ctx context.Context) ([]templates.TournamentListRow, error) {
	tours, err := s.store.Tournaments().List(ctx, tournamentListLimit)
	if err != nil {
		return nil, fmt.Errorf("list tournaments: %w", err)
	}

	var rows []templates.TournamentListRow
	for _, t := range tours {
		if !t.Open() {
			continue
		}
		rows = append(rows, tournamentRow(t))
	}
	return rows, nil
}

// tournamentRow is one tournament as a list entry. Both lists that show one
// say the same three things about it, so they say them the same way.
func tournamentRow(t domain.Tournament) templates.TournamentListRow {
	return templates.TournamentListRow{
		ID:      t.ID.String(),
		Name:    t.Name,
		Open:    t.Open(),
		Players: len(t.Players),
		Matches: tournament.Matches(len(t.Players)),
		Mode:    templates.TournamentModeLabel(t.BestOf, t.PointsToWin),
	}
}

// tournamentPath is where a draw is read, on the copy the reader came from.
// Sending somebody back to the other one would take away either the entry
// boxes or the grant that lets them appear.
func tournamentPath(id uuid.UUID, kiosk bool) string {
	if kiosk {
		return "/kiosk/tournaments/" + id.String()
	}
	return "/tournaments/" + id.String()
}

// tournamentPairing reads a result that came from a draw: which tournament,
// and the pairing in the order the schedule shows it.
//
// Returns a nil tournament and no message when the form named none, which is
// every ordinary break-time result.
func (s *Server) tournamentPairing(r *http.Request, self uuid.UUID) (
	tour *domain.Tournament, home, away uuid.UUID, msg string,
) {
	if tournamentIDFrom(r) == nil {
		return nil, uuid.Nil, uuid.Nil, ""
	}

	home, away, msg = parseKioskPlayers(r)
	if msg != "" {
		return nil, uuid.Nil, uuid.Nil, msg
	}
	// A player enters their own result. Entering one for two other people is
	// what the machine at the table is for, and it settles at once because
	// somebody is standing there — which is exactly what nobody can check
	// about a result typed on a phone across the room.
	if self != home && self != away {
		return nil, uuid.Nil, uuid.Nil, "In dieser Paarung spielst du nicht mit."
	}

	tour, msg = s.tournamentEntry(r.Context(), r, home, away)
	return tour, home, away, msg
}

// tournamentEntry is a tournament a result is being booked into, or nil when
// the form named none.
//
// Everything a caller has to be told before a result may go in lives here, so
// the two entry paths cannot drift apart on it: the tournament exists, it is
// still open, and both players are actually in the field. Without the last
// check the endpoint would book any two people into somebody else's draw, and
// the table would grow a row the schedule never had.
func (s *Server) tournamentEntry(ctx context.Context, r *http.Request,
	home, away uuid.UUID,
) (*domain.Tournament, string) {
	id := tournamentIDFrom(r)
	if id == nil {
		return nil, ""
	}

	tour, err := s.store.Tournaments().ByID(ctx, *id)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return nil, "Dieses Turnier gibt es nicht."
	case err != nil:
		s.log.ErrorContext(ctx, "loading the tournament failed", "error", err)
		return nil, "Das hat gerade nicht geklappt."
	case !tour.Open():
		return nil, "Das Turnier ist beendet."
	case !slices.Contains(tour.Players, home) || !slices.Contains(tour.Players, away):
		return nil, "Diese beiden spielen nicht in diesem Turnier."
	}
	return &tour, ""
}

// rejectTournament renders the form again with what was typed and why it was
// refused, so nobody re-ticks eight names.
func (s *Server) rejectTournament(w http.ResponseWriter, r *http.Request,
	name string, chosen []uuid.UUID, mode match.Mode, msg string,
) {
	players, err := s.store.Players().List(r.Context())
	if err != nil {
		s.log.ErrorContext(r.Context(), "loading players failed", "error", err)
		http.Error(w, "Turniere nicht verfügbar", http.StatusInternalServerError)
		return
	}

	form := templates.NewTournamentFormView(candidates(players, chosen))
	form.Name = name
	form.Error = msg
	// The mode comes back as it was picked, even when it is the reason for
	// the refusal: a select that snaps back to the default hides what was
	// wrong with the answer.
	form.BestOf = mode.BestOf
	form.PointsToWin = mode.PointsToWin

	view, err := s.tournamentListView(r.Context())
	if err != nil {
		s.log.ErrorContext(r.Context(), "loading tournaments failed", "error", err)
		http.Error(w, "Turniere nicht verfügbar", http.StatusInternalServerError)
		return
	}
	view.Form = form

	w.WriteHeader(http.StatusUnprocessableEntity)
	s.render(w, r, templates.TournamentList(view))
}

func candidates(players []domain.Player, chosen []uuid.UUID) []templates.TournamentCandidate {
	out := make([]templates.TournamentCandidate, 0, len(players))
	for _, p := range players {
		out = append(out, templates.TournamentCandidate{
			ID:          p.ID.String(),
			DisplayName: p.DisplayName,
			Chosen:      slices.Contains(chosen, p.ID),
		})
	}
	return out
}

func (s *Server) tournamentView(ctx context.Context, id uuid.UUID, kiosk bool) (templates.TournamentView, error) {
	tour, err := s.store.Tournaments().ByID(ctx, id)
	if err != nil {
		return templates.TournamentView{}, err
	}

	booked, err := s.store.Tournaments().Matches(ctx, id)
	if err != nil {
		return templates.TournamentView{}, err
	}

	players, err := s.store.Players().List(ctx)
	if err != nil {
		return templates.TournamentView{}, err
	}
	names := make(map[uuid.UUID]string, len(players))
	for _, p := range players {
		names[p.ID] = p.DisplayName
	}

	// Who is reading. A signed-in player may enter their own pairings from
	// their own device; anybody else reads the draw.
	self, signedIn := auth.PlayerID(ctx)

	view := templates.TournamentView{
		Header:      s.headerView(ctx),
		ID:          tour.ID.String(),
		Name:        tour.Name,
		Open:        tour.Open(),
		BestOf:      tour.BestOf,
		PointsToWin: tour.PointsToWin,
		CanEnter:    tour.Open() && kiosk,
		Total:       tournament.Matches(len(tour.Players)),
	}

	// What has already been played, so a pairing can say so instead of
	// offering a form for a match that happened an hour ago.
	played := make(map[[2]uuid.UUID]domain.Match, len(booked))
	for _, m := range booked {
		played[pairKey(m.HomeID, m.AwayID)] = m
		if m.Status == domain.MatchConfirmed {
			view.Played++
		}
	}

	for _, round := range tournament.Draw(tour.Players) {
		rv := templates.TournamentRoundView{No: round.No}
		if round.Bye != uuid.Nil {
			rv.Bye = names[round.Bye]
		}
		for _, p := range round.Pairings {
			pv := templates.TournamentPairingView{
				HomeID:   p.Home.String(),
				AwayID:   p.Away.String(),
				HomeName: names[p.Home],
				AwayName: names[p.Away],
			}
			if m, ok := played[pairKey(p.Home, p.Away)]; ok {
				pv.Result = describeResult(m, p.Home)
				pv.Pending = m.Status == domain.MatchPending
				pv.Disputed = m.Status == domain.MatchDisputed
			} else if signedIn && tour.Open() && (p.Home == self || p.Away == self) {
				pv.CanReport = true
				view.CanReport = true
			}
			rv.Pairings = append(rv.Pairings, pv)
		}
		view.Rounds = append(view.Rounds, rv)
	}

	for _, row := range tournament.Table(tour.Players, booked) {
		view.Table = append(view.Table, templates.TournamentTableRow{
			Rank:        row.Rank,
			Shared:      row.Shared,
			DisplayName: names[row.PlayerID],
			Played:      row.Played,
			Won:         row.Won,
			Lost:        row.Lost,
			Sets:        itoa(row.SetsWon) + ":" + itoa(row.SetsLost),
			SetDiff:     row.SetDiff(),
		})
	}
	return view, nil
}

// pairKey names an encounter regardless of orientation, so a result entered
// with the sides the other way round still finds its pairing. Which is the
// ordinary case: the draw alternates the orientation by round, and nobody at
// the table checks which name the app printed first.
func pairKey(a, b uuid.UUID) [2]uuid.UUID {
	if a.String() > b.String() {
		a, b = b, a
	}
	return [2]uuid.UUID{a, b}
}

// describeResult states a played pairing from home's side, since that is the
// order the draw prints the two names in.
func describeResult(m domain.Match, home uuid.UUID) string {
	var homeSets, awaySets int
	for _, set := range m.Sets {
		switch {
		case set.HomePoints > set.AwayPoints:
			homeSets++
		case set.AwayPoints > set.HomePoints:
			awaySets++
		}
	}
	if m.HomeID != home {
		homeSets, awaySets = awaySets, homeSets
	}
	return itoa(homeSets) + ":" + itoa(awaySets)
}
