package proxy

import "strings"
import "testing"

func TestSwarmComposeIncludesMaintenanceMounts(t *testing.T) {
	raw, err := CaddyStackCompose("", "dockfin")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"/etc/caddy/Caddyfile", "node.role == manager", "dockfin-proxy", CaddyDynamicDir, "mode: host"} {
		if want == "dockfin-proxy" {
			continue
		}
		if !strings.Contains(raw, want) {
			t.Fatalf("missing %s\n%s", want, raw)
		}
	}
	if _, err := CaddyStackCompose("bad image", "dockfin"); err == nil {
		t.Fatal("expected invalid image")
	}
	traefik, err := TraefikStackCompose("", "dockfin", "ops@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(traefik, "swarmMode=true") || !strings.Contains(traefik, "/dynamic") {
		t.Fatalf("%s", traefik)
	}
}
