package proxy

import (
	"strings"
	"testing"
)

func TestBuildMaintenanceConfig(t *testing.T) {
	cfg, err := BuildMaintenanceConfig([]MaintenanceHost{
		{ID: "app1", Host: "app.example.com", Message: "Back at 6", HTTPS: true},
		{ID: "bad", Host: "not a host", Message: "nope"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cfg.TraefikYAML, "Host(`app.example.com`)") {
		t.Fatalf("yaml:\n%s", cfg.TraefikYAML)
	}
	if !strings.Contains(cfg.TraefikYAML, "priority: 100000") {
		t.Fatal("priority")
	}
	if strings.Contains(cfg.TraefikYAML, "not a host") {
		t.Fatal("rejected host leaked")
	}
	if !strings.Contains(cfg.NginxConf, "server_name app.example.com") || !strings.Contains(cfg.HTML, "Back at 6") {
		t.Fatal(cfg.HTML)
	}
	empty, err := BuildMaintenanceConfig(nil)
	if err != nil || empty.TraefikYAML != "" {
		t.Fatalf("%+v %v", empty, err)
	}
}

func TestBuildCatchallYAML(t *testing.T) {
	off, err := BuildCatchallYAML(CatchallConfig{})
	if err != nil || off != "" {
		t.Fatal(off, err)
	}
	raw, err := BuildCatchallYAML(CatchallConfig{Enabled: true})
	if err != nil || !strings.Contains(raw, "dockfin-catchall") || !strings.Contains(raw, "priority: -1000") {
		t.Fatalf("%v\n%s", err, raw)
	}
	redir, err := BuildCatchallYAML(CatchallConfig{Enabled: true, URL: "https://status.example.com", Permanent: true})
	if err != nil || !strings.Contains(redir, "status.example.com") || !strings.Contains(redir, "permanent: true") {
		t.Fatalf("%v\n%s", err, redir)
	}
	if _, err := BuildCatchallYAML(CatchallConfig{Enabled: true, URL: "javascript:alert(1)"}); err == nil {
		t.Fatal("expected invalid url")
	}
}

func TestTraefikWWWRedirectPermanent(t *testing.T) {
	labels := TraefikWWWRedirectLabels("my app", "non-www", true)
	joined := strings.Join(labels, "\n")
	if !strings.Contains(joined, "redirectregex.permanent=true") {
		t.Fatal(joined)
	}
	temp := strings.Join(TraefikWWWRedirectLabels("my app", "www", false), "\n")
	if !strings.Contains(temp, "permanent=false") || !strings.Contains(temp, "-to-www") {
		t.Fatal(temp)
	}
}

func TestCaddyWWWRedirectPermanent(t *testing.T) {
	labels := CaddyWWWRedirectLabels("app", "example.com", "www", true)
	if len(labels) != 1 || !strings.Contains(labels[0], "301") || !strings.Contains(labels[0], "www.example.com") {
		t.Fatal(labels)
	}
	temp := CaddyWWWRedirectLabels("app", "example.com", "www", false)
	if len(temp) != 1 || strings.Contains(temp[0], "301") {
		t.Fatal(temp)
	}
}
