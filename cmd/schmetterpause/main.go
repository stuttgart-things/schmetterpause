// Command schmetterpause ist das einzige Binary des Projekts.
//
// Es enthaelt den HTTP-Server, die eingebetteten Templates und Assets sowie
// die Datenbank-Migrations. Dasselbe Binary und dasselbe Image laufen in
// Docker Compose, Kubernetes und Azure Container Apps (Invariante 1); was sich
// unterscheidet, sind ausschliesslich Environment-Variablen (Invariante 2).
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

// version wird beim Build via -ldflags gesetzt (Git-Tag bzw. Commit-SHA).
var version = "dev"

const usage = `schmetterpause — Büro-Tischtennis

Aufrufe:
  schmetterpause serve            HTTP-Server starten (Vorgabe)
  schmetterpause migrate up       Ausstehende Migrations anwenden
  schmetterpause migrate down     Letzte Migration zurücknehmen (nur lokal)
  schmetterpause migrate status   Stand der Migrations anzeigen
  schmetterpause healthcheck      Eigene Readiness prüfen (für Container-Healthchecks)
  schmetterpause version          Version ausgeben

Konfiguration ausschließlich über Environment-Variablen:
  SP_DATABASE_URL       Postgres-DSN (erforderlich)
  SP_HTTP_ADDR          Bind-Adresse, Vorgabe ":8080"
  SP_LOG_LEVEL          debug | info | warn | error, Vorgabe "info"
  SP_AUTO_MIGRATE       Migrations beim Start anwenden, Vorgabe "true"
  SP_SHUTDOWN_TIMEOUT   Frist für laufende Requests, Vorgabe "15s"
  SP_READINESS_TIMEOUT  Frist für den Datenbank-Check, Vorgabe "2s"
  SP_DB_CONNECT_TIMEOUT Frist, bis die Datenbank beim Start da sein muss,
                        Vorgabe "30s"
`

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, os.Args[1:]); err != nil {
		// Der Logger steht an dieser Stelle nicht zwingend, deshalb stderr.
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
		return fmt.Errorf("unbekannter Aufruf %q\n\n%s", command, usage)
	}
}

func serve(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
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
		log.InfoContext(ctx, "migrations werden angewendet")
		if err := postgres.Migrate(ctx, cfg.DatabaseURL); err != nil {
			return err
		}
	}

	// Im Geruest erkennt niemand einen Spieler. AP2 ersetzt Anonymous durch
	// die Cookie-Wiedererkennung, ohne dass ein Handler sich aendert.
	srv := server.New(cfg, store, log, auth.Anonymous{}, version)

	if err := srv.Run(ctx); err != nil {
		return err
	}
	log.Info("server beendet")
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
		return errors.New(`migrate erwartet "up", "down" oder "status"`)
	}
}

// waitForDatabase wartet, bis die Datenbank Verbindungen annimmt.
//
// Ohne diese Schleife startet die Anwendung nur dort zuverlaessig, wo die
// Startreihenfolge erzwungen werden kann — in Compose ueber depends_on. In
// Kubernetes und Azure Container Apps gibt es das nicht; dort ist eine kurz
// spaeter bereite Datenbank der Normalfall und kein Fehler.
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
				log.Info("datenbank erreichbar", "versuche", attempt)
			}
			return nil
		}

		if ctx.Err() != nil {
			return fmt.Errorf("datenbank binnen %s nicht erreichbar: %w", timeout, err)
		}

		log.Warn("datenbank noch nicht erreichbar, neuer versuch",
			"versuch", attempt, "wartezeit", delay, "fehler", err)

		select {
		case <-ctx.Done():
			return fmt.Errorf("datenbank binnen %s nicht erreichbar: %w", timeout, err)
		case <-time.After(delay):
		}

		delay = min(2*delay, maxDelay)
	}
}

// healthcheck fragt die eigene Readiness-Probe ueber Loopback ab.
//
// Das distroless-Laufzeitimage hat weder Shell noch curl. Damit Compose und
// andere Runtimes den Zustand trotzdem pruefen koennen, bringt das Binary den
// Check selbst mit — ohne ein zweites Werkzeug ins Image zu holen.
func healthcheck(ctx context.Context) error {
	addr := os.Getenv("SP_HTTP_ADDR")
	if addr == "" {
		addr = config.DefaultHTTPAddr
	}

	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("SP_HTTP_ADDR %q ist keine Bind-Adresse: %w", addr, err)
	}
	// Eine Wildcard-Bindung wird fuer den Selbstaufruf zu Loopback.
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	url := "http://" + net.JoinHostPort(host, port) + "/readyz"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("healthcheck-anfrage bauen: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("healthcheck gegen %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("readyz meldet %s", resp.Status)
	}
	return nil
}

func newLogger(level slog.Level) *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}
