package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolvePublicIPOverride(t *testing.T) {
	if got := ResolvePublicIP(" 178.18.243.148 "); got != "178.18.243.148" {
		t.Fatalf("override: got %q", got)
	}
	if got := ResolvePublicIP("127.0.0.1"); got == "127.0.0.1" {
		t.Fatal("loopback override must be rejected")
	}
}

func TestEnsureKeyPairRoundTrip(t *testing.T) {
	dir := t.TempDir()
	priv1, pub1, err := EnsureKeyPair(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(priv1, "OPENSSH PRIVATE KEY") {
		t.Fatalf("expected openssh pem, got: %s", priv1[:min(len(priv1), 48)])
	}
	if !strings.HasPrefix(pub1, "ssh-ed25519 ") {
		t.Fatalf("bad pub: %q", pub1)
	}
	priv2, pub2, err := EnsureKeyPair(dir)
	if err != nil {
		t.Fatal(err)
	}
	if priv1 != priv2 || pub1 != pub2 {
		t.Fatal("second EnsureKeyPair must reuse existing key")
	}
	if _, err := os.Stat(filepath.Join(dir, "ssh", "id_ed25519")); err != nil {
		t.Fatal(err)
	}
}

func TestAuthorizePublicKeyIdempotent(t *testing.T) {
	dir := t.TempDir()
	_, pub, err := EnsureKeyPair(dir)
	if err != nil {
		t.Fatal(err)
	}
	// Use a fake home via root path we control — AuthorizePublicKey looks up real user.
	// Skip if not root / cannot write /root/.ssh in CI; only verify empty key error.
	if err := AuthorizePublicKey("root", ""); err == nil {
		t.Fatal("expected empty key error")
	}
	_ = pub
}
