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
	"github.com/stuttgart-things/schmetterpause/internal/domain"
	"github.com/stuttgart-things/schmetterpause/internal/metrics"
	"github.com/stuttgart-things/schmetterpause/internal/ratelimit"
	"github.com/stuttgart-things/schmetterpause/internal/repository"
)

// Server bundles configuration, dependencies and routing.
type Server struct {
	cfg     config.Config
	store   repository.Store
	log     *slog.Logger
	auth    auth.SessionAuthenticator
	build   Build
	handler http.Handler
	// started is when this process read its configuration. /admin shows it
	// beside the settings, because a changed variable only counts after a
	// restart and the page should not claim a value it has not read yet.
	started time.Time
	// metrics is nil unless SP_METRICS_ADDR is set, and then nothing is
	// measured and no second port is opened.
	metrics *metrics.Metrics

	// The two halves of the brake on guessing at a credential. One alone is
	// not a limit: per player, somebody walks the roster; per address,
	// a second phone starts over. See signin.go for the policies.
	signInByPlayer  *ratelimit.Limiter
	signInByAddress *ratelimit.Limiter
	// The brake on guessing at the kiosk code. One dimension only, because
	// there is only one secret: no player half carries the weight here, so
	// this half has to.
	kioskByAddress *ratelimit.Limiter
}

// Build is what this binary can say about itself, set at link time.
//
// Two values rather than one string, so the page can show them as two things:
// which commit, and how old it is. CommitTime is when that commit was made,
// not when the binary was built — a build clock would make two images from
// one commit differ, and "is the same thing running here and there" is the
// question this exists to answer.
type Build struct {
	Version    string
	CommitTime string
}

// New wires up the server. The authenticator is an interface so that later
// providers (OIDC, WebAuthn) take effect without touching any handler.
func New(cfg config.Config, store repository.Store, log *slog.Logger, a auth.SessionAuthenticator, build Build) *Server {
	s := &Server{
		cfg: cfg, store: store, log: log, auth: a, build: build,
		started:         time.Now(),
		signInByPlayer:  ratelimit.New(signInPlayerPolicy),
		signInByAddress: ratelimit.New(signInAddressPolicy),
		kioskByAddress:  ratelimit.New(kioskPolicy),
	}
	if cfg.MetricsAddr != "" {
		s.metrics = metrics.New(build.Version,
			metrics.WithUnsettled(s.unsettledSummary, unsettledQueryTimeout))
	}
	s.handler = s.routes()
	return s
}

// unsettledQueryTimeout bounds the query behind the unsettled-results metric.
// Far below any scrape timeout, so a slow database costs this one series and
// not the whole scrape.
const unsettledQueryTimeout = 2 * time.Second

// unsettledSummary is the metric's view of the store, with the failure logged
// here: the collector can only say that it failed, not why.
func (s *Server) unsettledSummary(ctx context.Context) ([]domain.UnsettledGroup, error) {
	groups, err := s.store.Matches().UnsettledSummary(ctx)
	if err != nil {
		s.log.ErrorContext(ctx, "counting unsettled matches for /metrics failed", "error", err)
		return nil, fmt.Errorf("count unsettled matches: %w", err)
	}
	return groups, nil
}

// Handler returns the fully wired HTTP handler.
func (s *Server) Handler() http.Handler { return s.handler }

// MetricsHandler is what the metrics listener serves, or nil when
// SP_METRICS_ADDR is unset.
func (s *Server) MetricsHandler() http.Handler {
	if s.metrics == nil {
		return nil
	}
	return s.metrics.Handler()
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()

	// Liveness and readiness stay outside the auth middleware and render no
	// template on purpose: they must answer even when the application can do
	// nothing else.
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /readyz", s.handleReadyz)
	// Which build is answering, in one line. Beside the probes because it is
	// the same kind of thing — a question about the process rather than about
	// the office — and outside the auth middleware for the same reason they
	// are: whatever asks is a monitor, not a player (issue #229).
	mux.HandleFunc("GET /version", s.handleVersion)

	// The Zählwerk's surface (docs/adr/0015). On the outer mux rather than on
	// page, deliberately: these routes answer a machine holding a token, so
	// they have no business passing through the session middleware, reading a
	// recognition cookie or rendering a template. Same reasoning that keeps
	// the probes and /version out here.
	//
	// Unset token, no routes: /api/players is a 404 like any other unknown
	// path, exactly as /kiosk is without SP_KIOSK_TOKEN.
	if s.cfg.ScoreboardToken != "" {
		mux.HandleFunc("GET /api/players", s.handleAPIPlayers)
		mux.HandleFunc("POST /api/results", s.handleAPIResults)
	}

	mux.Handle("GET /static/", staticHandler())

	// Browsers ask for this whether or not the pages link an icon, and a 404
	// on every first visit is noise in the log for no reason.
	mux.Handle("GET /favicon.ico", faviconHandler())

	page := http.NewServeMux()
	page.HandleFunc("GET /{$}", s.handleIndex)
	page.HandleFunc("GET /info", s.handleInfo)
	page.HandleFunc("GET /fragments/status", s.handleStatusFragment)
	page.HandleFunc("GET /fragments/whoami", s.handleWhoami)
	page.HandleFunc("GET /standings", s.handleStandings)
	page.HandleFunc("GET /fragments/standings", s.handleStandingsFragment)
	page.HandleFunc("GET /fragments/refresh", s.handleRefresh)
	// What is on at the table, for the start page's own slow poll. Not
	// behind a player check: an open tournament is public on /tournaments,
	// and a reader nobody is recognised as simply sees no row marked as
	// theirs.
	page.HandleFunc("GET /fragments/running", s.handleRunningFragment)
	page.HandleFunc("GET /players/{id}", s.handleProfile)
	page.HandleFunc("POST /players", s.handleJoin)
	// The way back for a browser that lost its cookie (issue #70). Both
	// fragments serve the same region, so the two ways in replace each other
	// on the start page rather than sitting side by side.
	page.HandleFunc("GET /fragments/signin", s.handleSignInForm)
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
	// Both only for a tournament nobody has played in: the draw is a function
	// of the stored order, and matches outlive the bracket they were played
	// in (docs/adr/0010, docs/adr/0011).
	page.Handle("POST /tournaments/{id}/delete",
		auth.RequirePlayer(http.HandlerFunc(s.handleDeleteTournament)))
	page.Handle("POST /tournaments/{id}/edit",
		auth.RequirePlayer(http.HandlerFunc(s.handleEditTournament)))
	page.HandleFunc("GET /qr", s.handleQRSheet)
	page.HandleFunc("GET /rules", s.handleRulesSheet)
	// Who may act for other people (docs/adr/0008). Behind the flag itself:
	// the list is the record of who holds power over other people's records,
	// and that is not a public page.
	admin := auth.RequireAdmin(s.isAdmin, s.log)
	page.Handle("GET /admin", admin(http.HandlerFunc(s.handleAdmin)))
	// Taking a counted result back, which is what a correction is: there is
	// nothing left to edit in a settled match, so the wrong one goes and the
	// right one is entered normally (issue #105).
	page.Handle("POST /admin/matches/{id}/remove", admin(http.HandlerFunc(s.handleAdminRemoveMatch)))
	// The joke entry and the duplicate created before anybody played. The
	// schema refuses everybody else, which is what keeps this small (#105).
	page.Handle("POST /admin/players/{id}/remove", admin(http.HandlerFunc(s.handleAdminRemovePlayer)))
	// Somebody who does not play, and back (docs/adr/0022). Also on your own
	// row: the account this exists for flags itself.
	page.Handle("POST /admin/players/{id}/observer", admin(http.HandlerFunc(s.handleAdminSetObserver)))
	// Taking a kiosk machine back belongs to somebody, which is what
	// docs/adr/0008 settled and what issue #77 was waiting for. POST, so a
	// link nobody meant to follow cannot do it.
	page.Handle("POST /admin/kiosk/{id}/revoke", admin(http.HandlerFunc(s.handleRevokeKiosk)))
	page.Handle("POST /admin/kiosk/revoke-all", admin(http.HandlerFunc(s.handleRevokeAllKiosks)))
	// Unset token, no routes: the kiosk does not exist rather than existing
	// unlocked, and /kiosk is a 404 like any other unknown path.
	if s.cfg.KioskToken != "" {
		page.HandleFunc("GET /kiosk", s.handleKiosk)
		// The code goes in a form rather than a query string, so it does not
		// end up in the browser history of a laptop somebody borrows next.
		page.HandleFunc("POST /kiosk/unlock", s.handleKioskUnlock)
		// Who is typing. It comes before everything below it: an unlocked
		// machine that has not answered may not write anything (issue #90).
		page.HandleFunc("POST /kiosk/operator", s.handleKioskOperator)
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

	handler := recoverer(s.log)(requestLogger(s.log)(http.Handler(mux)))
	// Outside the recoverer, so a panic is counted as the 500 it becomes.
	if s.metrics != nil {
		handler = s.metrics.Middleware(routeOf(mux, page))(handler)
	}
	return handler
}

// routeOf names the pattern a request is served under, asking the muxes
// rather than letting them set it: the auth middleware hands the page mux a
// copy of the request, so the pattern it records never reaches a middleware
// wrapped around the outer one.
func routeOf(outer, page *http.ServeMux) func(*http.Request) string {
	return func(r *http.Request) string {
		_, pattern := outer.Handler(r)
		if pattern == "/" {
			_, pattern = page.Handler(r)
		}
		return pattern
	}
}

// Run starts the server — and the metrics listener, when there is one — and
// stops both gracefully once ctx is cancelled.
func (s *Server) Run(ctx context.Context) error {
	servers := []*http.Server{newHTTPServer(ctx, s.cfg.HTTPAddr, s.handler)}
	if s.metrics != nil {
		servers = append(servers, newHTTPServer(ctx, s.cfg.MetricsAddr, s.metrics.Handler()))
	}

	errCh := make(chan error, len(servers))
	for _, srv := range servers {
		go func() {
			if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errCh <- fmt.Errorf("http server on %s: %w", srv.Addr, err)
				return
			}
			errCh <- nil
		}()
	}

	s.log.Info("server started", "addr", s.cfg.HTTPAddr,
		"version", s.build.Version, "commit_time", s.build.CommitTime)
	if s.metrics != nil {
		s.log.Info("metrics listener started", "addr", s.cfg.MetricsAddr)
	}

	// Either listener failing stops both. A process that answers players but
	// cannot be measured is not running the way it was deployed, and a
	// crash-loop says so where a log line would be read past.
	received := 0
	var failed error
	select {
	case failed = <-errCh:
		received = 1
	case <-ctx.Done():
		s.log.Info("shutting down", "grace", s.cfg.ShutdownTimeout)
	}

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.cfg.ShutdownTimeout)
	defer cancel()

	errs := []error{failed}
	for _, srv := range servers {
		if err := srv.Shutdown(shutdownCtx); err != nil {
			errs = append(errs, fmt.Errorf("shut down server on %s: %w", srv.Addr, err))
		}
	}
	for ; received < len(servers); received++ {
		errs = append(errs, <-errCh)
	}
	return errors.Join(errs...)
}

func newHTTPServer(ctx context.Context, addr string, h http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
		BaseContext:       func(_ net.Listener) context.Context { return ctx },
	}
}
