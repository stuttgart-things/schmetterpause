package server

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

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
// A player created here holds no identity and cannot sign in from their own
// phone afterwards. That is the honest cost of entering somebody into a
// tournament while they are standing at the table; issue #37 is where a way
// back from it would live.
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

	settlement, err := scoring.Record(r.Context(), s.store, homeID, awayID, form.result, time.Now())

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
	filled.Note, filled.Error = view.Note, view.Error
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
