// Package testkit opens a throwaway Postgres database for integration tests.
package testkit

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/dockfin/dockfin/internal/crypto"
	"github.com/dockfin/dockfin/internal/db"
	"github.com/dockfin/dockfin/internal/store"
	"github.com/jackc/pgx/v5/pgxpool"
)

// OpenStore migrates a fresh database and returns a store connected to it.
// The database is dropped when the test ends. Tests skip when
// DOCKFIN_DATABASE_URL is unset or Postgres cannot be reached.
func OpenStore(t *testing.T) *store.Store {
	t.Helper()
	dsn := os.Getenv("DOCKFIN_DATABASE_URL")
	if dsn == "" {
		t.Skip("DOCKFIN_DATABASE_URL not set")
	}
	base, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	if err := admin.Ping(ctx); err != nil {
		admin.Close()
		t.Skipf("postgres unavailable: %v", err)
	}
	name := "dockfin_it_" + randomHex(t)
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		admin.Close()
		t.Fatalf("create database: %v", err)
	}
	isolated := *base
	isolated.Path = "/" + name
	if err := db.Migrate(isolated.String()); err != nil {
		_, _ = admin.Exec(context.Background(), "DROP DATABASE "+name)
		admin.Close()
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(ctx, isolated.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		cctx, ccancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer ccancel()
		_, _ = admin.Exec(cctx, `SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname=$1 AND pid <> pg_backend_pid()`, name)
		_, _ = admin.Exec(cctx, "DROP DATABASE "+name)
		admin.Close()
	})
	box, err := crypto.NewBox("test-master-key-for-integration-32b!")
	if err != nil {
		t.Fatal(err)
	}
	return store.New(pool, box)
}

func randomHex(t *testing.T) string {
	t.Helper()
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b[:])
}
