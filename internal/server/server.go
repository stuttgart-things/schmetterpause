// Package server enthaelt HTTP-Routing, Middleware und Handler.
//
// Handler kennen weder SQL (Invariante 5) noch Auth-Provider (Invariante 4):
// Datenzugriff laeuft ueber repository.Store, die Identitaet kommt als
// player_id aus dem Context.
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

// Server buendelt Konfiguration, Abhaengigkeiten und Routing.
type Server struct {
	cfg     config.Config
	store   repository.Store
	log     *slog.Logger
	auth    auth.Authenticator
	version string
	handler http.Handler
}

// New verdrahtet den Server. Der Authenticator ist eine Schnittstelle, damit
// spaetere Provider (OIDC, WebAuthn) ohne Aenderung an Handlern greifen.
func New(cfg config.Config, store repository.Store, log *slog.Logger, a auth.Authenticator, version string) *Server {
	s := &Server{cfg: cfg, store: store, log: log, auth: a, version: version}
	s.handler = s.routes()
	return s
}

// Handler liefert den fertig verdrahteten HTTP-Handler.
func (s *Server) Handler() http.Handler { return s.handler }

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()

	// Liveness und Readiness bleiben bewusst ausserhalb der Auth-Middleware
	// und rendern kein Template: sie muessen auch dann antworten, wenn die
	// Anwendung sonst nichts mehr kann.
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /readyz", s.handleReadyz)

	mux.Handle("GET /static/", staticHandler())

	page := http.NewServeMux()
	page.HandleFunc("GET /{$}", s.handleIndex)
	page.HandleFunc("GET /fragments/status", s.handleStatusFragment)
	mux.Handle("/", auth.Middleware(s.auth)(page))

	return recoverer(s.log)(requestLogger(s.log)(mux))
}

// Run startet den Server und beendet ihn geordnet, sobald ctx abgebrochen wird.
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
		s.log.Info("server gestartet", "addr", s.cfg.HTTPAddr, "version", s.version)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("http-server: %w", err)
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	s.log.Info("server wird beendet", "frist", s.cfg.ShutdownTimeout)

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.cfg.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server beenden: %w", err)
	}
	return <-errCh
}
