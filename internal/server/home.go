package server

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/auth"
	"github.com/stuttgart-things/schmetterpause/internal/credential"
	"github.com/stuttgart-things/schmetterpause/internal/domain"
	"github.com/stuttgart-things/schmetterpause/internal/repository"
	"github.com/stuttgart-things/schmetterpause/internal/templates"
)

// maxDisplayNameLen bounds a display name in runes, matching the maxlength on
// the form input. Checked here as well, because the attribute only guides a
// browser and is not a rule.
const maxDisplayNameLen = 40

// handleWhoami serves the top-right corner on its own, for the slow refresh
// that makes the badge appear without a reload.
func (s *Server) handleWhoami(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, templates.Whoami(s.headerView(r.Context())))
}

// headerView describes who this is and how much waits on them.
//
// It never fails the request: a top bar that cannot say how many results are
// waiting is worth less than a page, so a broken count is logged and the
// badge simply stays away.
func (s *Server) headerView(ctx context.Context) templates.HeaderView {
	id, ok := auth.PlayerID(ctx)
	if !ok {
		return templates.HeaderView{}
	}

	player, err := s.store.Players().ByID(ctx, id)
	if err != nil {
		s.log.WarnContext(ctx, "the session points at an unknown player",
			"player_id", id, "error", err)
		return templates.HeaderView{}
	}

	view := templates.HeaderView{
		DisplayName: player.DisplayName,
		ProfileURL:  "/players/" + player.ID.String(),
	}

	waiting, err := s.store.Matches().PendingCountFor(ctx, id)
	if err != nil {
		s.log.ErrorContext(ctx, "counting the pending matches failed", "error", err)
		return view
	}
	view.Pending = waiting
	return view
}

// handleIndex renders the start page: session, result entry and the roster.
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	view := templates.IndexView{
		Header:  s.headerView(r.Context()),
		Session: s.sessionView(r.Context()),
	}

	table, err := s.standingsView(r.Context())
	if err != nil {
		s.log.ErrorContext(r.Context(), "loading the standings failed", "error", err)
		http.Error(w, "Rangliste nicht verfügbar", http.StatusInternalServerError)
		return
	}
	view.Standings = table

	// Result entry needs somebody to attribute the report to, so it appears
	// only once the browser is recognised.
	if self, ok := auth.PlayerID(r.Context()); ok {
		opponents, err := s.opponentOptions(r.Context(), self, uuid.Nil)
		if err != nil {
			s.log.ErrorContext(r.Context(), "loading the opponents failed", "error", err)
			http.Error(w, "Gegnerliste nicht verfügbar", http.StatusInternalServerError)
			return
		}
		view.Match = templates.NewMatchFormView(opponents)
		view.ShowMatch = true

		pending, err := s.pendingListView(r.Context(), self)
		if err != nil {
			s.log.ErrorContext(r.Context(), "loading the pending matches failed", "error", err)
			http.Error(w, "Offene Ergebnisse nicht verfügbar", http.StatusInternalServerError)
			return
		}
		view.Pending = pending
	}

	s.render(w, r, templates.Index(view))
}

// handleJoin creates a player, starts a session for them and issues their
// recovery code.
//
// AP2 has no password and no verification: a display name is all it takes.
// The realistic abuse case for this app is a colleague entering a joke result,
// and the answer to that is the opponent confirming the match (AP5), not a
// login wall — see the threat-model note in docs/adr/0004.
//
// The code costs no interaction — it is generated, not chosen, and only
// displayed — so the interaction budget AP7 measures is unchanged
// (docs/adr/0006).
func (s *Server) handleJoin(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.FormValue("display_name"))

	if msg, ok := validateDisplayName(name); !ok {
		s.rejectJoin(w, r, name, msg)
		return
	}

	// The player, the identity that points at them and the way back have to
	// appear together. A player nobody can be recognised as is unreachable,
	// an identity pointing at nothing breaks the next lookup, and a player
	// with no recovery code is one browser away from issue #70.
	subject := auth.NewSubject()

	// Hashed out here rather than inside the transaction: Argon2id is
	// deliberately slow, and holding a write transaction open for a tenth of
	// a second of it buys nothing.
	code, codeHash := credential.NewRecoveryCode()

	var created domain.Player

	err := s.store.InTx(r.Context(), func(tx repository.Store) error {
		player, err := tx.Players().Create(r.Context(), name, domain.DefaultTTR)
		if err != nil {
			return err
		}
		if err := tx.Identities().Link(r.Context(), domain.ProviderLocal, subject, player.ID); err != nil {
			return err
		}
		if err := tx.Credentials().Put(r.Context(), player.ID, domain.CredentialRecovery, codeHash); err != nil {
			return err
		}
		created = player
		return nil
	})

	switch {
	case errors.Is(err, domain.ErrConflict):
		s.rejectJoin(w, r, name, "Diesen Namen gibt es schon. Nimm einen anderen.")
		return
	case err != nil:
		s.log.ErrorContext(r.Context(), "creating the player failed", "name", name, "error", err)
		s.rejectJoin(w, r, name, "Das hat gerade nicht geklappt. Versuch es noch einmal.")
		return
	}

	s.auth.SetCookie(w, subject)

	signedIn := auth.WithPlayerID(r.Context(), created.ID)

	table, err := s.standingsView(signedIn)
	if err != nil {
		// The player exists and is signed in; only the ranking is missing.
		// Saying so beats discarding a successful join.
		s.log.ErrorContext(r.Context(), "loading the standings failed", "error", err)
	}

	// The corner and the greeting are refreshed out of band in the same
	// response, so the name appears where it now lives rather than only
	// after a reload.
	joined := templates.SessionView{
		DisplayName: created.DisplayName,
		PlayerID:    created.ID.String(),
		// The one response that ever carries the code in the clear.
		RecoveryCode: code,
	}
	s.render(w, r, templates.Joined(joined))
	s.render(w, r, templates.WhoamiOOB(s.headerView(signedIn)))
	s.render(w, r, templates.PageHeadOOB(joined))
	s.render(w, r, templates.StandingsOOB(table))

	// And so does result entry, which is the whole reason somebody just
	// typed their name. Without this the form arrives only on the next
	// reload, with nothing on the page saying so — see issue #75.
	opponents, err := s.opponentOptions(signedIn, created.ID, uuid.Nil)
	if err != nil {
		// Same trade as the standings above: the player exists and is
		// signed in. A reload brings the form; discarding the join would
		// bring nothing.
		s.log.ErrorContext(r.Context(), "loading the opponents failed", "error", err)
		return
	}
	s.render(w, r, templates.MatchFormOOB(templates.NewMatchFormView(opponents)))
}

// rejectJoin re-renders the form with the reason, keeping what was typed.
func (s *Server) rejectJoin(w http.ResponseWriter, r *http.Request, name, msg string) {
	// 422 rather than 400: the request was well formed, its content was not.
	// HTMX swaps 4xx responses only when told to, so the form asks for it.
	w.WriteHeader(http.StatusUnprocessableEntity)
	s.render(w, r, templates.Session(templates.SessionView{Name: name, Error: msg}))
}

// sessionView describes who, if anyone, the request belongs to.
func (s *Server) sessionView(ctx context.Context) templates.SessionView {
	id, ok := auth.PlayerID(ctx)
	if !ok {
		return templates.SessionView{}
	}

	player, err := s.store.Players().ByID(ctx, id)
	if err != nil {
		// The cookie resolved to a player that has since vanished. Show the
		// join form rather than an error; the next join issues a new cookie.
		s.log.WarnContext(ctx, "the session points at an unknown player",
			"player_id", id, "error", err)
		return templates.SessionView{}
	}
	return templates.SessionView{DisplayName: player.DisplayName, PlayerID: player.ID.String()}
}

// validateDisplayName reports whether a name is usable, and why not if it is
// not. The message is shown to the player, so it says what to do about it.
func validateDisplayName(name string) (string, bool) {
	switch {
	case name == "":
		return "Ohne Namen geht es nicht.", false
	case utf8.RuneCountInString(name) > maxDisplayNameLen:
		return "Das ist zu lang. Höchstens 40 Zeichen.", false
	case strings.ContainsAny(name, "\n\r\t"):
		return "Zeilenumbrüche und Tabs gehen nicht.", false
	}
	return "", true
}
