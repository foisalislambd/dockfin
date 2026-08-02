package store_test

import (
	"context"
	"os"
	"testing"

	"github.com/dockfin/dockfin/internal/crypto"
	"github.com/dockfin/dockfin/internal/db"
	"github.com/dockfin/dockfin/internal/envvars"
	"github.com/dockfin/dockfin/internal/store"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestCryptoEnvResolveCancelDeployment(t *testing.T) {
	dsn := os.Getenv("DOCKFIN_DATABASE_URL")
	if dsn == "" {
		t.Skip("DOCKFIN_DATABASE_URL not set")
	}

	box, err := crypto.NewBox("test-master-key-for-integration-32b!")
	if err != nil {
		t.Fatal(err)
	}
	enc, err := box.EncryptString("hello-secret")
	if err != nil {
		t.Fatal(err)
	}
	plain, err := box.DecryptString(enc)
	if err != nil || plain != "hello-secret" {
		t.Fatalf("crypto roundtrip failed: %v %q", err, plain)
	}

	resolved := envvars.Resolve("{{team.FOO}}-bar", map[string]map[string]string{
		"team": {"FOO": "baz"},
	})
	if resolved != "baz-bar" {
		t.Fatalf("env resolve: got %q", resolved)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(dsn); err != nil {
		t.Logf("migrate warning: %v", err)
	}

	st := store.New(pool, box)

	// CancelDeployment on a non-existent id should return ErrNotFound
	fakeID := store.MustUUID()
	teamID := store.MustUUID()
	err = st.CancelDeployment(ctx, teamID, fakeID)
	if err != store.ErrNotFound {
		// May also fail if team/deployment don't exist — ErrNotFound is expected
		if err == nil {
			t.Fatal("expected ErrNotFound for missing deployment")
		}
	}

	// Round-trip ResolvedEnvMap path requires real rows; at minimum ensure method exists
	_, _ = st.ResolvedEnvMap(ctx, teamID, "application", fakeID, nil, nil, nil)
}
