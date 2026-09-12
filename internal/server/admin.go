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
	"github.com/stuttgart-things/schmetterpause/internal/scoring"
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
	s.renderAdmin(w, r, "")
}

// adminRecentMatches is how far back the removable results go.
//
// Long enough to hold the evening somebody is asking about, short enough that
// the page stays a list rather than a history — /matches is the history. The
// guard in scoring.Remove refuses everything but the newest result per player
// anyway, so a longer list would mostly be rows whose button says no.
const adminRecentMatches = 20

// renderAdmin draws the page, with note as what just happened.
func (s *Server) renderAdmin(w http.ResponseWriter, r *http.Request, note string) {
	s.renderAdminWith(w, r, note, "", http.StatusOK)
}

// rejectAdmin draws it with a refusal instead, and says so in the status. The
// same split the kiosk makes: "entfernt" and "geht nicht" are opposite
// outcomes, and a page that answers 200 to both tells a script they are one.
func (s *Server) rejectAdmin(w http.ResponseWriter, r *http.Request, msg string) {
	s.renderAdminWith(w, r, "", msg, http.StatusUnprocessableEntity)
}

func (s *Server) renderAdminWith(
	w http.ResponseWriter, r *http.Request, note, refusal string, status int,
) {
	view, err := s.adminView(r.Context(), note, refusal)
	if err != nil {
		s.log.ErrorContext(r.Context(), "building the admin page failed", "error", err)
		http.Error(w, "Liste nicht verfügbar", http.StatusInternalServerError)
		return
	}
	if status != http.StatusOK {
		w.WriteHeader(status)
	}
	s.render(w, r, templates.Admin(view))
}

// adminView is everything the page says, in one place because two handlers
// draw it: opening it, and coming back from a removal with something to say.
func (s *Server) adminView(ctx context.Context, note, refusal string) (templates.AdminView, error) {
	admins, err := s.store.Players().Admins(ctx)
	if err != nil {
		return templates.AdminView{}, fmt.Errorf("load the admins: %w", err)
	}

	self, _ := auth.PlayerID(ctx)

	view := templates.AdminView{
		Header: s.headerView(ctx),
		Note:   note,
		Error:  refusal,
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
	grants, err := s.store.KioskGrants().Active(ctx, time.Now())
	if err != nil {
		return templates.AdminView{}, fmt.Errorf("load the kiosk grants: %w", err)
	}
	// Names for the operators the grants point at. One list rather than a
	// lookup per row: the page already holds every player for the flag list
	// above, and a kiosk evening has a handful of machines at most.
	players, err := s.store.Players().List(ctx)
	if err != nil {
		return templates.AdminView{}, fmt.Errorf("load the players: %w", err)
	}
	names := make(map[uuid.UUID]string, len(players))
	for _, p := range players {
		names[p.ID] = p.DisplayName
	}

	view.Kiosks = kioskGrantViews(grants, names)

	matches, err := s.store.Matches().Recent(ctx, adminRecentMatches)
	if err != nil {
		return templates.AdminView{}, fmt.Errorf("load the recent matches: %w", err)
	}
	view.Matches = adminMatchRows(matches, names)

	return view, nil
}

// adminMatchRows keeps the settled results and puts them in the words the
// match list already uses.
//
// Pending and contested ones are dropped rather than shown greyed out: the
// flag is for results nobody can fix any other way, and those two have a
// path that does not need it — which is the boundary docs/adr/0008 draws.
func adminMatchRows(matches []domain.Match, names map[uuid.UUID]string) []templates.AdminMatchRow {
	rows := make([]templates.AdminMatchRow, 0, len(matches))
	for _, m := range matches {
		if m.Status != domain.MatchConfirmed {
			continue
		}
		// Built by matchListRow rather than beside it: which side won is
		// read off the set scores, and one copy of that is enough.
		row := matchListRow(m, names, nil, uuid.Nil)
		rows = append(rows, templates.AdminMatchRow{
			ID:         m.ID.String(),
			PlayedAt:   row.PlayedAt,
			WinnerName: row.WinnerName,
			LoserName:  row.LoserName,
			WinnerSets: row.WinnerSets,
			LoserSets:  row.LoserSets,
			Sets:       row.Sets,
		})
	}
	return rows
}

// handleAdminRemoveMatch takes a counted result back, with both ratings.
//
// The one thing issue #105 puts first, and the reason it does: the office
// week put real results in the database, and until now the only way to fix a
// wrong one was SQL against the database the office plays on — which issue
// #163 showed is easy to point at the wrong target.
//
// It renders rather than redirects, the way the kiosk answers its own undo.
// Two of the three outcomes are a sentence somebody has to read — above all
// ErrNotLast, which is not a failure but an instruction — and a redirect
// would drop them. Re-posting after a reload is harmless: the match is gone,
// and the second attempt says so.
func (s *Server) handleAdminRemoveMatch(w http.ResponseWriter, r *http.Request) {
	self, _ := auth.PlayerID(r.Context())

	id, err := uuid.Parse(strings.TrimSpace(r.PathValue("id")))
	if err != nil {
		s.rejectAdmin(w, r, "Dieses Ergebnis gibt es nicht.")
		return
	}

	undone, err := scoring.Remove(r.Context(), s.store, id)
	switch {
	case err == nil:
	case errors.Is(err, domain.ErrNotFound), errors.Is(err, scoring.ErrNotUndoable):
		s.rejectAdmin(w, r, "Dieses Ergebnis lässt sich nicht entfernen — es ist schon weg, "+
			"oder es wurde nie gewertet.")
		return
	case errors.Is(err, scoring.ErrNotLast):
		s.rejectAdmin(w, r, "Seit diesem Ergebnis wurde für einen der beiden schon ein "+
			"weiteres gewertet. Erst das neuere entfernen, dann dieses — sonst würde die "+
			"Wertung des neueren stillschweigend mit zurückgenommen.")
		return
	default:
		s.log.ErrorContext(r.Context(), "removing a counted match failed",
			"match_id", id, "error", err)
		s.rejectAdmin(w, r, "Das hat gerade nicht geklappt.")
		return
	}

	// Named, like every other admin action: without the line the flag is the
	// kiosk's mistake under a new name (docs/adr/0008, issue #105).
	s.log.InfoContext(r.Context(), "counted match removed",
		"match_id", id, "home_id", undone.Home.ID, "away_id", undone.Away.ID, "by", self)

	s.renderAdmin(w, r, "Entfernt: "+undone.Home.DisplayName+" gegen "+
		undone.Away.DisplayName+" "+strconv.Itoa(undone.HomeSets)+":"+
		strconv.Itoa(undone.AwaySets)+". Beide Wertungen stehen wieder wie vorher. "+
		"Das richtige Ergebnis wird jetzt normal eingetragen.")
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
