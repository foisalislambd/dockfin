package sshx

import "testing"

func TestValidRemotePath(t *testing.T) {
	ok := []string{"/data/dockfin/services/x/docker-compose.yml", "/tmp/a"}
	bad := []string{"", "relative", "/data/../etc/passwd", "/tmp/\nfoo", "/tmp/foo\x00"}
	for _, p := range ok {
		if !ValidRemotePath(p) {
			t.Fatalf("expected valid: %q", p)
		}
	}
	for _, p := range bad {
		if ValidRemotePath(p) {
			t.Fatalf("expected invalid: %q", p)
		}
	}
}
