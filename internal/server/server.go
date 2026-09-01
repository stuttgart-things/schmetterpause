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
	"github.com/stuttgart-things/schmetterpause/internal/ratelimit"
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

	// The two halves of the brake on guessing at a credential. One alone is
	// not a limit: per player, somebody walks the roster; per address,
	// a second phone starts over. See signin.go for the policies.
	signInByPlayer  *ratelimit.Limiter
	signInByAddress *ratelimit.Limiter
}

// New wires up the server. The authenticator is an interface so that later
// providers (OIDC, WebAuthn) take effect without touching any handler.
func New(cfg config.Config, store repository.Store, log *slog.Logger, a auth.SessionAuthenticator, version string) *Server {
	s := &Server{
		cfg: cfg, store: store, log: log, auth: a, version: version,
		signInByPlayer:  ratelimit.New(signInPlayerPolicy),
		signInByAddress: ratelimit.New(signInAddressPolicy),
	}
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
	page.HandleFunc("GET /fragments/refresh", s.handleRefresh)
	page.HandleFunc("GET /players/{id}", s.handleProfile)
	page.HandleFunc("POST /players", s.handleJoin)
	// The way back for a browser that lost its cookie (issue #70). Both
	// fragments serve the same region, so the two ways in replace each other
	// on the start page rather than sitting side by side.
	page.HandleFunc("GET /fragments/signin", s.handleSignInForm)
	page.HandleFunc("GET /fragments/signin-secret", s.handleSignInSecret)
	page.HandleFunc("GET /fragments/join", s.handleJoinForm)
	page.HandleFunc("POST /signin", s.handleSignIn)
	// Only ever for yourself, which is what RequirePlayer says here. Issuing
	// a credential for somebody else is the kiosk's job and nobody else's
	// (docs/adr/0006).
	page.Handle("POST /credentials/pin", auth.RequirePlayer(http.HandlerFunc(s.handleSetPIN)))
	page.Handle("POST /credentials/recovery", auth.RequirePlayer(http.HandlerFunc(s.handleNewRecoveryCode)))
	// POST, never GET. Chat programs follow links to build previews, so a
	// GET /signout is a URL somebody pastes into Teams that signs people out.
	page.Handle("POST /signout", auth.RequirePlayer(http.HandlerFunc(s.handleSignOut)))
	page.HandleFunc("GET /fragments/match", s.handleMatchForm)
	// Not behind a player check: the kiosk has no player session and needs
	// the same rows.
	page.HandleFunc("GET /fragments/sets", s.handleSetsFragment)
	page.HandleFunc("POST /matches", s.handleRecordMatch)
	page.HandleFunc("GET /matches", s.handleMatchList)
	page.HandleFunc("GET /fragments/pending", s.handlePendingFragment)
	page.HandleFunc("POST /matches/{id}/confirm", s.handleConfirmMatch)
	page.HandleFunc("POST /matches/{id}/dispute", s.handleDisputeMatch)
	page.HandleFunc("POST /matches/{id}/correct", s.handleCorrectMatch)
	page.HandleFunc("GET /statistics", s.handleStatistics)
	page.HandleFunc("GET /tournaments", s.handleTournaments)
	page.HandleFunc("POST /tournaments", s.handleCreateTournament)
	page.HandleFunc("GET /fragments/tournament-size", s.handleTournamentSize)
	page.HandleFunc("GET /tournaments/{id}", s.handleTournament)
	page.HandleFunc("POST /tournaments/{id}/close", s.handleCloseTournament)
	page.HandleFunc("GET /qr", s.handleQRSheet)
	// Who may act for other people (docs/adr/0008). Behind the flag itself:
	// the list is the record of who holds power over other people's records,
	// and that is not a public page.
	admin := auth.RequireAdmin(s.isAdmin, s.log)
	page.Handle("GET /admin", admin(http.HandlerFunc(s.handleAdmin)))
	// Taking a kiosk machine back belongs to somebody, which is what
	// docs/adr/0008 settled and what issue #77 was waiting for. POST, so a
	// link nobody meant to follow cannot do it.
	page.Handle("POST /admin/kiosk/{id}/revoke", admin(http.HandlerFunc(s.handleRevokeKiosk)))
	page.Handle("POST /admin/kiosk/revoke-all", admin(http.HandlerFunc(s.handleRevokeAllKiosks)))
	// Unset token, no routes: the kiosk does not exist rather than existing
	// unlocked, and /kiosk is a 404 like any other unknown path.
	if s.cfg.KioskToken != "" {
		page.HandleFunc("GET /kiosk", s.handleKiosk)
		page.HandleFunc("POST /kiosk/players", s.handleKioskAddPlayer)
		page.HandleFunc("POST /kiosk/credentials", s.handleKioskIssueCode)
		page.HandleFunc("POST /kiosk/matches", s.handleKioskRecord)
		page.HandleFunc("POST /kiosk/matches/{id}/undo", s.handleKioskUndo)
		// The same tournament page, served from under /kiosk so the machine
		// at the table reaches it with its cookie.
		//
		// The cookie is scoped Path=/kiosk deliberately, so it is simply not
		// sent to /tournaments/{id} — a page there can never know it is the
		// table, however it asks. Rather than widening that scope, the entry
		// view lives where the cookie already goes. The draw and the table
		// stay readable for everybody at /tournaments/{id}; what is behind
		// /kiosk is the boxes to type into.
		page.HandleFunc("GET /kiosk/tournaments/{id}", s.handleTournament)
		page.HandleFunc("POST /kiosk/tournaments/{id}/matches", s.handleTournamentRecord)
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
