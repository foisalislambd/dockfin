package scheduler

import (
	"strings"
	"testing"
)

func TestUpdateImageRef(t *testing.T) {
	cases := []struct {
		registry string
		channel  string
		want     string
	}{
		{"", "stable", "ghcr.io/foisalislambd/dockfin:latest"},
		{"ghcr.io", "next", "ghcr.io/foisalislambd/dockfin:next"},
		{"ghcr.io", "nightly", "ghcr.io/foisalislambd/dockfin:nightly"},
		{"ghcr.io", "", "ghcr.io/foisalislambd/dockfin:latest"},
		{"docker.io", "stable", "docker.io/foisalislambd/dockfin:latest"},
	}
	for _, c := range cases {
		if got := updateImageRef(c.registry, c.channel); got != c.want {
			t.Errorf("updateImageRef(%q,%q) = %q, want %q", c.registry, c.channel, got, c.want)
		}
	}
}

func TestRewriteComposeImage(t *testing.T) {
	in := `services:
  postgres:
    image: postgres:16-alpine
  dockfin:
    image: ghcr.io/foisalislambd/dockfin:latest
`
	out, changed := rewriteComposeImage(in, "ghcr.io/foisalislambd/dockfin:next")
	if !changed {
		t.Fatal("expected change")
	}
	if want := "image: ghcr.io/foisalislambd/dockfin:next"; !strings.Contains(out, want) {
		t.Fatalf("missing %q in:\n%s", want, out)
	}
	if !strings.Contains(out, "image: postgres:16-alpine") {
		t.Fatalf("postgres image was rewritten:\n%s", out)
	}
	if _, changed := rewriteComposeImage(out, "ghcr.io/foisalislambd/dockfin:next"); changed {
		t.Fatal("expected no change on second pass")
	}
}
