package server

import (
	"context"
	"net/http"
)

// handleHealthz ist die Liveness-Probe: Sie beantwortet ausschliesslich die
// Frage, ob der Prozess laeuft, und fasst die Datenbank bewusst nicht an. Ein
// DB-Ausfall darf keinen Neustart des Containers ausloesen.
func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writePlain(w, http.StatusOK, "ok")
}

// handleReadyz ist die Readiness-Probe: Sie meldet erst dann bereit, wenn die
// Datenbank erreichbar ist. Faellt sie aus, nimmt der Load Balancer die
// Instanz aus dem Verkehr, ohne sie zu toeten.
func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), s.cfg.ReadinessTimeout)
	defer cancel()

	if err := s.store.Ping(ctx); err != nil {
		s.log.WarnContext(ctx, "readiness fehlgeschlagen", "fehler", err)
		writePlain(w, http.StatusServiceUnavailable, "datenbank nicht erreichbar")
		return
	}
	writePlain(w, http.StatusOK, "ready")
}

func writePlain(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body + "\n"))
}
