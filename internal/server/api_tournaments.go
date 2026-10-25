package server

import (
	"errors"
	"net/http"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
	"github.com/stuttgart-things/schmetterpause/internal/tournament"
)

// What the Zählwerk reads about tournaments (docs/adr/0028): the open ones,
// and the places in them nobody has filled. Read-only, like the player list —
// a tournament is set up by a person on the tournament page, and the draw is
// computed here and nowhere else.

// apiTournament is one entry of GET /api/tournaments.
//
// The mode is what the Zählwerk counts by: a result in any other mode is
// refused rather than rewritten. The sides name the halves of the table, side
// A being home and the left half there. CreatedAt lets a caller play the
// oldest first without relying on the order of the list.
type apiTournament struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	BestOf      int       `json:"best_of"`
	PointsToWin int       `json:"points_to_win"`
	SideA       string    `json:"side_a"`
	SideB       string    `json:"side_b"`
	Rated       bool      `json:"rated"`
	Matches     int       `json:"matches"`
	Played      int       `json:"played"`
	CreatedAt   time.Time `json:"created_at"`
}

// apiPlace is one entry of GET /api/tournaments/{id}/pairings: a place in the
// draw that a result may still fill. Home is side A (docs/adr/0013).
type apiPlace struct {
	Round    int       `json:"round"`
	Final    bool      `json:"final"`
	HomeID   uuid.UUID `json:"home_id"`
	AwayID   uuid.UUID `json:"away_id"`
	HomeName string    `json:"home_name"`
	AwayName string    `json:"away_name"`
}

// handleAPITournaments lists the open tournaments, oldest first.
//
// Closed ones are left out: once a tournament is closed there is nothing the
// Zählwerk could still enter into it.
func (s *Server) handleAPITournaments(w http.ResponseWriter, r *http.Request) {
	if !s.scoreboardAuthorized(r) {
		s.apiRefuse(w, r, http.StatusUnauthorized, "a bearer token is required")
		return
	}
	ctx := r.Context()

	tours, err := s.store.Tournaments().List(ctx, tournamentListLimit)
	if err != nil {
		s.log.ErrorContext(ctx, "listing tournaments for the api failed", "error", err)
		s.apiRefuse(w, r, http.StatusInternalServerError, "the tournament list is not available")
		return
	}

	out := make([]apiTournament, 0, len(tours))
	for _, t := range tours {
		if !t.Open() {
			continue
		}
		booked, err := s.store.Tournaments().Matches(ctx, t.ID)
		if err != nil {
			s.log.ErrorContext(ctx, "loading a tournament's matches for the api failed",
				"tournament_id", t.ID, "error", err)
			s.apiRefuse(w, r, http.StatusInternalServerError, "the tournament list is not available")
			return
		}
		played := 0
		for _, m := range booked {
			if m.Status == domain.MatchConfirmed {
				played++
			}
		}
		out = append(out, apiTournament{
			ID:          t.ID,
			Name:        t.Name,
			BestOf:      t.BestOf,
			PointsToWin: t.PointsToWin,
			SideA:       t.SideA,
			SideB:       t.SideB,
			Rated:       t.Rated,
			Matches:     tournament.Matches(len(t.Players), t.Format.Legs(), t.WithFinal),
			Played:      played,
			CreatedAt:   t.CreatedAt,
		})
	}
	// The repository lists newest first, which is how a person reads the
	// tournament page. A machine working through them wants the one that has
	// waited longest.
	slices.SortStableFunc(out, func(a, b apiTournament) int { return a.CreatedAt.Compare(b.CreatedAt) })
	s.writeJSON(w, r, http.StatusOK, out)
}

// handleAPIPairings lists the open places of one tournament in the order of
// the draw, the final last once the table decides it.
//
// These are exactly the places POST /api/results accepts: both come from
// tournament.OpenPlaces. A closed tournament has none, and says so with an
// empty list rather than an error.
func (s *Server) handleAPIPairings(w http.ResponseWriter, r *http.Request) {
	if !s.scoreboardAuthorized(r) {
		s.apiRefuse(w, r, http.StatusUnauthorized, "a bearer token is required")
		return
	}
	ctx := r.Context()

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.apiRefuse(w, r, http.StatusNotFound, "no such tournament")
		return
	}
	tour, err := s.store.Tournaments().ByID(ctx, id)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		s.apiRefuse(w, r, http.StatusNotFound, "no such tournament")
		return
	case err != nil:
		s.log.ErrorContext(ctx, "loading a tournament for the api failed", "tournament_id", id, "error", err)
		s.apiRefuse(w, r, http.StatusInternalServerError, "the pairings are not available")
		return
	}

	out := []apiPlace{}
	if !tour.Open() {
		s.writeJSON(w, r, http.StatusOK, out)
		return
	}

	booked, err := s.store.Tournaments().Matches(ctx, id)
	if err != nil {
		s.log.ErrorContext(ctx, "loading a tournament's matches for the api failed", "tournament_id", id, "error", err)
		s.apiRefuse(w, r, http.StatusInternalServerError, "the pairings are not available")
		return
	}
	players, err := s.store.Players().List(ctx)
	if err != nil {
		s.log.ErrorContext(ctx, "listing players for the api failed", "error", err)
		s.apiRefuse(w, r, http.StatusInternalServerError, "the pairings are not available")
		return
	}
	names := make(map[uuid.UUID]string, len(players))
	for _, p := range players {
		names[p.ID] = p.DisplayName
	}

	for _, p := range tournament.OpenPlaces(fieldOf(tour), booked) {
		out = append(out, apiPlace{
			Round:    p.Round,
			Final:    p.Final,
			HomeID:   p.Pairing.Home,
			AwayID:   p.Pairing.Away,
			HomeName: names[p.Pairing.Home],
			AwayName: names[p.Pairing.Away],
		})
	}
	s.writeJSON(w, r, http.StatusOK, out)
}
