package server

import (
	"net/http"

	"github.com/a-h/templ"
)

// render schreibt ein templ-Component als HTML.
//
// Ein Fehler beim Rendern kommt praktisch nur vor, wenn die Verbindung
// abbricht. Dann sind die Header schon raus und ein http.Error waere
// wirkungslos — deshalb wird nur geloggt.
func (s *Server) render(w http.ResponseWriter, r *http.Request, c templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if err := c.Render(r.Context(), w); err != nil {
		s.log.ErrorContext(r.Context(), "template rendern fehlgeschlagen",
			"path", r.URL.Path, "fehler", err)
	}
}
