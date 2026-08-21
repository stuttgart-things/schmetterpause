package server

import (
	"net/http"

	"github.com/a-h/templ"
)

// render writes a templ component as HTML.
//
// A rendering error happens in practice only when the connection drops. By
// then the headers are out and an http.Error would have no effect, so this
// only logs.
func (s *Server) render(w http.ResponseWriter, r *http.Request, c templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if err := c.Render(r.Context(), w); err != nil {
		s.log.ErrorContext(r.Context(), "rendering template failed",
			"path", r.URL.Path, "error", err)
	}
}
