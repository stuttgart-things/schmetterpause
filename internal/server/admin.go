package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

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

	// The second question this page answers, and the one issue #77 filed:
	// which machines are kiosks right now. A derived cookie could not answer
	// it, because it was the same value everywhere.
	grants, err := s.store.KioskGrants().Active(r.Context(), time.Now())
	if err != nil {
		s.log.ErrorContext(r.Context(), "loading the kiosk grants failed", "error", err)
		http.Error(w, "Liste nicht verfügbar", http.StatusInternalServerError)
		return
	}
	// Names for the operators the grants point at. One list rather than a
	// lookup per row: the page already holds every player for the flag list
	// above, and a kiosk evening has a handful of machines at most.
	players, err := s.store.Players().List(r.Context())
	if err != nil {
		s.log.ErrorContext(r.Context(), "loading the players failed", "error", err)
		http.Error(w, "Liste nicht verfügbar", http.StatusInternalServerError)
		return
	}
	names := make(map[uuid.UUID]string, len(players))
	for _, p := range players {
		names[p.ID] = p.DisplayName
	}

	view.Kiosks = kioskGrantViews(grants, names)

	s.render(w, r, templates.Admin(view))
}

// kioskGrantViews puts the grants into the words the page uses.
func kioskGrantViews(
	grants []domain.KioskGrant, names map[uuid.UUID]string,
) []templates.KioskGrantView {
	views := make([]templates.KioskGrantView, 0, len(grants))
	for _, g := range grants {
		views = append(views, templates.KioskGrantView{
			ID:        g.ID.String(),
			UserAgent: g.UserAgent,
			Unlocked:  g.CreatedAt.Local().Format("02.01.2006 15:04"),
			LastSeen:  g.LastSeenAt.Local().Format("02.01.2006 15:04"),
			Expires:   g.ExpiresAt.Local().Format("02.01.2006 15:04"),
			Operator:  operatorLabel(g, names),
		})
	}
	return views
}

// operatorLabel is who a machine says is typing. Empty when it has not been
// asked yet, or when the player it named has since been removed — the column
// is set null on delete, and either way the machine cannot write anything
// until somebody answers again (issue #90).
func operatorLabel(g domain.KioskGrant, names map[uuid.UUID]string) string {
	if g.OperatorID == nil {
		return ""
	}
	return names[*g.OperatorID]
}

// handleRevokeKiosk takes one machine back.
//
// The thing the old cookie could not do. It was base64(HMAC(session key,
// "kiosk:" + token)) — a constant — so revoking one browser meant changing
// the token and restarting, which logged out the laptop at the table along
// with everybody else (issue #77).
func (s *Server) handleRevokeKiosk(w http.ResponseWriter, r *http.Request) {
	self, _ := auth.PlayerID(r.Context())

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Dieses Gerät gibt es nicht", http.StatusNotFound)
		return
	}

	if err := s.store.KioskGrants().Revoke(r.Context(), id, time.Now()); err != nil {
		s.log.ErrorContext(r.Context(), "revoking the kiosk grant failed",
			"grant_id", id, "error", err)
		http.Error(w, "Das hat gerade nicht geklappt", http.StatusInternalServerError)
		return
	}

	// Named, because that is the half of #77 that is not about revocation:
	// the log says who took it back, not that "the kiosk" did something.
	s.log.InfoContext(r.Context(), "kiosk grant revoked", "grant_id", id, "by", self)

	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

// handleRevokeAllKiosks takes every unlocked machine back at once.
//
// The answer to "somebody read the token over a shoulder" that does not
// involve a restart. It logs out the laptop at the table too, on purpose —
// whoever presses this wants exactly that, and the laptop gets back in by
// entering the token again.
func (s *Server) handleRevokeAllKiosks(w http.ResponseWriter, r *http.Request) {
	self, _ := auth.PlayerID(r.Context())

	n, err := s.store.KioskGrants().RevokeAll(r.Context(), time.Now())
	if err != nil {
		s.log.ErrorContext(r.Context(), "revoking all kiosk grants failed", "error", err)
		http.Error(w, "Das hat gerade nicht geklappt", http.StatusInternalServerError)
		return
	}

	s.log.InfoContext(r.Context(), "all kiosk grants revoked", "count", n, "by", self)

	http.Redirect(w, r, "/admin", http.StatusSeeOther)
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
