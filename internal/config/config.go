// Package config reads the runtime configuration from the environment.
//
// Invariant 2 in CLAUDE.md: configuration comes exclusively from environment
// variables. There is no config file in the image, and defaults live here in
// code rather than in a shipped file.
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

// envPrefix keeps this application's variables apart from anyone else's.
const envPrefix = "SP_"

// DefaultHTTPAddr is the bind address used when SP_HTTP_ADDR is unset. It
// lives here rather than in a config file or the Dockerfile (invariant 2),
// and the healthcheck subcommand needs it too.
const DefaultHTTPAddr = ":8080"

// Config is the complete runtime configuration.
type Config struct {
	// HTTPAddr is the server's bind address, for example ":8080".
	HTTPAddr string
	// DatabaseURL is the Postgres DSN. Without it the application refuses to
	// start; a default would mean a hardcoded host and break invariant 2.
	DatabaseURL string
	// LogLevel drives the structured logger.
	LogLevel slog.Level
	// AutoMigrate applies pending migrations at startup. Convenient in
	// Compose and for single-replica deployments; with several replicas,
	// prefer turning it off and running "schmetterpause migrate up" as its
	// own step.
	AutoMigrate bool
	// ShutdownTimeout is how long in-flight requests get when stopping.
	ShutdownTimeout time.Duration
	// ReadinessTimeout bounds the database check behind /readyz.
	ReadinessTimeout time.Duration
	// DatabaseConnectTimeout is how long startup waits for the database
	// before giving up. Compose can wait for a healthy Postgres through
	// depends_on; Kubernetes and Azure Container Apps cannot — there the
	// application would crash-loop merely because the database is ready a few
	// seconds later.
	DatabaseConnectTimeout time.Duration
}

// Load reads the configuration from the environment and validates it.
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
			errs = append(errs, fmt.Errorf("%sAUTO_MIGRATE=%q is not a boolean: %w", envPrefix, raw, err))
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
			errs = append(errs, fmt.Errorf("%s%s=%q is not a duration: %w", envPrefix, d.key, raw, err))
			continue
		}
		if v <= 0 {
			errs = append(errs, fmt.Errorf("%s%s must be positive, got %s", envPrefix, d.key, v))
			continue
		}
		*d.target = v
	}

	if cfg.HTTPAddr == "" {
		errs = append(errs, fmt.Errorf("%sHTTP_ADDR must not be empty", envPrefix))
	}
	if cfg.DatabaseURL == "" {
		errs = append(errs, fmt.Errorf("%sDATABASE_URL is required", envPrefix))
	}

	if err := errors.Join(errs...); err != nil {
		return Config{}, fmt.Errorf("read configuration from the environment: %w", err)
	}
	return cfg, nil
}

func parseLevel(raw string) (slog.Level, error) {
	var level slog.Level
	if err := level.UnmarshalText([]byte(strings.ToUpper(raw))); err != nil {
		return slog.LevelInfo, fmt.Errorf("%sLOG_LEVEL=%q is not a valid level: %w", envPrefix, raw, err)
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
