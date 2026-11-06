package serverhealth

import "testing"

func TestParseDiskPercent(t *testing.T) {
	raw := "Filesystem 1024-blocks Used Available Capacity Mounted on\n/dev/vda1 100 81 19 81% /\n"
	n, ok := ParseDiskPercent(raw)
	if !ok || n != 81 {
		t.Fatalf("%d %v", n, ok)
	}
	n, ok = ParseDiskPercent("7")
	if !ok || n != 7 {
		t.Fatalf("%d %v", n, ok)
	}
	if _, ok := ParseDiskPercent("nope"); ok {
		t.Fatal("expected miss")
	}
}

func TestTraefikOutdated(t *testing.T) {
	if !TraefikOutdated("traefik:v3.5", "traefik:v3.6") {
		t.Fatal("3.5 should be outdated")
	}
	if TraefikOutdated("traefik:v3.6.2", "traefik:v3.6") {
		t.Fatal("patch should not be outdated")
	}
	if TraefikOutdated("traefik:latest", "traefik:v3.6") {
		t.Fatal("latest is unknown")
	}
	if TraefikOutdated("ghcr.io/library/traefik:v3.6", "traefik:v3.6") {
		t.Fatal("same version")
	}
}

func TestCountPackageLines(t *testing.T) {
	if CountPackageLines("No pending updates.") != 0 {
		t.Fatal("none")
	}
	if CountPackageLines("openssl/stable 3.0 amd64\ncurl/stable 8.0 amd64\n") != 2 {
		t.Fatal("count")
	}
}
