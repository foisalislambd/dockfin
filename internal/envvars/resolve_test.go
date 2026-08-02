package envvars_test

import (
	"testing"

	"github.com/dockfin/dockfin/internal/envvars"
)

func TestResolve(t *testing.T) {
	out := envvars.Resolve("postgres://{{team.DB_USER}}:{{team.DB_PASS}}@db/app", map[string]map[string]string{
		"team": {"DB_USER": "dockfin", "DB_PASS": "secret"},
	})
	want := "postgres://dockfin:secret@db/app"
	if out != want {
		t.Fatalf("got %q want %q", out, want)
	}
}
