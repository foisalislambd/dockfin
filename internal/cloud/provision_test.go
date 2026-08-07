package cloud

import (
	"context"
	"testing"
)

func TestSameSSHKey(t *testing.T) {
	a := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIExample user@host\n"
	b := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIExample dockfin"
	if !sameSSHKey(a, b) {
		t.Fatal("expected keys with different comments to match")
	}
	if sameSSHKey(a, "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOther user@host") {
		t.Fatal("expected different key bodies not to match")
	}
}

func TestDefaults(t *testing.T) {
	for _, p := range []string{"hetzner", "digitalocean", "vultr"} {
		region, size, image := Defaults(p)
		if region == "" || size == "" || image == "" {
			t.Fatalf("%s: incomplete defaults %q %q %q", p, region, size, image)
		}
	}
	if region, _, _ := Defaults("nope"); region != "" {
		t.Fatal("expected no defaults for unknown provider")
	}
}

func TestProvisionValidation(t *testing.T) {
	if _, err := Provision(context.Background(), ProvisionRequest{Provider: "hetzner"}); err == nil {
		t.Fatal("expected error for missing token")
	}
	_, err := Provision(context.Background(), ProvisionRequest{
		Provider: "unknown", Token: "t", Name: "n", PublicKey: "ssh-ed25519 AAAA",
	})
	if err == nil {
		t.Fatal("expected error for unsupported provider")
	}
}
