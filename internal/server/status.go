package server

import (
	"net/http"

	"github.com/stuttgart-things/schmetterpause/internal/templates"
)

// handleStatusFragment serves the status fragment loaded by HTMX.
//
// It is the proof that handler, repository, database and template work
// together — exactly the path the pipeline's verify step checks against the
// built image, which is why the markup it asserts on should stay stable.
func (s *Server) handleStatusFragment(w http.ResponseWriter, r *http.Request) {
	view := templates.StatusView{Version: s.version}

	if err := s.store.Ping(r.Context()); err != nil {
		s.log.WarnContext(r.Context(), "status: database unreachable", "error", err)
		s.render(w, r, templates.Status(view))
		return
	}
	view.DatabaseReachable = true

	count, err := s.store.Players().Count(r.Context())
	if err != nil {
		s.log.ErrorContext(r.Context(), "status: counting players failed", "error", err)
		// User-facing text stays German; see CLAUDE.md.
		http.Error(w, "Status nicht ermittelbar", http.StatusInternalServerError)
		return
	}
	view.Players = count

	s.render(w, r, templates.Status(view))
}
