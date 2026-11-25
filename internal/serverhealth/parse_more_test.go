package serverhealth

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dockfin/dockfin/internal/store"
)

func TestParseDiskPercentEdges(t *testing.T) {
	if _, ok := ParseDiskPercent(""); ok {
		t.Fatal("empty")
	}
	if _, ok := ParseDiskPercent("101"); ok {
		t.Fatal("over 100")
	}
	if _, ok := ParseDiskPercent("-1"); ok {
		t.Fatal("negative")
	}
	if n, ok := ParseDiskPercent("  100% \n"); !ok || n != 100 {
		t.Fatalf("%d %v", n, ok)
	}
	raw := "Filesystem 1024-blocks Used Available Capacity Mounted on\n/dev/vda1 100 10 90 150% /\n/dev/vda1 100 40 60 40% /\n"
	if n, ok := ParseDiskPercent(raw); !ok || n != 40 {
		t.Fatalf("last valid row: %d %v", n, ok)
	}
	if _, ok := ParseDiskPercent("nope 1 2 3 nope"); ok {
		t.Fatal("non numeric capacity")
	}
	if n, ok := ParseDiskPercent("0"); !ok || n != 0 {
		t.Fatalf("%d %v", n, ok)
	}
}

func TestImageTagAndVersions(t *testing.T) {
	if ImageTag("ghcr.io/org/traefik:v3.6.1@sha256:abcdef") != "v3.6.1" {
		t.Fatal(ImageTag("ghcr.io/org/traefik:v3.6.1@sha256:abcdef"))
	}
	if ImageTag("localhost:5000/traefik:v3.6") != "v3.6" {
		t.Fatal(ImageTag("localhost:5000/traefik:v3.6"))
	}
	if ImageTag("localhost:5000/traefik") != "" || ImageTag("traefik") != "" || ImageTag("  ") != "" {
		t.Fatal("untagged")
	}
	if !TraefikOutdated("traefik:v2.10.4", "traefik:v3.0.0") {
		t.Fatal("major")
	}
	if TraefikOutdated("traefik:v3.6.0", "traefik:v3.6.0") || TraefikOutdated("traefik:v3", "traefik:v3.6") {
		t.Fatal("equal or incomplete should not be outdated")
	}
	if TraefikOutdated("traefik:v3.6.1", "not-an-image") {
		t.Fatal("missing expected version")
	}
}

func TestCountPackageLinesEdges(t *testing.T) {
	if CountPackageLines("") != 0 || CountPackageLines("   ") != 0 {
		t.Fatal("blank")
	}
	if CountPackageLines("no supported package manager found") != 0 {
		t.Fatal("sentinel")
	}
	got := CountPackageLines("Listing...\n\nopenssl 1\n  \ncurl 2\n")
	if got != 2 {
		t.Fatalf("%d", got)
	}
}

func TestProbeNilClientEdges(t *testing.T) {
	now := time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)
	next, notes := Probe(nil, store.ServerHealth{}, "edge", 0, "caddy", true, nil, now)
	if next.Reachable || next.LastError != "ssh dial failed" || next.DiskMount != "/" || len(notes) != 0 {
		t.Fatalf("%+v %v", next, notes)
	}
	if next.PatchNotifiedCount != -1 {
		t.Fatalf("first observation: %+v", next)
	}
	long := errors.New(strings.Repeat("x", 800))
	next, _ = Probe(nil, next, "edge", 90, "caddy", false, long, now)
	if len(next.LastError) != 500 {
		t.Fatalf("trimmed %d", len(next.LastError))
	}
}
