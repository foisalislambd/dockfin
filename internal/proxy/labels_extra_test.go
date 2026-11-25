package proxy

import (
	"strings"
	"testing"
)

func TestTraefikLabelBuilders(t *testing.T) {
	if TraefikLabels("web", "", "80") != nil || TraefikLabels("web", "   ", "80") != nil {
		t.Fatal("empty domain")
	}
	labels := TraefikLabels("Web; rm", "https://App.Example.com/path, app.example.com", "8080")
	joined := strings.Join(labels, "\n")
	if !strings.Contains(joined, "traefik.http.routers.web--rm.rule=") || !strings.Contains(joined, "Host(`app.example.com`)") {
		t.Fatalf("%s", joined)
	}
	if strings.Count(joined, "Host(`app.example.com`)") != 2 {
		t.Fatalf("expected https router and http router:\n%s", joined)
	}
	if !strings.Contains(joined, "loadbalancer.server.port=8080") || !strings.Contains(joined, "certresolver=letsencrypt") {
		t.Fatalf("%s", joined)
	}
	if strings.Contains(joined, "redirectscheme") {
		t.Fatal("TraefikLabels must not force the HTTP bounce")
	}
	multi := strings.Join(TraefikLabelsHTTPS("api", "a.example.com, b.example.com", "3000", true), "\n")
	if !strings.Contains(multi, "Host(`a.example.com`) || Host(`b.example.com`)") || !strings.Contains(multi, "permanent=true") {
		t.Fatalf("%s", multi)
	}
}

func TestTraefikBasicAuthAndCustomLabels(t *testing.T) {
	if TraefikBasicAuthLabels("", "user:hash") != nil || TraefikBasicAuthLabels("web", "") != nil {
		t.Fatal("empty auth labels")
	}
	auth := TraefikBasicAuthLabels("My App", "user:$2y$10$hash")
	joined := strings.Join(auth, "\n")
	if !strings.Contains(joined, "my-app-basicauth.basicauth.users=user:$2y$10$hash") {
		t.Fatalf("%s", joined)
	}
	if !strings.Contains(joined, "traefik.http.routers.my-app.middlewares=my-app-basicauth") {
		t.Fatalf("%s", joined)
	}
	if TraefikWWWRedirectLabels("", "www", true) != nil || TraefikWWWRedirectLabels("app", "sideways", false) != nil {
		t.Fatal("bad redirect mode")
	}
	parsed := ParseCustomLabels(" # comment\ntraefik.enable=true, traefik.http.middlewares.x.compress=true\n\nnoequals\n  a=b  ")
	if strings.Join(parsed, "|") != "traefik.enable=true|traefik.http.middlewares.x.compress=true|a=b" {
		t.Fatalf("%#v", parsed)
	}
	if ParseCustomLabels("") != nil || ParseCustomLabels("# only") != nil {
		t.Fatal("expected no labels")
	}
}

func TestCaddyLabelAbuse(t *testing.T) {
	if CaddyLabels("app", "example.com", "", false) != nil {
		t.Fatal("empty port")
	}
	if CaddyLabels("app", strings.Repeat("a", 254), "80", false) != nil {
		t.Fatal("overlong host")
	}
	if CaddyLabels("app", "example.com;id", "80", false) != nil || CaddyLabels("app", "example.com", "100000", true) != nil {
		t.Fatal("metacharacter or long port")
	}
	wild := CaddyLabels("app", "*.example.com", "00080", false)
	if len(wild) != 2 || wild[0] != "caddy=http://*.example.com" || !strings.Contains(wild[1], "00080") {
		t.Fatalf("%#v", wild)
	}
	https := CaddyLabels("app", "https://example.com", "443", true)
	if https[0] != "caddy=example.com" {
		t.Fatalf("%#v", https)
	}
	kept := CaddyLabels("app", "http://example.com", "80", false)
	if kept[0] != "caddy=http://example.com" {
		t.Fatalf("%#v", kept)
	}
	if CaddyWWWRedirectLabels("app", "www.example.com", "www", true) != nil {
		t.Fatal("www mode on a www host")
	}
	if CaddyWWWRedirectLabels("app", "example.com", "non-www", true) != nil {
		t.Fatal("non-www mode without www")
	}
	if CaddyWWWRedirectLabels("app", "bad host", "www", false) != nil || CaddyWWWRedirectLabels("app", "example.com", "nope", true) != nil {
		t.Fatal("bad redirect")
	}
	redir := CaddyWWWRedirectLabels("app", "https://www.example.com/ignored", "non-www", true)
	if len(redir) != 1 || redir[0] != "caddy.redir=https://example.com{uri} 301" {
		t.Fatalf("%#v", redir)
	}
	magic := CaddyWWWRedirectLabels("app", "app.1.2.3.4.sslip.io", "www", true)
	if len(magic) != 1 || !strings.HasPrefix(magic[0], "caddy.redir=http://www.") {
		t.Fatalf("%#v", magic)
	}
}

func TestStartProxyAndSwarmInputs(t *testing.T) {
	if err := StartProxy(nil, "none", "", "", "", ""); err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("%v", err)
	}
	if err := StartProxy(nil, "nginx", "", "", "", ""); err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("%v", err)
	}
	if SwarmManager(nil) {
		t.Fatal("nil client is not a swarm manager")
	}
	if _, err := TraefikStackCompose("traefik:v3.6", "dockfin", "ops@example.com\n"); err == nil {
		t.Fatal("newline email")
	}
	if _, err := TraefikStackCompose("traefik:v3.6", "bad network", "ops@example.com"); err == nil {
		t.Fatal("bad network")
	}
	if _, err := CaddyStackCompose("bad;image", "dockfin"); err == nil {
		t.Fatal("bad image")
	}
	raw, err := TraefikStackCompose("traefik:v3.6", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(raw, "image: traefik:v3.6") || !strings.Contains(raw, "admin@sslip.io") || !strings.Contains(raw, "network=dockfin") {
		t.Fatalf("%s", raw)
	}
}

func TestComposeLabelsAndCaddyfileEdges(t *testing.T) {
	labels := TraefikComposeLabels("My App", "app.example.com", "8080", "dockfin")
	if labels["traefik.enable"] != "true" || labels["traefik.docker.network"] != "dockfin" {
		t.Fatalf("%#v", labels)
	}
	if labels["traefik.http.routers.my-app.entrypoints"] != "http" || labels["traefik.http.services.my-app.loadbalancer.server.port"] != "8080" {
		t.Fatalf("%#v", labels)
	}
	if !strings.Contains(labels["traefik.http.routers.my-app.rule"], "Host(`app.example.com`)") {
		t.Fatalf("%#v", labels)
	}
	raw, err := BuildCaddyfile([]MaintenanceHost{
		{Host: "App.Example.com", HTTPS: true},
		{Host: "app.example.com", HTTPS: false},
		{Host: "not a host"},
		{Host: ""},
	}, CatchallConfig{Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(raw, "app.example.com") != 1 || strings.Contains(raw, "not a host") {
		t.Fatalf("%s", raw)
	}
	if !strings.Contains(raw, "copy_response 503") || !strings.Contains(raw, ":80, :443") {
		t.Fatalf("%s", raw)
	}
	off, err := BuildCaddyfile(nil, CatchallConfig{})
	if err != nil || strings.Contains(off, ":80") {
		t.Fatalf("%v\n%s", err, off)
	}
}
