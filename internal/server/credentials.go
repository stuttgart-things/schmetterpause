package server

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/auth"
	"github.com/stuttgart-things/schmetterpause/internal/credential"
	"github.com/stuttgart-things/schmetterpause/internal/domain"
	"github.com/stuttgart-things/schmetterpause/internal/templates"
)

// handleSetPIN sets or replaces the signed-in player's PIN.
//
// Only ever your own. A PIN somebody else knows is not a PIN, which is why
// the kiosk can issue a recovery code for another player but cannot do this
// (docs/adr/0007, open point 3).
//
// It does not touch the recovery code. The PIN sits on top of it: PIN
// forgotten means recovery code, code lost means the kiosk.
func (s *Server) handleSetPIN(w http.ResponseWriter, r *http.Request) {
	self, ok := auth.PlayerID(r.Context())
	if !ok {
		http.Error(w, "Erst anmelden", http.StatusUnauthorized)
		return
	}

	pin := strings.TrimSpace(r.FormValue("pin"))
	had := s.hasPIN(r.Context(), self)

	if msg, valid := validatePIN(pin); !valid {
		s.rejectPIN(w, r, had, msg)
		return
	}

	if err := s.store.Credentials().Put(r.Context(), self, domain.CredentialPIN, credential.Hash(pin)); err != nil {
		s.log.ErrorContext(r.Context(), "storing the pin failed", "player_id", self, "error", err)
		s.rejectPIN(w, r, had, "Das hat gerade nicht geklappt. Versuch es noch einmal.")
		return
	}

	// A new PIN is a way in, so a stale brake on the old one would hold up
	// the player who just proved they are themselves.
	s.signInByPlayer.Succeeded(self.String())
	s.log.InfoContext(r.Context(), "pin set", "player_id", self, "replaced", had)

	s.render(w, r, templates.PINForm(templates.PINFormView{Set: true, Done: true}))
}

// handleNewRecoveryCode issues the signed-in player a fresh code and shows it
// once, invalidating whatever they had.
//
// Not an administrative act, though it looks like one. Whoever asks already
// holds a valid session, so it grants them nothing new — it takes validity
// away from the old code. Making it somebody else's decision would mean that
// anyone who suspects their code has leaked has to wait for a tournament
// evening, and in a normal week no kiosk runs at all (docs/adr/0006).
func (s *Server) handleNewRecoveryCode(w http.ResponseWriter, r *http.Request) {
	self, ok := auth.PlayerID(r.Context())
	if !ok {
		http.Error(w, "Erst anmelden", http.StatusUnauthorized)
		return
	}

	code, hash := credential.NewRecoveryCode()
	if err := s.store.Credentials().Put(r.Context(), self, domain.CredentialRecovery, hash); err != nil {
		s.log.ErrorContext(r.Context(), "issuing a recovery code failed", "player_id", self, "error", err)
		s.render(w, r, templates.RecoveryCard(templates.RecoveryCardView{
			Error: "Das hat gerade nicht geklappt. Versuch es noch einmal.",
		}))
		return
	}

	s.signInByPlayer.Succeeded(self.String())
	s.log.InfoContext(r.Context(), "recovery code reissued", "player_id", self)

	s.render(w, r, templates.RecoveryCard(templates.RecoveryCardView{Code: code}))
}

// rejectPIN re-renders the form with the reason. What was typed is not handed
// back: it is a secret, and echoing it into the page would put it in the
// response, in the browser's back-forward cache and in anything watching.
func (s *Server) rejectPIN(w http.ResponseWriter, r *http.Request, had bool, msg string) {
	w.WriteHeader(http.StatusUnprocessableEntity)
	s.render(w, r, templates.PINForm(templates.PINFormView{Set: had, Error: msg}))
}

// hasPIN reports whether this player already has one. Only the wording
// depends on it, so a failure to find out is logged and treated as "no".
func (s *Server) hasPIN(ctx context.Context, id uuid.UUID) bool {
	_, err := s.store.Credentials().ForPlayer(ctx, id, domain.CredentialPIN)
	switch {
	case err == nil:
		return true
	case errors.Is(err, domain.ErrNotFound):
		return false
	default:
		s.log.ErrorContext(ctx, "looking up the pin failed", "player_id", id, "error", err)
		return false
	}
}

// validatePIN reports whether a PIN is usable, and why not if it is not. The
// message is shown to the player, so it says what to do about it.
//
// Digits only, and that is not pedantry. docs/adr/0006 refuses a self-chosen
// secret because a field somebody may type anything into becomes a field
// somebody types their company password into; docs/adr/0007 answers that with
// the shape of the field rather than with hope. A company password does not
// fit in a digits-only field.
func validatePIN(pin string) (string, bool) {
	switch {
	case pin == "":
		return "Ohne Ziffern geht es nicht.", false
	case !isDigits(pin):
		return "Nur Ziffern. Kein Passwort, keine Buchstaben.", false
	case len(pin) < credential.MinPINLength:
		return "Mindestens " + strconv.Itoa(credential.MinPINLength) + " Ziffern.", false
	case len(pin) > credential.MaxPINLength:
		return "Höchstens " + strconv.Itoa(credential.MaxPINLength) + " Ziffern.", false
	}
	return "", true
}
