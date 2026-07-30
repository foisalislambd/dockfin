package envvars_test

import (
	"testing"

	"github.com/goolify/goolify/internal/envvars"
)

func TestResolve(t *testing.T) {
	out := envvars.Resolve("postgres://{{team.DB_USER}}:{{team.DB_PASS}}@db/app", map[string]map[string]string{
		"team": {"DB_USER": "goolify", "DB_PASS": "secret"},
	})
	want := "postgres://goolify:secret@db/app"
	if out != want {
		t.Fatalf("got %q want %q", out, want)
	}
}
