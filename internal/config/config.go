// Package config liest die Laufzeitkonfiguration aus der Umgebung.
//
// Invariante 2 aus CLAUDE.md: Konfiguration ausschliesslich ueber
// Environment-Variablen. Es gibt keine Config-Datei im Image, und Defaults
// stehen hier im Code — nicht in einer mitgelieferten Datei.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

// envPrefix haelt die Variablen dieser Anwendung von fremden auseinander.
const envPrefix = "SP_"

// DefaultHTTPAddr ist die Bind-Adresse, wenn SP_HTTP_ADDR nicht gesetzt ist.
// Sie steht hier und nicht in einer Config-Datei oder im Dockerfile
// (Invariante 2) und wird ausserdem vom healthcheck-Aufruf gebraucht.
const DefaultHTTPAddr = ":8080"

// Config ist die vollstaendige Laufzeitkonfiguration.
type Config struct {
	// HTTPAddr ist die Bind-Adresse des Servers, etwa ":8080".
	HTTPAddr string
	// DatabaseURL ist die Postgres-DSN. Ohne sie startet die Anwendung nicht;
	// ein Default waere ein hartkodierter Host und damit ein Invariantenbruch.
	DatabaseURL string
	// LogLevel steuert den strukturierten Logger.
	LogLevel slog.Level
	// AutoMigrate wendet ausstehende Migrations beim Start an. Praktisch in
	// Compose und fuer Einzel-Replica-Deployments; bei mehreren Replicas
	// besser abschalten und "schmetterpause migrate up" als eigenen Schritt
	// fahren.
	AutoMigrate bool
	// ShutdownTimeout ist die Frist fuer laufende Requests beim Beenden.
	ShutdownTimeout time.Duration
	// ReadinessTimeout begrenzt den Datenbank-Check hinter /readyz.
	ReadinessTimeout time.Duration
	// DatabaseConnectTimeout ist die Frist, die der Start der Datenbank
	// einraeumt, bevor er aufgibt. Compose kann ueber depends_on auf einen
	// gesunden Postgres warten, Kubernetes und Azure Container Apps koennen
	// das nicht — dort wuerde die App sonst in einen Crash-Loop laufen, nur
	// weil die Datenbank ein paar Sekunden spaeter bereit ist.
	DatabaseConnectTimeout time.Duration
}

// Load liest die Konfiguration aus der Umgebung und validiert sie.
func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:               env("HTTP_ADDR", DefaultHTTPAddr),
		DatabaseURL:            env("DATABASE_URL", ""),
		AutoMigrate:            true,
		ShutdownTimeout:        15 * time.Second,
		ReadinessTimeout:       2 * time.Second,
		DatabaseConnectTimeout: 30 * time.Second,
	}

	var errs []error

	level, err := parseLevel(env("LOG_LEVEL", "info"))
	if err != nil {
		errs = append(errs, err)
	}
	cfg.LogLevel = level

	if raw, ok := lookup("AUTO_MIGRATE"); ok {
		v, err := strconv.ParseBool(raw)
		if err != nil {
			errs = append(errs, fmt.Errorf("%sAUTO_MIGRATE=%q ist kein boolescher Wert: %w", envPrefix, raw, err))
		}
		cfg.AutoMigrate = v
	}

	for _, d := range []struct {
		key    string
		target *time.Duration
	}{
		{"SHUTDOWN_TIMEOUT", &cfg.ShutdownTimeout},
		{"READINESS_TIMEOUT", &cfg.ReadinessTimeout},
		{"DB_CONNECT_TIMEOUT", &cfg.DatabaseConnectTimeout},
	} {
		raw, ok := lookup(d.key)
		if !ok {
			continue
		}
		v, err := time.ParseDuration(raw)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s%s=%q ist keine Dauer: %w", envPrefix, d.key, raw, err))
			continue
		}
		if v <= 0 {
			errs = append(errs, fmt.Errorf("%s%s muss positiv sein, ist %s", envPrefix, d.key, v))
			continue
		}
		*d.target = v
	}

	if cfg.HTTPAddr == "" {
		errs = append(errs, fmt.Errorf("%sHTTP_ADDR darf nicht leer sein", envPrefix))
	}
	if cfg.DatabaseURL == "" {
		errs = append(errs, fmt.Errorf("%sDATABASE_URL ist erforderlich", envPrefix))
	}

	if err := errors.Join(errs...); err != nil {
		return Config{}, fmt.Errorf("konfiguration aus der umgebung lesen: %w", err)
	}
	return cfg, nil
}

func parseLevel(raw string) (slog.Level, error) {
	var level slog.Level
	if err := level.UnmarshalText([]byte(strings.ToUpper(raw))); err != nil {
		return slog.LevelInfo, fmt.Errorf("%sLOG_LEVEL=%q ist kein gueltiger Level: %w", envPrefix, raw, err)
	}
	return level, nil
}

func lookup(key string) (string, bool) {
	v, ok := os.LookupEnv(envPrefix + key)
	if !ok {
		return "", false
	}
	return strings.TrimSpace(v), true
}

func env(key, fallback string) string {
	if v, ok := lookup(key); ok && v != "" {
		return v
	}
	return fallback
}
