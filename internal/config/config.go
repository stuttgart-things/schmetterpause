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
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// envPrefix keeps this application's variables apart from anyone else's.
const envPrefix = "SP_"

// minSessionKeyLen is the shortest session key accepted. HMAC-SHA256 keys
// shorter than the hash are not worth the trouble of attacking, but they are
// also not worth accepting when generating a longer one is one command.
const minSessionKeyLen = 32

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
	// SessionKey signs the recognition cookie. It has no default: a
	// hardcoded fallback would let anyone forge a session, and a random one
	// per start would make every player a stranger after a deployment —
	// which is precisely what the cookie exists to prevent.
	//
	// Load does not insist on it, because only serve signs anything.
	// Requiring it everywhere would mean handing the cookie secret to a
	// migration job that never uses it. ValidateForServe is where it counts.
	SessionKey []byte
	// CookieSecure marks the session cookie HTTPS-only. Defaults to true, so
	// a forgotten setting fails closed: the cookie is simply not sent over
	// plain HTTP, rather than travelling over it in the clear. Local
	// development over http://localhost sets it to false.
	CookieSecure bool
	// DatabaseConnectTimeout is how long startup waits for the database
	// before giving up. Compose can wait for a healthy Postgres through
	// depends_on; Kubernetes and Azure Container Apps cannot — there the
	// application would crash-loop merely because the database is ready a few
	// seconds later.
	DatabaseConnectTimeout time.Duration
	// KioskToken unlocks the kiosk: one machine at the table where somebody
	// creates players and enters results for everybody. Empty by default,
	// and then the kiosk does not exist at all rather than existing
	// unlocked.
	//
	// It is a lock against stumbling into the page, not a security boundary.
	// What it protects is the ability to enter a wrong result, which is what
	// the application already lets anybody do and which docs/adr/0004
	// answers socially rather than technically.
	KioskToken string
	// BootstrapAdmin names the player who gets the admin flag at startup, by
	// display name. Empty by default, and then nothing is granted.
	//
	// docs/adr/0008 makes this the way the first admin comes into being:
	// issue #73 names "a way to grant it that is not psql" as the price of a
	// flag on the player, and an environment variable is that way and the
	// only form that fits invariant 2 — no config file in the image, no
	// reaching into the database by hand.
	//
	// It is read on every start rather than only the first. That is
	// deliberate: somebody who withdraws the flag from the last admin gets
	// back in by restarting with the variable set, not by opening psql.
	BootstrapAdmin string
	// PublicBaseURL is the address a phone has to reach, scheme and host and
	// nothing else. Empty by default: the QR sheet then reads the address off
	// the request, which is right wherever the application is reached
	// directly. Set it where a proxy terminates TLS or rewrites the host,
	// because there the incoming request no longer says where the code should
	// point.
	PublicBaseURL string
}

// Load reads the configuration from the environment and validates it.
func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:               env("HTTP_ADDR", DefaultHTTPAddr),
		DatabaseURL:            env("DATABASE_URL", ""),
		AutoMigrate:            true,
		CookieSecure:           true,
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

	for _, b := range []struct {
		key    string
		target *bool
	}{
		{"AUTO_MIGRATE", &cfg.AutoMigrate},
		{"COOKIE_SECURE", &cfg.CookieSecure},
	} {
		raw, ok := lookup(b.key)
		if !ok {
			continue
		}
		v, err := strconv.ParseBool(raw)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s%s=%q is not a boolean: %w", envPrefix, b.key, raw, err))
			continue
		}
		*b.target = v
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

	cfg.SessionKey = []byte(env("SESSION_KEY", ""))

	cfg.KioskToken = env("KIOSK_TOKEN", "")

	cfg.BootstrapAdmin = env("BOOTSTRAP_ADMIN", "")

	if raw := env("PUBLIC_BASE_URL", ""); raw != "" {
		base, err := parseBaseURL(raw)
		if err != nil {
			errs = append(errs, err)
		}
		cfg.PublicBaseURL = base
	}

	if err := errors.Join(errs...); err != nil {
		return Config{}, fmt.Errorf("read configuration from the environment: %w", err)
	}
	return cfg, nil
}

// ValidateForServe reports whether the configuration is complete enough to
// serve HTTP. Everything it checks is needed only by the server, so the other
// commands are spared it.
func (c Config) ValidateForServe() error {
	switch {
	case len(c.SessionKey) == 0:
		return fmt.Errorf(
			"%sSESSION_KEY is required to serve; generate one with: openssl rand -base64 32",
			envPrefix)
	case len(c.SessionKey) < minSessionKeyLen:
		return fmt.Errorf("%sSESSION_KEY is %d characters, at least %d are required",
			envPrefix, len(c.SessionKey), minSessionKeyLen)
	}
	return nil
}

// parseBaseURL accepts an absolute http or https address without a path.
//
// A path prefix is rejected rather than half-supported: every link in this
// application is root-absolute, so a code pointing at /schmetterpause/ would
// scan fine and send the first click after it to the wrong place.
func parseBaseURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("%sPUBLIC_BASE_URL=%q is not a URL: %w", envPrefix, raw, err)
	}

	switch {
	case u.Scheme != "http" && u.Scheme != "https":
		return "", fmt.Errorf("%sPUBLIC_BASE_URL=%q needs an http:// or https:// scheme", envPrefix, raw)
	case u.Host == "":
		return "", fmt.Errorf("%sPUBLIC_BASE_URL=%q has no host", envPrefix, raw)
	case strings.Trim(u.Path, "/") != "", u.RawQuery != "", u.Fragment != "":
		return "", fmt.Errorf("%sPUBLIC_BASE_URL=%q must be scheme and host only, without a path",
			envPrefix, raw)
	}
	return u.Scheme + "://" + u.Host, nil
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
