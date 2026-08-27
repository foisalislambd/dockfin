package config_test

import (
	"testing"

	"github.com/dockfin/dockfin/internal/config"
)

func TestRejectsExampleMasterKeyInProduction(t *testing.T) {
	t.Setenv("DOCKFIN_ENV", "production")
	t.Setenv("DOCKFIN_MASTER_KEY", "change-me-to-a-32-byte-secret-key!!")
	t.Setenv("DOCKFIN_DATABASE_URL", "postgres://dockfin:dockfin@localhost:5432/dockfin?sslmode=disable")
	if _, err := config.Load(); err == nil {
		t.Fatal("expected error for example master key")
	}
}

func TestAllowsExampleMasterKeyInDevelopment(t *testing.T) {
	t.Setenv("DOCKFIN_ENV", "development")
	t.Setenv("DOCKFIN_MASTER_KEY", "change-me-to-a-32-byte-secret-key!!")
	t.Setenv("DOCKFIN_DATABASE_URL", "postgres://dockfin:dockfin@localhost:5432/dockfin?sslmode=disable")
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AllowUnsignedWebhooks {
		t.Fatal("unsigned webhooks must stay off by default")
	}
}
