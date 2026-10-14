package server

import (
	"context"
	"errors"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/auth"
	"github.com/stuttgart-things/schmetterpause/internal/domain"
	"github.com/stuttgart-things/schmetterpause/internal/templates"
)

// fobLabelLen is how much of the hash the profile shows to tell two fobs
// apart. Six hex characters are plenty for the handful one person holds.
const fobLabelLen = 6

// handleOpenFobWindow opens the window in which the next unknown fob at the
// reader becomes the signed-in player's (docs/adr/0025).
func (s *Server) handleOpenFobWindow(w http.ResponseWriter, r *http.Request) {
	self, _ := auth.PlayerID(r.Context())

	known, err := s.fobs.ForPlayer(r.Context(), self)
	if err != nil {
		s.fobCardError(w, r, self, "listing fobs failed", err)
		return
	}

	window, err := s.fobs.OpenWindow(r.Context(), self)
	if errors.Is(err, domain.ErrConflict) {
		view, err := s.fobCardView(r.Context(), self)
		if err != nil {
			s.fobCardError(w, r, self, "loading the fob card failed", err)
			return
		}
		view.BusySeconds = max(secondsUntil(window.ExpiresAt), 1)
		s.render(w, r, templates.FobCard(view))
		return
	}
	if err != nil {
		s.fobCardError(w, r, self, "opening the fob window failed", err)
		return
	}

	s.log.InfoContext(r.Context(), "fob window opened", "player_id", self)
	view, err := s.fobCardView(r.Context(), self)
	if err != nil {
		s.fobCardError(w, r, self, "loading the fob card failed", err)
		return
	}
	view.Known = len(known)
	s.render(w, r, templates.FobCard(view))
}

// handleCloseFobWindow gives the player's window up before it runs out.
func (s *Server) handleCloseFobWindow(w http.ResponseWriter, r *http.Request) {
	self, _ := auth.PlayerID(r.Context())

	if err := s.fobs.CloseWindow(r.Context(), self); err != nil {
		s.fobCardError(w, r, self, "closing the fob window failed", err)
		return
	}
	s.renderFobCard(w, r, self)
}

// handleRemoveFob unlinks one of the player's own fobs.
func (s *Server) handleRemoveFob(w http.ResponseWriter, r *http.Request) {
	self, _ := auth.PlayerID(r.Context())

	err := s.fobs.Remove(r.Context(), self, r.FormValue("subject"))
	switch {
	case errors.Is(err, domain.ErrNotFound):
		// Already gone, from another tab or a double click. The card shows
		// what is true now, which is the answer.
	case err != nil:
		s.fobCardError(w, r, self, "removing a fob failed", err)
		return
	default:
		s.log.InfoContext(r.Context(), "fob removed", "player_id", self)
	}
	s.renderFobCard(w, r, self)
}

// handleFobFragment is what the card polls while its window is open.
//
// known is how many fobs the player had when the window opened. Once the
// window is gone, one more than that means the fob arrived; the same number
// means the window ran out.
func (s *Server) handleFobFragment(w http.ResponseWriter, r *http.Request) {
	self, _ := auth.PlayerID(r.Context())

	view, err := s.fobCardView(r.Context(), self)
	if err != nil {
		s.fobCardError(w, r, self, "loading the fob card failed", err)
		return
	}
	if known, err := strconv.Atoi(r.URL.Query().Get("known")); err == nil && !view.Waiting {
		if len(view.Fobs) > known {
			view.JustLinked = true
		} else {
			view.Expired = true
		}
	}
	s.render(w, r, templates.FobCard(view))
}

func (s *Server) renderFobCard(w http.ResponseWriter, r *http.Request, self uuid.UUID) {
	view, err := s.fobCardView(r.Context(), self)
	if err != nil {
		s.fobCardError(w, r, self, "loading the fob card failed", err)
		return
	}
	s.render(w, r, templates.FobCard(view))
}

// fobCardView reads the player's fobs and whether their window is open.
func (s *Server) fobCardView(ctx context.Context, self uuid.UUID) (templates.FobCardView, error) {
	fobs, err := s.fobs.ForPlayer(ctx, self)
	if err != nil {
		return templates.FobCardView{}, err
	}

	view := templates.FobCardView{Fobs: make([]templates.FobRow, 0, len(fobs)), Known: len(fobs)}
	for _, fob := range fobs {
		row := templates.FobRow{
			Subject:  fob.Subject,
			Label:    fob.Subject[:min(fobLabelLen, len(fob.Subject))],
			LinkedAt: fob.CreatedAt.Local().Format("02.01.2006"),
		}
		if fob.LastUsedAt != nil {
			row.LastUsed = fob.LastUsedAt.Local().Format("02.01.2006 15:04")
		}
		view.Fobs = append(view.Fobs, row)
	}

	window, err := s.fobs.Window(ctx)
	switch {
	case errors.Is(err, domain.ErrNotFound):
	case err != nil:
		return templates.FobCardView{}, err
	case window.PlayerID == self:
		view.Waiting = true
		view.SecondsLeft = max(secondsUntil(window.ExpiresAt), 1)
	}
	return view, nil
}

func (s *Server) fobCardError(w http.ResponseWriter, r *http.Request, self uuid.UUID, msg string, err error) {
	s.log.ErrorContext(r.Context(), msg, "player_id", self, "error", err)
	s.render(w, r, templates.FobCard(templates.FobCardView{
		Error: "Das hat gerade nicht geklappt. Versuch es noch einmal.",
	}))
}

// secondsUntil rounds up, so a window with half a second left still says one
// rather than zero.
func secondsUntil(t time.Time) int {
	return int(math.Ceil(time.Until(t).Seconds()))
}
