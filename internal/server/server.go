// Package server holds HTTP routing, middleware and handlers.
//
// Handlers know neither SQL (invariant 5) nor auth providers (invariant 4):
// data access goes through repository.Store, and identity arrives as a
// player_id on the request context.
package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/stuttgart-things/schmetterpause/internal/auth"
	"github.com/stuttgart-things/schmetterpause/internal/config"
	"github.com/stuttgart-things/schmetterpause/internal/repository"
)

// Server bundles configuration, dependencies and routing.
type Server struct {
	cfg     config.Config
	store   repository.Store
	log     *slog.Logger
	auth    auth.SessionAuthenticator
	version string
	handler http.Handler
}

// New wires up the server. The authenticator is an interface so that later
// providers (OIDC, WebAuthn) take effect without touching any handler.
func New(cfg config.Config, store repository.Store, log *slog.Logger, a auth.SessionAuthenticator, version string) *Server {
	s := &Server{cfg: cfg, store: store, log: log, auth: a, version: version}
	s.handler = s.routes()
	return s
}

// Handler returns the fully wired HTTP handler.
func (s *Server) Handler() http.Handler { return s.handler }

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()

	// Liveness and readiness stay outside the auth middleware and render no
	// template on purpose: they must answer even when the application can do
	// nothing else.
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /readyz", s.handleReadyz)

	mux.Handle("GET /static/", staticHandler())

	// Browsers ask for this whether or not the pages link an icon, and a 404
	// on every first visit is noise in the log for no reason.
	mux.Handle("GET /favicon.ico", faviconHandler())

	page := http.NewServeMux()
	page.HandleFunc("GET /{$}", s.handleIndex)
	page.HandleFunc("GET /fragments/status", s.handleStatusFragment)
	page.HandleFunc("GET /fragments/whoami", s.handleWhoami)
	page.HandleFunc("GET /fragments/standings", s.handleStandingsFragment)
	page.HandleFunc("GET /players/{id}", s.handleProfile)
	page.HandleFunc("POST /players", s.handleJoin)
	page.HandleFunc("GET /fragments/match", s.handleMatchForm)
	// Not behind a player check: the kiosk has no player session and needs
	// the same rows.
	page.HandleFunc("GET /fragments/sets", s.handleSetsFragment)
	page.HandleFunc("POST /matches", s.handleRecordMatch)
	page.HandleFunc("GET /fragments/pending", s.handlePendingFragment)
	page.HandleFunc("POST /matches/{id}/confirm", s.handleConfirmMatch)
	page.HandleFunc("POST /matches/{id}/dispute", s.handleDisputeMatch)
	page.HandleFunc("POST /matches/{id}/correct", s.handleCorrectMatch)
	page.HandleFunc("GET /qr", s.handleQRSheet)
	// Unset token, no routes: the kiosk does not exist rather than existing
	// unlocked, and /kiosk is a 404 like any other unknown path.
	if s.cfg.KioskToken != "" {
		page.HandleFunc("GET /kiosk", s.handleKiosk)
		page.HandleFunc("POST /kiosk/players", s.handleKioskAddPlayer)
		page.HandleFunc("POST /kiosk/matches", s.handleKioskRecord)
	}

	mux.Handle("/", auth.Middleware(s.auth, s.log)(page))

	return recoverer(s.log)(requestLogger(s.log)(mux))
}

// Run starts the server and stops it gracefully once ctx is cancelled.
func (s *Server) Run(ctx context.Context) error {
	srv := &http.Server{
		Addr:              s.cfg.HTTPAddr,
		Handler:           s.handler,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
		BaseContext:       func(_ net.Listener) context.Context { return ctx },
	}

	errCh := make(chan error, 1)
	go func() {
		s.log.Info("server started", "addr", s.cfg.HTTPAddr, "version", s.version)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("http server: %w", err)
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	s.log.Info("shutting down", "grace", s.cfg.ShutdownTimeout)

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.cfg.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shut down server: %w", err)
	}
	return <-errCh
}
