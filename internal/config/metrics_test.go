package config_test

import (
	"strings"
	"testing"

	"github.com/stuttgart-things/schmetterpause/internal/config"
)

func TestMetricsAreOffByDefault(t *testing.T) {
	t.Setenv("SP_DATABASE_URL", "postgres://user:pw@db:5432/schmetterpause")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if cfg.MetricsAddr != "" {
		t.Errorf("MetricsAddr = %q, want empty", cfg.MetricsAddr)
	}
}

func TestMetricsAddr(t *testing.T) {
	tests := []struct {
		name, httpAddr, metricsAddr string
		wantErr                     string
	}{
		{name: "own port", metricsAddr: ":9090"},
		{name: "own port on loopback", metricsAddr: "127.0.0.1:9090"},
		{name: "same port as the app", metricsAddr: ":8080", wantErr: "port of its own"},
		{name: "same port on another host", metricsAddr: "0.0.0.0:8080", wantErr: "port of its own"},
		{name: "same port as a moved app", httpAddr: ":9000", metricsAddr: ":9000", wantErr: "port of its own"},
		{name: "a bare port", metricsAddr: "9090", wantErr: "not host:port"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("SP_DATABASE_URL", "postgres://user:pw@db:5432/schmetterpause")
			t.Setenv("SP_HTTP_ADDR", tc.httpAddr)
			t.Setenv("SP_METRICS_ADDR", tc.metricsAddr)

			cfg, err := config.Load()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("Load(): %v", err)
				}
				if cfg.MetricsAddr != tc.metricsAddr {
					t.Errorf("MetricsAddr = %q, want %q", cfg.MetricsAddr, tc.metricsAddr)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("Load() error = %v, want one mentioning %q", err, tc.wantErr)
			}
		})
	}
}
