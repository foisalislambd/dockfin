package proxy

import "strings"
import "testing"

func TestBuildCaddyfileMaintenanceAndCatchall(t *testing.T) {
	raw, err := BuildCaddyfile(
		[]MaintenanceHost{{ID: "a", Host: "app.example.com", HTTPS: true}},
		CatchallConfig{Enabled: true, URL: "https://example.com", Permanent: true},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(raw, "app.example.com {") || !strings.Contains(raw, "redir https://example.com 301") {
		t.Fatalf("%s", raw)
	}
	if _, err := BuildCaddyfile(nil, CatchallConfig{Enabled: true, URL: "javascript:alert(1)"}); err == nil {
		t.Fatal("expected invalid catch-all URL")
	}
}
