// Command schmetterpause is the project's only binary.
//
// It contains the HTTP server, the embedded templates and assets, and the
// database migrations. The same binary and the same image run in Docker
// Compose, Kubernetes and Azure Container Apps (invariant 1); the only thing
// that differs between them is environment variables (invariant 2).
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/stuttgart-things/schmetterpause/internal/auth"
	"github.com/stuttgart-things/schmetterpause/internal/config"
	"github.com/stuttgart-things/schmetterpause/internal/repository/postgres"
	"github.com/stuttgart-things/schmetterpause/internal/server"
)

// version is set at build time via -ldflags (git tag or commit SHA).
var version = "dev"

const usage = `schmetterpause — office table tennis

Usage:
  schmetterpause serve            start the HTTP server (default)
  schmetterpause migrate up       apply pending migrations
  schmetterpause migrate down     roll back the last migration (local only)
  schmetterpause migrate status   show the migration state
  schmetterpause healthcheck      probe own readiness (for container health checks)
  schmetterpause version          print the version

Configuration comes exclusively from environment variables:
  SP_DATABASE_URL       Postgres DSN (required)
  SP_HTTP_ADDR          bind address, default ":8080"
  SP_LOG_LEVEL          debug | info | warn | error, default "info"
  SP_AUTO_MIGRATE       apply migrations at startup, default "true"
  SP_SHUTDOWN_TIMEOUT   grace period for in-flight requests, default "15s"
  SP_READINESS_TIMEOUT  bound on the database check, default "2s"
  SP_DB_CONNECT_TIMEOUT how long startup waits for the database, default "30s"
  SP_SESSION_KEY        key that signs the recognition cookie; required by
                        serve, at least 32 characters
                        (openssl rand -base64 32)
  SP_COOKIE_SECURE      send the cookie over HTTPS only, default "true"
`

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, os.Args[1:]); err != nil {
		// The logger is not guaranteed to exist at this point, so stderr.
		fmt.Fprintf(os.Stderr, "schmetterpause: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	command := "serve"
	if len(args) > 0 {
		command = args[0]
	}

	switch command {
	case "serve":
		return serve(ctx)
	case "migrate":
		return migrate(ctx, args[1:])
	case "healthcheck":
		return healthcheck(ctx)
	case "version":
		fmt.Println(version)
		return nil
	case "help", "-h", "--help":
		fmt.Print(usage)
		return nil
	default:
		return fmt.Errorf("unknown command %q\n\n%s", command, usage)
	}
}

func serve(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if err := cfg.ValidateForServe(); err != nil {
		return err
	}

	log := newLogger(cfg.LogLevel)

	store, err := postgres.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer store.Close()

	if err := waitForDatabase(ctx, store, cfg.DatabaseConnectTimeout, log); err != nil {
		return err
	}

	if cfg.AutoMigrate {
		log.InfoContext(ctx, "applying migrations")
		if err := postgres.Migrate(ctx, cfg.DatabaseURL); err != nil {
			return err
		}
	}

	// The only provider in the MVP. OIDC and WebAuthn arrive as further
	// implementations of the same interface, without a handler changing
	// (docs/adr/0003).
	sessions := auth.NewCookieAuthenticator(store.Identities(), cfg.SessionKey, cfg.CookieSecure)

	srv := server.New(cfg, store, log, sessions, version)

	if err := srv.Run(ctx); err != nil {
		return err
	}
	log.Info("server stopped")
	return nil
}

func migrate(ctx context.Context, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	direction := "up"
	if len(args) > 0 {
		direction = args[0]
	}

	switch direction {
	case "up":
		return postgres.Migrate(ctx, cfg.DatabaseURL)
	case "down":
		return postgres.MigrateDown(ctx, cfg.DatabaseURL)
	case "status":
		return postgres.MigrationStatus(ctx, cfg.DatabaseURL)
	default:
		return errors.New(`migrate expects "up", "down" or "status"`)
	}
}

// waitForDatabase waits until the database accepts connections.
//
// Without this loop the application starts reliably only where the startup
// order can be enforced — in Compose, through depends_on. Kubernetes and
// Azure Container Apps have no such thing; there, a database that becomes
// ready moments later is the normal case and not a fault.
func waitForDatabase(ctx context.Context, store *postgres.Store, timeout time.Duration, log *slog.Logger) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	const (
		firstDelay = 250 * time.Millisecond
		maxDelay   = 5 * time.Second
	)

	delay := firstDelay
	for attempt := 1; ; attempt++ {
		err := store.Ping(ctx)
		if err == nil {
			if attempt > 1 {
				log.Info("database reachable", "attempts", attempt)
			}
			return nil
		}

		if ctx.Err() != nil {
			return fmt.Errorf("database not reachable within %s: %w", timeout, err)
		}

		log.Warn("database not reachable yet, retrying",
			"attempt", attempt, "delay", delay, "error", err)

		select {
		case <-ctx.Done():
			return fmt.Errorf("database not reachable within %s: %w", timeout, err)
		case <-time.After(delay):
		}

		delay = min(2*delay, maxDelay)
	}
}

// healthcheck probes the application's own readiness endpoint over loopback.
//
// The distroless runtime image has neither a shell nor curl. So that Compose
// and other runtimes can still check the state, the binary brings the check
// along itself rather than pulling a second tool into the image.
func healthcheck(ctx context.Context) error {
	addr := os.Getenv("SP_HTTP_ADDR")
	if addr == "" {
		addr = config.DefaultHTTPAddr
	}

	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("SP_HTTP_ADDR %q is not a bind address: %w", addr, err)
	}
	// A wildcard binding becomes loopback for the self-call.
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	url := "http://" + net.JoinHostPort(host, port) + "/readyz"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build healthcheck request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("healthcheck against %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("readyz reports %s", resp.Status)
	}
	return nil
}

func newLogger(level slog.Level) *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}
