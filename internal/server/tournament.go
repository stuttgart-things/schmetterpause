package server

import (
	"context"
	"errors"
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
// The field is optional on every entry path, and a malformed one is a request
// nobody made on purpose — refusing the whole result over it would lose a
// match somebody just played.
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

	if msg := checkField(field); msg != "" {
		s.rejectTournament(w, r, name, field, msg)
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
		Name:      name,
		Format:    domain.TournamentRoundRobin,
		CreatedBy: author,
		Players:   field,
	})
	if err != nil {
		s.log.ErrorContext(ctx, "creating the tournament failed", "error", err)
		s.rejectTournament(w, r, name, field, "Das hat gerade nicht geklappt.")
		return
	}

	s.log.InfoContext(ctx, "tournament created",
		"tournament_id", created.ID, "players", len(created.Players))

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
	// uuid.Nil for a browser nobody is signed in on: it reads the draw and
	// gets no boxes.
	viewer, _ := auth.PlayerID(r.Context())

	view, err := s.tournamentView(r.Context(), id, viewer, s.kioskUnlocked(r))
	switch {
	case errors.Is(err, domain.ErrNotFound):
		http.Error(w, "Turnier nicht gefunden", http.StatusNotFound)
		return
	case err != nil:
		s.log.ErrorContext(r.Context(), "loading the tournament failed", "error", err)
		http.Error(w, "Turnier nicht verfügbar", http.StatusInternalServerError)
		return
	}

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
	// a row the draw never had.
	if !slices.Contains(tour.Players, homeID) || !slices.Contains(tour.Players, awayID) {
		s.tournamentBack(w, r, id, true, "Diese beiden spielen nicht in diesem Turnier.")
		return
	}

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

// handleTournamentReport enters a result for a pairing the reporting player
// played in — from their own phone, with no kiosk involved.
//
// It is the ordinary path with a bracket attached: the match is recorded as
// pending and the opponent confirms it, exactly as a Tuesday-afternoon match
// is. That is what makes it safe to open to everybody, and it is the
// difference to the kiosk, where an entry counts at once because one person
// is typing for the room and twenty-eight confirmations would be the evening.
//
// The reporter must be one of the two. Without that check anybody could file
// results for other people's pairings, and the opponents would be left
// dismissing matches that never happened — issue #90's problem in a new
// place.
//
// entered_via stays 'player': this is somebody logging a match they played,
// which is the kind the Definition of Done counts (issue #71). A tournament
// entered from phones is still not voluntary logging, and SINCE remains the
// answer to that, as docs/turnier-vor-ort.md says.
func (s *Server) handleTournamentReport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Turnier nicht gefunden", http.StatusNotFound)
		return
	}
	self, ok := auth.PlayerID(ctx)
	if !ok {
		http.Error(w, "Anmeldung nötig", http.StatusForbidden)
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
		s.tournamentBack(w, r, id, false, "Das Turnier ist beendet.")
		return
	}

	homeID, awayID, msg := parseKioskPlayers(r)
	form, setsMsg := parseResultForm(r)
	if msg == "" {
		msg = setsMsg
	}
	if msg != "" {
		s.tournamentBack(w, r, id, false, msg)
		return
	}

	if !slices.Contains(tour.Players, homeID) || !slices.Contains(tour.Players, awayID) {
		s.tournamentBack(w, r, id, false, "Diese beiden spielen nicht in diesem Turnier.")
		return
	}
	if self != homeID && self != awayID {
		s.tournamentBack(w, r, id, false,
			"Dieses Spiel ist nicht deins. Eintragen darf es einer der beiden — "+
				"oder das Gerät an der Platte.")
		return
	}

	if _, err := match.Validate(form.result); err != nil {
		s.tournamentBack(w, r, id, false, describeRejection(err))
		return
	}

	sets := make([]domain.MatchSet, 0, len(form.result.Sets))
	for i, set := range form.result.Sets {
		sets = append(sets, domain.MatchSet{
			SetNo: i + 1, HomePoints: set.Home, AwayPoints: set.Away,
		})
	}

	created, err := s.store.Matches().Create(ctx, domain.Match{
		HomeID:       homeID,
		AwayID:       awayID,
		BestOf:       form.result.Mode.BestOf,
		PointsToWin:  form.result.Mode.PointsToWin,
		Status:       domain.MatchPending,
		ReportedBy:   self,
		PlayedAt:     time.Now(),
		EnteredVia:   domain.EnteredViaPlayer,
		TournamentID: &tour.ID,
		Sets:         sets,
	})
	if err != nil {
		s.log.ErrorContext(ctx, "recording the tournament match failed", "error", err)
		s.tournamentBack(w, r, id, false, "Das hat gerade nicht geklappt.")
		return
	}

	s.log.InfoContext(ctx, "tournament match reported",
		"tournament_id", id, "match_id", created.ID, "reported_by", self)

	s.tournamentBack(w, r, id, false, "")
}

// tournamentBack returns to the draw, carrying a complaint in the query when
// there is one.
//
// A redirect rather than rendering in place, so a reload after entering a
// result does not offer to enter it again — the mistake that a settled result
// makes expensive, because a kiosk entry counts immediately.
func (s *Server) tournamentBack(w http.ResponseWriter, r *http.Request, id uuid.UUID, kiosk bool, msg string) {
	target := "/tournaments/" + id.String()
	if kiosk {
		target = "/kiosk/tournaments/" + id.String()
	}
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
		view.Tournaments = append(view.Tournaments, templates.TournamentListRow{
			ID:      t.ID.String(),
			Name:    t.Name,
			Open:    t.Open(),
			Players: len(t.Players),
			Matches: tournament.Matches(len(t.Players)),
		})
	}
	return view, nil
}

// rejectTournament renders the form again with what was typed and why it was
// refused, so nobody re-ticks eight names.
func (s *Server) rejectTournament(w http.ResponseWriter, r *http.Request, name string, chosen []uuid.UUID, msg string) {
	players, err := s.store.Players().List(r.Context())
	if err != nil {
		s.log.ErrorContext(r.Context(), "loading players failed", "error", err)
		http.Error(w, "Turniere nicht verfügbar", http.StatusInternalServerError)
		return
	}

	form := templates.NewTournamentFormView(candidates(players, chosen))
	form.Name = name
	form.Error = msg

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

// tournamentView builds the page. viewer is the signed-in player or uuid.Nil,
// and kiosk says whether this is the copy served from under /kiosk.
//
// The two decide different things. kiosk decides how an entry settles — at
// once, because one person types for everybody and twenty-eight confirmations
// would be the evening. viewer decides which pairings offer a form at all: a
// player may enter the matches they played and nobody else's, which is the
// same rule the ordinary entry form follows.
func (s *Server) tournamentView(ctx context.Context, id, viewer uuid.UUID, kiosk bool) (templates.TournamentView, error) {
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

	view := templates.TournamentView{
		Header:   s.headerView(ctx),
		ID:       tour.ID.String(),
		Name:     tour.Name,
		Open:     tour.Open(),
		Kiosk:    kiosk,
		SignedIn: viewer != uuid.Nil,
		Total:    tournament.Matches(len(tour.Players)),
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
				pv.Pending = m.Status != domain.MatchConfirmed
			} else if tour.Open() {
				switch {
				case kiosk:
					pv.EntryAction = "/kiosk/tournaments/" + view.ID + "/matches"
					pv.Immediate = true
				case viewer == p.Home || viewer == p.Away:
					pv.EntryAction = "/tournaments/" + view.ID + "/matches"
				}
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
