package server

import (
	"context"
	"net/http"
)

// handleHealthz is the liveness probe: it answers only the question of
// whether the process is running, and deliberately does not touch the
// database. A database outage must not trigger a container restart.
func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writePlain(w, http.StatusOK, "ok")
}

// handleReadyz is the readiness probe: it reports ready only once the
// database is reachable. If the database fails, the load balancer takes the
// instance out of rotation without killing it.
func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), s.cfg.ReadinessTimeout)
	defer cancel()

	if err := s.store.Ping(ctx); err != nil {
		s.log.WarnContext(ctx, "readiness check failed", "error", err)
		writePlain(w, http.StatusServiceUnavailable, "database unreachable")
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
