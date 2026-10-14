package server

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/stuttgart-things/schmetterpause/internal/auth"
	"github.com/stuttgart-things/schmetterpause/internal/domain"
	"github.com/stuttgart-things/schmetterpause/internal/qr"
	"github.com/stuttgart-things/schmetterpause/internal/ratelimit"
	"github.com/stuttgart-things/schmetterpause/internal/templates"
	"github.com/stuttgart-things/schmetterpause/internal/zaehlwerk"
)

// tablePolicy brakes a phone that keeps pressing "Hier anmelden". Every press
// counts, not only failed ones: each one is a check-in row and a call to the
// Zählwerk, and the sixth in a row is not a player waiting for an opponent.
// Free is four because the limiter waits once the count is past it, and the
// count is taken before the press is added: five pass, the sixth waits.
// Short, because the person it slows down is standing at the table.
var tablePolicy = ratelimit.Policy{
	Free:   4,
	Step:   5 * time.Second,
	Max:    time.Minute,
	Forget: 10 * time.Minute,
}

// halfLabel names a half in the page's words.
func halfLabel(h domain.Half) string {
	if h == domain.HalfRight {
		return "rechte Seite"
	}
	return "linke Seite"
}

// tableHalf reads the half from the path. Anything else is not a page.
func tableHalf(r *http.Request) (domain.Half, bool) {
	h := domain.Half(r.PathValue("half"))
	return h, h.Valid()
}

// handleTablePage is where the QR code at one half leads (docs/adr/0026). It
// shows the button and checks nobody in.
func (s *Server) handleTablePage(w http.ResponseWriter, r *http.Request) {
	half, ok := tableHalf(r)
	if !ok {
		http.NotFound(w, r)
		return
	}

	view := templates.TableView{
		Header:    s.headerView(r.Context()),
		Half:      string(half),
		HalfLabel: halfLabel(half),
		Panel:     templates.TablePanelView{Half: string(half), HalfLabel: halfLabel(half)},
	}
	if self, ok := auth.PlayerID(r.Context()); ok {
		player, err := s.store.Players().ByID(r.Context(), self)
		if err != nil {
			s.log.ErrorContext(r.Context(), "loading the player for the table page failed", "error", err)
			http.Error(w, "Seite nicht verfügbar", http.StatusInternalServerError)
			return
		}
		view.SignedIn, view.Observer = true, player.IsObserver
	}
	s.render(w, r, templates.TablePage(view))
}

// handleTableCheckin checks the signed-in player in at a half and claims it on
// the Zählwerk (docs/adr/0026).
//
// The check-in row is written before the call and stays when the call fails.
// Alone it proves nothing, and it expires by itself (checkin.Validity);
// removing it would erase that somebody tried.
func (s *Server) handleTableCheckin(w http.ResponseWriter, r *http.Request) {
	half, ok := tableHalf(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	self, _ := auth.PlayerID(r.Context())
	panel := templates.TablePanelView{Half: string(half), HalfLabel: halfLabel(half)}

	key := self.String()
	if wait := s.tableByPlayer.Retry(key); wait > 0 {
		panel.Result = "Einen Moment, du hast dich gerade erst angemeldet."
		s.render(w, r, templates.TablePanel(panel))
		return
	}
	s.tableByPlayer.Failed(key)

	player, err := s.store.Players().ByID(r.Context(), self)
	if err != nil {
		s.tableError(w, r, panel, "loading the player for a table check-in failed", err)
		return
	}
	// An observer does not play (docs/adr/0022). The page has no button for
	// them; this is for a form sent anyway.
	if player.IsObserver {
		panel.Result = "Als Beobachter spielst du nicht mit."
		s.render(w, r, templates.TablePanel(panel))
		return
	}

	created, err := s.store.Checkins().Create(r.Context(), domain.Checkin{
		PlayerID:  self,
		Half:      half,
		Method:    domain.CheckinPhone,
		CreatedAt: time.Now(),
	})
	if err != nil {
		s.tableError(w, r, panel, "recording a phone check-in failed", err)
		return
	}

	answer, err := s.tables.Claim(r.Context(), half, self, created.ID)
	if err != nil {
		if !errors.Is(err, zaehlwerk.ErrUnavailable) {
			s.log.ErrorContext(r.Context(), "claiming a half failed", "error", err)
		} else {
			s.log.WarnContext(r.Context(), "claiming a half failed", "error", err)
		}
		panel.Result = "Das Zählwerk ist gerade nicht erreichbar. Versuch es gleich noch einmal."
		s.render(w, r, templates.TablePanel(panel))
		return
	}

	s.log.InfoContext(r.Context(), "phone check-in",
		"player_id", self, "checkin_id", created.ID, "half", half, "outcome", answer.Outcome)

	panel.Left, panel.Right = answer.Left, answer.Right
	switch answer.Outcome {
	case zaehlwerk.Refused:
		panel.Result = "Am Tisch läuft gerade ein Match. Melde dich an, wenn es vorbei ist."
	case zaehlwerk.Started:
		panel.Result, panel.OK = "Ihr seid beide angemeldet. Das Match startet.", true
	default:
		panel.Result, panel.OK = "Du bist angemeldet. Jetzt fehlt noch die andere Seite.", true
	}
	s.render(w, r, templates.TablePanel(panel))
}

func (s *Server) tableError(w http.ResponseWriter, r *http.Request, panel templates.TablePanelView, msg string, err error) {
	s.log.ErrorContext(r.Context(), msg, "error", err)
	panel.Result = "Das hat nicht geklappt. Versuch es gleich noch einmal."
	s.render(w, r, templates.TablePanel(panel))
}

// handleTableSheet prints one QR code per half.
func (s *Server) handleTableSheet(w http.ResponseWriter, r *http.Request) {
	view := templates.TableSheetView{Header: s.headerView(r.Context())}
	for _, half := range []domain.Half{domain.HalfLeft, domain.HalfRight} {
		target := s.publicBaseURL(r) + "/table/" + string(half)
		code, err := qr.Render(target)
		if err != nil {
			s.log.ErrorContext(r.Context(), "rendering a table QR code failed", "target", target, "error", err)
			http.Error(w, "QR-Code nicht verfügbar", http.StatusInternalServerError)
			return
		}
		view.Codes = append(view.Codes, templates.TableCode{
			// Capitalised, because on the sheet it stands alone as a heading.
			Label:  strings.ToUpper(halfLabel(half)[:1]) + halfLabel(half)[1:],
			Target: target, Path: code.Path, Size: code.Size,
		})
	}
	s.render(w, r, templates.TableSheet(view))
}
