package server

import (
	"net/http"

	"github.com/stuttgart-things/schmetterpause/internal/templates"
)

// handleIndex rendert die Startseite.
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, templates.Index())
}

// handleStatusFragment liefert das per HTMX nachgeladene Statusfragment.
//
// Im Geruest ist das der Nachweis, dass Handler, Repository, Datenbank und
// Template zusammenspielen — genau der Pfad, den der Verify-Schritt der
// Pipeline gegen das gebaute Image prueft.
func (s *Server) handleStatusFragment(w http.ResponseWriter, r *http.Request) {
	view := templates.StatusView{Version: s.version}

	if err := s.store.Ping(r.Context()); err != nil {
		s.log.WarnContext(r.Context(), "status: datenbank nicht erreichbar", "fehler", err)
		s.render(w, r, templates.Status(view))
		return
	}
	view.DatabaseReachable = true

	count, err := s.store.Players().Count(r.Context())
	if err != nil {
		s.log.ErrorContext(r.Context(), "status: spieler zaehlen fehlgeschlagen", "fehler", err)
		http.Error(w, "Status nicht ermittelbar", http.StatusInternalServerError)
		return
	}
	view.Players = count

	s.render(w, r, templates.Status(view))
}
