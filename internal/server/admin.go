package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/auth"
	"github.com/stuttgart-things/schmetterpause/internal/domain"
	"github.com/stuttgart-things/schmetterpause/internal/templates"
)

// isAdmin answers auth.RequireAdmin. Nobody is an admin when the lookup
// fails: the error travels up and the request is refused, because an action
// on somebody else's record should not happen while we are unsure who asked.
func (s *Server) isAdmin(ctx context.Context, id uuid.UUID) (bool, error) {
	player, err := s.store.Players().ByID(ctx, id)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return false, nil
	case err != nil:
		return false, fmt.Errorf("load player %s: %w", id, err)
	}
	return player.IsAdmin, nil
}

// handleAdmin lists who may act for other people, and says what that means.
//
// The Definition of Done in issue #88 asks for exactly this half: "it is
// recorded who may act on someone else's behalf". Until now the answer was a
// cookie value identical in every browser that had ever opened the token URL,
// which records nothing about anybody.
//
// The boundary is written on the page rather than only in docs/adr/0008,
// because the person who wonders whether the kiosk may delete a result is
// standing in front of the application, not in front of the repository.
func (s *Server) handleAdmin(w http.ResponseWriter, r *http.Request) {
	admins, err := s.store.Players().Admins(r.Context())
	if err != nil {
		s.log.ErrorContext(r.Context(), "loading the admins failed", "error", err)
		http.Error(w, "Liste nicht verfügbar", http.StatusInternalServerError)
		return
	}

	self, _ := auth.PlayerID(r.Context())

	view := templates.AdminView{
		Header: s.headerView(r.Context()),
		People: make([]templates.AdminPerson, 0, len(admins)),
	}
	for _, p := range admins {
		view.People = append(view.People, templates.AdminPerson{
			ID:          p.ID.String(),
			DisplayName: p.DisplayName,
			IsSelf:      p.ID == self,
		})
	}
	s.render(w, r, templates.Admin(view))
}

// GrantBootstrapAdmin gives the flag to the player SP_BOOTSTRAP_ADMIN names.
//
// Run at every start, not only the first. Somebody who withdraws the flag
// from the last admin gets back in by restarting with the variable set, and
// not by opening psql — which is the price issue #73 puts on a flag and what
// docs/adr/0008 answers.
//
// A name nobody has is a warning, not a failure. The variable is often set
// before the person it names has joined, and refusing to start over it would
// turn a typo into an outage.
func (s *Server) GrantBootstrapAdmin(ctx context.Context) {
	name := s.cfg.BootstrapAdmin
	if name == "" {
		return
	}

	player, err := s.store.Players().ByDisplayName(ctx, name)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		s.log.WarnContext(ctx, "SP_BOOTSTRAP_ADMIN names a player who does not exist yet",
			"display_name", name)
		return
	case err != nil:
		s.log.ErrorContext(ctx, "looking up the bootstrap admin failed",
			"display_name", name, "error", err)
		return
	}

	if player.IsAdmin {
		return
	}

	if err := s.store.Players().SetAdmin(ctx, player.ID, true); err != nil {
		s.log.ErrorContext(ctx, "granting the bootstrap admin failed",
			"player_id", player.ID, "error", err)
		return
	}

	// Every change to who may act for other people leaves a line naming the
	// person. Without that the flag would be the kiosk's mistake under a new
	// name (docs/adr/0008).
	s.log.InfoContext(ctx, "admin flag granted from SP_BOOTSTRAP_ADMIN",
		"player_id", player.ID, "display_name", player.DisplayName)
}
