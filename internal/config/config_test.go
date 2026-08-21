package config_test

import (
	"log/slog"
	"testing"
	"time"

	"github.com/stuttgart-things/schmetterpause/internal/config"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("SP_DATABASE_URL", "postgres://user:pw@db:5432/schmetterpause")

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
}

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Setenv("SP_DATABASE_URL", "")

	if _, err := config.Load(); err == nil {
		t.Fatal("Load() without SP_DATABASE_URL returned no error")
	}
}

func TestLoadOverrides(t *testing.T) {
	t.Setenv("SP_DATABASE_URL", "postgres://user:pw@db:5432/schmetterpause")
	t.Setenv("SP_HTTP_ADDR", ":9000")
	t.Setenv("SP_LOG_LEVEL", "debug")
	t.Setenv("SP_AUTO_MIGRATE", "false")
	t.Setenv("SP_SHUTDOWN_TIMEOUT", "42s")

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
}

func TestLoadRejectsInvalidValues(t *testing.T) {
	t.Setenv("SP_DATABASE_URL", "postgres://user:pw@db:5432/schmetterpause")
	t.Setenv("SP_LOG_LEVEL", "laut")
	t.Setenv("SP_SHUTDOWN_TIMEOUT", "gleich")

	if _, err := config.Load(); err == nil {
		t.Fatal("Load() with invalid values returned no error")
	}
}
