package config_test

import (
	"log/slog"
	"testing"
	"time"

	"github.com/stuttgart-things/schmetterpause/internal/config"
)

// testKey is long enough to satisfy the minimum; its content does not matter.
const testKey = "0123456789abcdef0123456789abcdef"

func TestLoadDefaults(t *testing.T) {
	t.Setenv("SP_DATABASE_URL", "postgres://user:pw@db:5432/schmetterpause")
	t.Setenv("SP_SESSION_KEY", testKey)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}

	if cfg.HTTPAddr != ":8080" {
		t.Errorf("HTTPAddr = %q, want \":8080\"", cfg.HTTPAddr)
	}
	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("LogLevel = %v, want info", cfg.LogLevel)
	}
	if !cfg.AutoMigrate {
		t.Error("AutoMigrate = false, want true")
	}
	if cfg.ShutdownTimeout != 15*time.Second {
		t.Errorf("ShutdownTimeout = %v, want 15s", cfg.ShutdownTimeout)
	}
	// Fails closed: a forgotten setting keeps the cookie off plain HTTP.
	if !cfg.CookieSecure {
		t.Error("CookieSecure = false, want true")
	}
	if string(cfg.SessionKey) != testKey {
		t.Errorf("SessionKey = %q, want the value from the environment", cfg.SessionKey)
	}
}

// TestLoadWithoutASessionKey covers the commands that never sign anything.
// "schmetterpause migrate" must not need the cookie secret — requiring it
// would mean handing that secret to a migration job for nothing.
func TestLoadWithoutASessionKey(t *testing.T) {
	t.Setenv("SP_DATABASE_URL", "postgres://user:pw@db:5432/schmetterpause")
	t.Setenv("SP_SESSION_KEY", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() without SP_SESSION_KEY: %v", err)
	}
	if err := cfg.ValidateForServe(); err == nil {
		t.Error("ValidateForServe() without a session key returned no error")
	}
}

func TestValidateForServeRejectsAShortKey(t *testing.T) {
	// A short key would sign cookies that are cheap to forge, so it is
	// refused rather than accepted with a warning nobody reads.
	t.Setenv("SP_DATABASE_URL", "postgres://user:pw@db:5432/schmetterpause")
	t.Setenv("SP_SESSION_KEY", "tooshort")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if err := cfg.ValidateForServe(); err == nil {
		t.Error("ValidateForServe() with a short key returned no error")
	}
}

func TestValidateForServeAcceptsAProperKey(t *testing.T) {
	t.Setenv("SP_DATABASE_URL", "postgres://user:pw@db:5432/schmetterpause")
	t.Setenv("SP_SESSION_KEY", testKey)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if err := cfg.ValidateForServe(); err != nil {
		t.Errorf("ValidateForServe(): %v", err)
	}
}

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Setenv("SP_DATABASE_URL", "")

	if _, err := config.Load(); err == nil {
		t.Fatal("Load() without SP_DATABASE_URL returned no error")
	}
}

func TestLoadOverrides(t *testing.T) {
	t.Setenv("SP_DATABASE_URL", "postgres://user:pw@db:5432/schmetterpause")
	t.Setenv("SP_SESSION_KEY", testKey)
	t.Setenv("SP_HTTP_ADDR", ":9000")
	t.Setenv("SP_LOG_LEVEL", "debug")
	t.Setenv("SP_AUTO_MIGRATE", "false")
	t.Setenv("SP_SHUTDOWN_TIMEOUT", "42s")
	t.Setenv("SP_COOKIE_SECURE", "false")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}

	if cfg.HTTPAddr != ":9000" {
		t.Errorf("HTTPAddr = %q, want \":9000\"", cfg.HTTPAddr)
	}
	if cfg.LogLevel != slog.LevelDebug {
		t.Errorf("LogLevel = %v, want debug", cfg.LogLevel)
	}
	if cfg.AutoMigrate {
		t.Error("AutoMigrate = true, want false")
	}
	if cfg.ShutdownTimeout != 42*time.Second {
		t.Errorf("ShutdownTimeout = %v, want 42s", cfg.ShutdownTimeout)
	}
	if cfg.CookieSecure {
		t.Error("CookieSecure = true, want false")
	}
}

func TestLoadRejectsInvalidValues(t *testing.T) {
	t.Setenv("SP_DATABASE_URL", "postgres://user:pw@db:5432/schmetterpause")
	t.Setenv("SP_SESSION_KEY", testKey)
	t.Setenv("SP_LOG_LEVEL", "laut")
	t.Setenv("SP_SHUTDOWN_TIMEOUT", "gleich")

	if _, err := config.Load(); err == nil {
		t.Fatal("Load() with invalid values returned no error")
	}
}

func TestPublicBaseURL(t *testing.T) {
	tests := []struct {
		name  string
		raw   string
		want  string
		valid bool
	}{
		{"unset", "", "", true},
		{"host and scheme", "https://tt.example.org", "https://tt.example.org", true},
		{"with a port", "http://192.168.1.5:8080", "http://192.168.1.5:8080", true},
		{"trailing slash", "https://tt.example.org/", "https://tt.example.org", true},
		{"no scheme", "tt.example.org", "", false},
		{"wrong scheme", "ftp://tt.example.org", "", false},
		{"no host", "https://", "", false},
		// Every link in the application is root-absolute, so a prefix would
		// scan fine and break the first click after the scan.
		{"path prefix", "https://tt.example.org/schmetterpause", "", false},
		{"query", "https://tt.example.org/?x=1", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("SP_DATABASE_URL", "postgres://user:pw@db:5432/schmetterpause")
			t.Setenv("SP_SESSION_KEY", testKey)
			t.Setenv("SP_PUBLIC_BASE_URL", tc.raw)

			cfg, err := config.Load()
			switch {
			case tc.valid && err != nil:
				t.Fatalf("Load(): %v", err)
			case !tc.valid && err == nil:
				t.Fatalf("Load() accepted %q", tc.raw)
			case !tc.valid:
				return
			}

			if cfg.PublicBaseURL != tc.want {
				t.Errorf("PublicBaseURL = %q, want %q", cfg.PublicBaseURL, tc.want)
			}
		})
	}
}
