package proxy

import "testing"

func TestSkipProxyNetworkConnect(t *testing.T) {
	skip := []string{"", "bridge", "host", "none", "ingress", "BRIDGE", "docker_gwbridge"}
	for _, n := range skip {
		if !SkipProxyNetworkConnect(n) {
			t.Fatalf("expected skip %q", n)
		}
	}
	keep := []string{"dockfin", "dockfin-svc-abcd1234", "project_default"}
	for _, n := range keep {
		if SkipProxyNetworkConnect(n) {
			t.Fatalf("should connect %q", n)
		}
	}
	if !SkipProxyNetworkConnect("docker_gwbridge") {
		t.Fatal("must skip docker_gwbridge")
	}
}
