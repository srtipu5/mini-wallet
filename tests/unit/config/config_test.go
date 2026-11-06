package config_test

import (
	"testing"

	"github.com/srtipu5/mini-wallet/internal/config"
)

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("PORT", "")

	cfg := config.Load()
	if cfg.DatabaseURL != "postgres://wallet:wallet@localhost:5432/wallet?sslmode=disable" || cfg.Port != "8080" {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	if cfg.ReadHeaderTimeout <= 0 || cfg.ReadTimeout <= 0 || cfg.WriteTimeout <= 0 ||
		cfg.IdleTimeout <= 0 || cfg.ShutdownTimeout <= 0 {
		t.Fatalf("all timeouts must be set: %+v", cfg)
	}
}

func TestLoad_FromEnvironment(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://u:p@db:5432/x")
	t.Setenv("PORT", "9090")

	cfg := config.Load()
	if cfg.DatabaseURL != "postgres://u:p@db:5432/x" || cfg.Port != "9090" {
		t.Fatalf("env not applied: %+v", cfg)
	}
}
