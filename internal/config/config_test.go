package config

import (
	"testing"
	"time"
)

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	if _, err := Load(); err == nil {
		t.Fatal("expected error when DATABASE_URL is missing")
	}
}

func TestLoadReadsEnvironment(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db?sslmode=disable")
	t.Setenv("APP_ENV", "test")
	t.Setenv("HTTP_ADDR", ":9090")
	t.Setenv("HTTP_READ_TIMEOUT", "12s")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if cfg.AppEnv != "test" {
		t.Fatalf("expected APP_ENV test, got %q", cfg.AppEnv)
	}
	if cfg.HTTPAddr != ":9090" {
		t.Fatalf("expected HTTP_ADDR :9090, got %q", cfg.HTTPAddr)
	}
	if cfg.DatabaseURL == "" {
		t.Fatal("expected DATABASE_URL to be populated")
	}
	if cfg.ReadTimeout != 12*time.Second {
		t.Fatalf("expected HTTP_READ_TIMEOUT 12s, got %s", cfg.ReadTimeout)
	}
	if cfg.ShutdownTimeout != 10*time.Second {
		t.Fatalf("expected default SHUTDOWN_TIMEOUT 10s, got %s", cfg.ShutdownTimeout)
	}
}

func TestLoadRejectsInvalidDuration(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db?sslmode=disable")
	t.Setenv("HTTP_READ_TIMEOUT", "not-a-duration")

	if _, err := Load(); err == nil {
		t.Fatal("expected error for invalid HTTP_READ_TIMEOUT")
	}
}
