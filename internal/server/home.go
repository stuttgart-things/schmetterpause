package server

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/auth"
	"github.com/stuttgart-things/schmetterpause/internal/domain"
	"github.com/stuttgart-things/schmetterpause/internal/repository"
	"github.com/stuttgart-things/schmetterpause/internal/templates"
)

// maxDisplayNameLen bounds a display name in runes, matching the maxlength on
// the form input. Checked here as well, because the attribute only guides a
// browser and is not a rule.
const maxDisplayNameLen = 40

// handleIndex renders the start page with the current session and the roster.
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	session := s.sessionView(r.Context())

	players, err := s.playerListView(r.Context())
	if err != nil {
		s.log.ErrorContext(r.Context(), "loading the player list failed", "error", err)
		http.Error(w, "Spielerliste nicht verfügbar", http.StatusInternalServerError)
		return
	}

	s.render(w, r, templates.Index(session, players))
}

// handleJoin creates a player and starts a session for them.
//
// AP2 has no password and no verification: a display name is all it takes.
// The realistic abuse case for this app is a colleague entering a joke result,
// and the answer to that is the opponent confirming the match (AP5), not a
// login wall — see the threat-model note in docs/adr/0004.
func (s *Server) handleJoin(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.FormValue("display_name"))

	if msg, ok := validateDisplayName(name); !ok {
		s.rejectJoin(w, r, name, msg)
		return
	}

	// The player and the identity that points at them have to appear
	// together. A player nobody can be recognised as is unreachable, and an
	// identity pointing at nothing breaks the next lookup.
	subject := auth.NewSubject()
	var created domain.Player

	err := s.store.InTx(r.Context(), func(tx repository.Store) error {
		player, err := tx.Players().Create(r.Context(), name, domain.DefaultTTR)
		if err != nil {
			return err
		}
		if err := tx.Identities().Link(r.Context(), domain.ProviderLocal, subject, player.ID); err != nil {
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

	players, err := s.playerListView(auth.WithPlayerID(r.Context(), created.ID))
	if err != nil {
		// The player exists and is signed in; only the roster is missing.
		// Saying so beats discarding a successful join.
		s.log.ErrorContext(r.Context(), "loading the player list failed", "error", err)
	}

	s.render(w, r, templates.Session(templates.SessionView{DisplayName: created.DisplayName}))
	s.render(w, r, templates.PlayerListOOB(players))
}

// handlePlayersFragment serves the roster on its own.
func (s *Server) handlePlayersFragment(w http.ResponseWriter, r *http.Request) {
	players, err := s.playerListView(r.Context())
	if err != nil {
		s.log.ErrorContext(r.Context(), "loading the player list failed", "error", err)
		http.Error(w, "Spielerliste nicht verfügbar", http.StatusInternalServerError)
		return
	}
	s.render(w, r, templates.PlayerList(players))
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
	return templates.SessionView{DisplayName: player.DisplayName}
}

func (s *Server) playerListView(ctx context.Context) (templates.PlayerListView, error) {
	players, err := s.store.Players().List(ctx)
	if err != nil {
		return templates.PlayerListView{}, err
	}

	self, _ := auth.PlayerID(ctx)

	view := templates.PlayerListView{Players: make([]templates.PlayerListEntry, 0, len(players))}
	for _, p := range players {
		view.Players = append(view.Players, templates.PlayerListEntry{
			DisplayName: p.DisplayName,
			IsSelf:      p.ID == self && self != uuid.Nil,
		})
	}
	return view, nil
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
