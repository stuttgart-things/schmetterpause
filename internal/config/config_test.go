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
		t.Errorf("HTTPAddr = %q, erwartet \":8080\"", cfg.HTTPAddr)
	}
	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("LogLevel = %v, erwartet info", cfg.LogLevel)
	}
	if !cfg.AutoMigrate {
		t.Error("AutoMigrate = false, erwartet true")
	}
	if cfg.ShutdownTimeout != 15*time.Second {
		t.Errorf("ShutdownTimeout = %v, erwartet 15s", cfg.ShutdownTimeout)
	}
}

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Setenv("SP_DATABASE_URL", "")

	if _, err := config.Load(); err == nil {
		t.Fatal("Load() ohne SP_DATABASE_URL lieferte keinen Fehler")
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
		t.Errorf("HTTPAddr = %q, erwartet \":9000\"", cfg.HTTPAddr)
	}
	if cfg.LogLevel != slog.LevelDebug {
		t.Errorf("LogLevel = %v, erwartet debug", cfg.LogLevel)
	}
	if cfg.AutoMigrate {
		t.Error("AutoMigrate = true, erwartet false")
	}
	if cfg.ShutdownTimeout != 42*time.Second {
		t.Errorf("ShutdownTimeout = %v, erwartet 42s", cfg.ShutdownTimeout)
	}
}

func TestLoadRejectsInvalidValues(t *testing.T) {
	t.Setenv("SP_DATABASE_URL", "postgres://user:pw@db:5432/schmetterpause")
	t.Setenv("SP_LOG_LEVEL", "laut")
	t.Setenv("SP_SHUTDOWN_TIMEOUT", "gleich")

	if _, err := config.Load(); err == nil {
		t.Fatal("Load() mit ungueltigen Werten lieferte keinen Fehler")
	}
}
