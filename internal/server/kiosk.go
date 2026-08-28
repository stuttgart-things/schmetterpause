package server

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/auth"
	"github.com/stuttgart-things/schmetterpause/internal/credential"
	"github.com/stuttgart-things/schmetterpause/internal/domain"
	"github.com/stuttgart-things/schmetterpause/internal/match"
	"github.com/stuttgart-things/schmetterpause/internal/scoring"
	"github.com/stuttgart-things/schmetterpause/internal/templates"
)

// kioskCookieName marks a browser that has shown the token once.
const kioskCookieName = "schmetterpause_kiosk"

// kioskCookieMaxAge is a working day and a bit. Long enough to survive a
// tournament, short enough that a laptop nobody logged out of stops being a
// kiosk by the next morning.
const kioskCookieMaxAge = 12 * time.Hour

// handleKiosk serves the page one machine at the table works from.
//
// A token in the query unlocks it and is then swapped for a cookie, so it is
// typed once rather than sitting in the address bar all afternoon where the
// next person to borrow the laptop reads it.
func (s *Server) handleKiosk(w http.ResponseWriter, r *http.Request) {
	if token := r.URL.Query().Get("token"); token != "" {
		if !s.kioskTokenMatches(token) {
			http.Error(w, "Falscher Zugang", http.StatusForbidden)
			return
		}
		s.setKioskCookie(w)
		// Redirected rather than rendered, so a reload does not carry the
		// token along and the browser history does not keep it.
		http.Redirect(w, r, "/kiosk", http.StatusSeeOther)
		return
	}

	if !s.kioskUnlocked(r) {
		http.Error(w, "Zugang nötig", http.StatusForbidden)
		return
	}

	view, err := s.kioskView(r.Context())
	if err != nil {
		s.log.ErrorContext(r.Context(), "loading the kiosk failed", "error", err)
		http.Error(w, "Kiosk nicht verfügbar", http.StatusInternalServerError)
		return
	}
	s.render(w, r, templates.Kiosk(view))
}

// handleKioskAddPlayer creates a player without touching this browser's own
// session. Eight players entered from one laptop must not leave the laptop
// signed in as the eighth.
//
// A player created here holds no identity, so their own phone does not know
// them yet. The way from here to there is a recovery code, which the kiosk
// can issue on demand — see handleKioskIssueCode. It is not issued
// automatically, because the code would then be on the laptop's screen at a
// moment when the person it belongs to may be three tables away.
func (s *Server) handleKioskAddPlayer(w http.ResponseWriter, r *http.Request) {
	if !s.kioskUnlocked(r) {
		http.Error(w, "Zugang nötig", http.StatusForbidden)
		return
	}

	name := strings.TrimSpace(r.FormValue("display_name"))
	if msg, ok := validateDisplayName(name); !ok {
		s.rejectKiosk(w, r, msg, "")
		return
	}

	_, err := s.store.Players().Create(r.Context(), name, domain.DefaultTTR)
	switch {
	case errors.Is(err, domain.ErrConflict):
		s.rejectKiosk(w, r, "Diesen Namen gibt es schon.", "")
		return
	case err != nil:
		s.log.ErrorContext(r.Context(), "creating the player failed", "name", name, "error", err)
		s.rejectKiosk(w, r, "Das hat gerade nicht geklappt.", "")
		return
	}

	s.renderKiosk(w, r, templates.KioskView{Note: name + " ist dabei."})
}

// handleKioskIssueCode issues a recovery code for somebody standing at the
// table with nothing left: no cookie, no code, no PIN.
//
// The only place in the application where a credential is made for another
// person, and the condition is the room — somebody is standing there and the
// people know each other. That is the whole justification, and it is why this
// stays bound to the kiosk instead of moving into the ordinary interface
// (docs/adr/0006).
//
// It cannot set a PIN. A PIN somebody else knows is not a PIN, so the player
// sets that themselves once they are back in (docs/adr/0007, open point 3).
//
// The new code invalidates whatever they had, which is the same trade as
// anywhere else: somebody who turns up saying they lost it is far more likely
// to have lost it than to be somebody else.
func (s *Server) handleKioskIssueCode(w http.ResponseWriter, r *http.Request) {
	if !s.kioskUnlocked(r) {
		http.Error(w, "Zugang nötig", http.StatusForbidden)
		return
	}

	playerID, err := uuid.Parse(strings.TrimSpace(r.FormValue("player_id")))
	if err != nil {
		s.rejectKiosk(w, r, "Erst den Spieler wählen.", "")
		return
	}

	player, err := s.store.Players().ByID(r.Context(), playerID)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		s.rejectKiosk(w, r, "Diesen Spieler gibt es nicht.", "")
		return
	case err != nil:
		s.log.ErrorContext(r.Context(), "loading the player failed", "player_id", playerID, "error", err)
		s.rejectKiosk(w, r, "Das hat gerade nicht geklappt.", "")
		return
	}

	code, hash := credential.NewRecoveryCode()
	if err := s.store.Credentials().Put(r.Context(), playerID, domain.CredentialRecovery, hash); err != nil {
		s.log.ErrorContext(r.Context(), "issuing a recovery code failed", "player_id", playerID, "error", err)
		s.rejectKiosk(w, r, "Das hat gerade nicht geklappt.", "")
		return
	}

	// Whoever is being helped is standing right there, and a brake left over
	// from their own wrong guesses would keep them out of the code they just
	// watched somebody issue.
	s.signInByPlayer.Succeeded(playerID.String())

	// Issuing a credential for somebody else is the one kiosk action worth a
	// line in the log. Recording and undo already have one; issue #77 asks
	// for this to have it too.
	s.log.InfoContext(r.Context(), "recovery code issued at the kiosk",
		"player_id", playerID, "display_name", player.DisplayName)

	s.renderKiosk(w, r, templates.KioskView{
		IssuedCode: code,
		IssuedFor:  player.DisplayName,
	})
}

// handleKioskRecord stores a result between any two players and settles it at
// once. Nobody is asked to confirm: somebody watched the match and wrote it
// down, which is what a tournament sheet is.
func (s *Server) handleKioskRecord(w http.ResponseWriter, r *http.Request) {
	if !s.kioskUnlocked(r) {
		http.Error(w, "Zugang nötig", http.StatusForbidden)
		return
	}

	homeID, awayID, msg := parseKioskPlayers(r)
	form, setsMsg := parseResultForm(r)
	if msg == "" {
		msg = setsMsg
	}

	if msg != "" {
		s.rejectKiosk(w, r, msg, "")
		return
	}

	if s.kioskSelfEntry(r, homeID, awayID) {
		s.rejectKiosk(w, r,
			"Dein eigenes Spiel nicht hier — trag es auf der Startseite ein, "+
				"dann bestätigt es dein Gegner.", "")
		return
	}

	settlement, err := scoring.Record(r.Context(), s.store, homeID, awayID, form.result,
		domain.EnteredViaKiosk, time.Now())

	var rejection *match.Rejection
	switch {
	case err == nil:
	case errors.Is(err, scoring.ErrSamePlayer):
		s.rejectKiosk(w, r, "Zwei verschiedene Spieler, bitte.", "")
		return
	case errors.Is(err, domain.ErrNotFound):
		s.rejectKiosk(w, r, "Diesen Spieler gibt es nicht.", "")
		return
	case errors.As(err, &rejection):
		s.rejectKiosk(w, r, describeRejection(err), "")
		return
	default:
		s.log.ErrorContext(r.Context(), "recording the match failed", "error", err)
		s.rejectKiosk(w, r, "Das hat gerade nicht geklappt.", "")
		return
	}

	winner, loser := settlement.Home, settlement.Away
	winnerSets, loserSets := settlement.HomeSets, settlement.AwaySets
	if !settlement.HomeWon {
		winner, loser = settlement.Away, settlement.Home
		winnerSets, loserSets = settlement.AwaySets, settlement.HomeSets
	}

	s.log.InfoContext(r.Context(), "kiosk recorded a match",
		"match_id", settlement.Match.ID, "winner", winner.ID, "loser", loser.ID)

	s.renderKiosk(w, r, templates.KioskView{
		Note: winner.DisplayName + " schlägt " + loser.DisplayName + " " +
			strconv.Itoa(winnerSets) + ":" + strconv.Itoa(loserSets) + ".",
		// Offered only here, in the answer to the entry that just happened.
		// A reload loses the button, which is right: this is for the typo
		// somebody is still looking at.
		UndoID: settlement.Match.ID.String(),
	})
}

// rejectKiosk re-renders the page with the reason. 422 rather than 400: the
// request was well formed, its content was not.
func (s *Server) rejectKiosk(w http.ResponseWriter, r *http.Request, msg, _ string) {
	w.WriteHeader(http.StatusUnprocessableEntity)
	s.renderKiosk(w, r, templates.KioskView{Error: msg})
}

// renderKiosk fills in the parts of the page that are the same either way —
// the players and the ranking — and renders it.
func (s *Server) renderKiosk(w http.ResponseWriter, r *http.Request, view templates.KioskView) {
	filled, err := s.kioskView(r.Context())
	if err != nil {
		s.log.ErrorContext(r.Context(), "loading the kiosk failed", "error", err)
		http.Error(w, "Kiosk nicht verfügbar", http.StatusInternalServerError)
		return
	}
	// Everything the caller owns, in one place: kioskView builds the page and
	// knows nothing about what just happened, so anything it does not set has
	// to be carried across here. A field forgotten in this line is a field
	// that silently never reaches the page.
	filled.Note, filled.Error, filled.UndoID = view.Note, view.Error, view.UndoID
	filled.IssuedCode, filled.IssuedFor = view.IssuedCode, view.IssuedFor
	s.render(w, r, templates.Kiosk(filled))
}

func (s *Server) kioskView(ctx context.Context) (templates.KioskView, error) {
	players, err := s.store.Players().List(ctx)
	if err != nil {
		return templates.KioskView{}, err
	}

	view := templates.KioskView{
		Players: make([]templates.OpponentOption, 0, len(players)),
		Sets:    make([]templates.SetInput, templates.MaxSetRows),
		BestOf:  match.DefaultBestOf, PointsToWin: match.PointsToEleven,
	}
	for _, p := range players {
		view.Players = append(view.Players, templates.OpponentOption{
			ID: p.ID.String(), DisplayName: p.DisplayName,
		})
	}

	view.Standings, err = s.standingsView(ctx)
	if err != nil {
		return templates.KioskView{}, err
	}
	return view, nil
}

// parseKioskPlayers reads the two pickers. Unlike result entry there is no
// "self" here, so both sides have to be chosen.
func parseKioskPlayers(r *http.Request) (uuid.UUID, uuid.UUID, string) {
	home, homeErr := uuid.Parse(strings.TrimSpace(r.FormValue("home_id")))
	away, awayErr := uuid.Parse(strings.TrimSpace(r.FormValue("away_id")))

	if homeErr != nil || awayErr != nil {
		return uuid.Nil, uuid.Nil, "Wähle beide Spieler."
	}
	if home == away {
		return uuid.Nil, uuid.Nil, "Zwei verschiedene Spieler, bitte."
	}
	return home, away, ""
}

// kioskUnlocked reports whether this browser has shown the token.
func (s *Server) kioskUnlocked(r *http.Request) bool {
	cookie, err := r.Cookie(kioskCookieName)
	if err != nil {
		return false
	}
	return hmac.Equal([]byte(cookie.Value), []byte(s.kioskCookieValue()))
}

func (s *Server) setKioskCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     kioskCookieName,
		Value:    s.kioskCookieValue(),
		Path:     "/kiosk",
		HttpOnly: true,
		Secure:   s.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(kioskCookieMaxAge.Seconds()),
	})
}

// kioskCookieValue signs a marker with the session key rather than storing
// the token, so the token itself never travels back out of the server.
func (s *Server) kioskCookieValue() string {
	mac := hmac.New(sha256.New, s.cfg.SessionKey)
	mac.Write([]byte("kiosk:" + s.cfg.KioskToken))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (s *Server) kioskTokenMatches(token string) bool {
	return subtle.ConstantTimeCompare([]byte(token), []byte(s.cfg.KioskToken)) == 1
}

// kioskSelfEntry reports whether the player signed in on this browser is one
// of the players named.
//
// A kiosk result counts at once, so the opponent never gets to agree with it.
// That is right for somebody at the table writing down other people's games,
// and wrong for entering your own — it removes the one check the application
// has. See issue #90.
//
// This is a speed bump and not a boundary, and the difference matters. The
// kiosk deliberately holds no identity of its own, so all this can see is a
// player signed in on this very browser; the same person in a private window
// is invisible to it. What it stops is the convenient path, which is the one
// that actually gets taken. The boundary is issue #73, where the kiosk gets
// an operator instead of a shared token.
func (s *Server) kioskSelfEntry(r *http.Request, players ...uuid.UUID) bool {
	self, ok := auth.PlayerID(r.Context())
	if !ok {
		return false
	}
	return slices.Contains(players, self)
}

// handleKioskUndo takes back the result the kiosk just entered.
//
// A kiosk result counts at once, so there is no pending state to dispute and
// nothing to correct — without this, a mistyped one stands for good and two
// ratings stay wrong. See issue #49.
func (s *Server) handleKioskUndo(w http.ResponseWriter, r *http.Request) {
	if !s.kioskUnlocked(r) {
		http.Error(w, "Zugang nötig", http.StatusForbidden)
		return
	}

	id, err := uuid.Parse(strings.TrimSpace(r.PathValue("id")))
	if err != nil {
		s.rejectKiosk(w, r, "Dieses Ergebnis gibt es nicht.", "")
		return
	}

	// The same rule as recording: taking back a result you played in is the
	// same act from the other side. A match that cannot be read is left to
	// scoring.Undo below, which says why in the words the page already has.
	if m, err := s.store.Matches().ByID(r.Context(), id); err == nil &&
		s.kioskSelfEntry(r, m.HomeID, m.AwayID) {
		s.rejectKiosk(w, r, "Ein Ergebnis, in dem du selbst mitspielst, "+
			"kannst du hier nicht zurücknehmen.", "")
		return
	}

	undone, err := scoring.Undo(r.Context(), s.store, id, time.Now())
	switch {
	case err == nil:
	case errors.Is(err, domain.ErrNotFound), errors.Is(err, scoring.ErrNotUndoable):
		s.rejectKiosk(w, r, "Dieses Ergebnis lässt sich nicht mehr zurücknehmen.", "")
		return
	case errors.Is(err, scoring.ErrTooLate):
		s.rejectKiosk(w, r, "Zu spät — zurücknehmen geht nur kurz nach dem Eintragen.", "")
		return
	case errors.Is(err, scoring.ErrNotLast):
		s.rejectKiosk(w, r,
			"Seit diesem Ergebnis wurde schon ein weiteres gewertet. Zurücknehmen würde das mit rückgängig machen.", "")
		return
	default:
		s.log.ErrorContext(r.Context(), "taking a match back failed", "match_id", id, "error", err)
		s.rejectKiosk(w, r, "Das hat gerade nicht geklappt.", "")
		return
	}

	s.log.InfoContext(r.Context(), "kiosk took a match back", "match_id", id)

	s.renderKiosk(w, r, templates.KioskView{
		Note: "Zurückgenommen: " + undone.Home.DisplayName + " gegen " +
			undone.Away.DisplayName + " " + strconv.Itoa(undone.HomeSets) + ":" +
			strconv.Itoa(undone.AwaySets) + ". Beide Wertungen stehen wieder wie vorher.",
	})
}
