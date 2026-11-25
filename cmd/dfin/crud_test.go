package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResourceAndParityCommands(t *testing.T) {
	var method, path string
	var body any
	call := func(base, token, team, m, p string, b any) (int, string, error) {
		method, path, body = m, p, b
		return 200, `{"status":"ok"}`, nil
	}
	ok := func(args []string, wantMethod, wantPath string) {
		t.Helper()
		method, path, body = "", "", nil
		if code := run(args, envToken, ioDiscard(), ioDiscard(), call); code != 0 {
			t.Fatalf("%v -> %d", args, code)
		}
		if method != wantMethod || !strings.Contains(path, wantPath) {
			t.Fatalf("%v -> %s %s", args, method, path)
		}
	}
	fail := func(args []string) {
		t.Helper()
		if code := run(args, envToken, ioDiscard(), ioDiscard(), call); code == 0 {
			t.Fatalf("%v succeeded", args)
		}
	}

	ok([]string{"servers"}, "GET", "/api/v1/servers")
	ok([]string{"servers", "get", "s1"}, "GET", "/api/v1/servers/s1")
	fail([]string{"servers", "get"})
	fail([]string{"servers", "create", "--name", "a"})
	ok([]string{"servers", "create", "--name", "a", "--ip", "10.0.0.2", "--port", "2222", "--user", "root", "--proxy", "traefik"}, "POST", "/api/v1/servers")
	got := body.(map[string]any)
	if got["user_name"] != "root" || got["proxy_type"] != "traefik" || got["port"] != 2222 {
		t.Fatal(got)
	}
	ok([]string{"servers", "delete", "s1"}, "DELETE", "/api/v1/servers/s1")
	fail([]string{"servers", "update"})

	ok([]string{"projects"}, "GET", "/api/v1/projects")
	ok([]string{"projects", "get", "p1"}, "GET", "/api/v1/projects/p1")
	fail([]string{"projects", "create"})
	ok([]string{"projects", "create", "--name", "web", "--description", "d"}, "POST", "/api/v1/projects")
	ok([]string{"projects", "update", "p1", "--name", "web2"}, "PATCH", "/api/v1/projects/p1")
	fail([]string{"projects", "update", "p1"})
	ok([]string{"projects", "delete", "p1", "--confirm", "web2", "--password", "pw"}, "DELETE", "/api/v1/projects/p1")
	del := body.(map[string]any)
	if del["confirmation_name"] != "web2" || del["password"] != "pw" {
		t.Fatal(del)
	}

	fail([]string{"environments"})
	ok([]string{"environments", "list", "p1"}, "GET", "/projects/p1/environments")
	ok([]string{"environments", "get", "p1", "e1"}, "GET", "/environments/e1")
	fail([]string{"environments", "create", "p1"})
	ok([]string{"environments", "create", "p1", "--name", "prod"}, "POST", "/projects/p1/environments")
	ok([]string{"environments", "update", "p1", "e1", "--name", "prod"}, "PATCH", "/environments/e1")
	ok([]string{"environments", "delete", "p1", "e1", "--confirm", "prod"}, "DELETE", "/environments/e1")

	ok([]string{"applications", "list", "--environment", "e1"}, "GET", "environment_id=e1")
	ok([]string{"apps", "get", "a1"}, "GET", "/api/v1/applications/a1")
	fail([]string{"apps", "create", "--name", "web"})
	ok([]string{"apps", "create", "--environment", "e1", "--name", "web", "--git", "https://github.com/acme/web.git", "--branch", "main", "--build-pack", "dockerfile", "--fqdn", "web.example.com"}, "POST", "/api/v1/applications")
	app := body.(map[string]any)
	if app["environment_id"] != "e1" || app["git_repository"] != "https://github.com/acme/web.git" || app["build_pack"] != "dockerfile" {
		t.Fatal(app)
	}
	ok([]string{"apps", "delete", "a1", "--confirm", "web", "--password", "pw"}, "DELETE", "/applications/a1")

	ok([]string{"services"}, "GET", "/api/v1/services")
	ok([]string{"services", "get", "sv1"}, "GET", "/services/sv1")
	fail([]string{"services", "create", "--name", "redis"})
	ok([]string{"services", "create", "--environment", "e1", "--name", "redis", "--template", "redis", "--server", "s1", "--fqdn", "redis.example.com"}, "POST", "/api/v1/services")
	ok([]string{"services", "delete", "sv1", "--confirm", "redis"}, "DELETE", "/services/sv1")

	ok([]string{"databases", "list", "--environment", "e1"}, "GET", "environment_id=e1")
	ok([]string{"databases", "get", "db1"}, "GET", "/databases/db1")
	fail([]string{"databases", "create", "--environment", "e1", "--name", "pg"})
	ok([]string{"databases", "create", "--environment", "e1", "--name", "pg", "--engine", "postgresql"}, "POST", "/api/v1/databases")
	ok([]string{"databases", "delete", "db1", "--confirm", "pg", "--password", "pw"}, "DELETE", "/databases/db1")

	fail([]string{"env", "list"})
	ok([]string{"env", "list", "--resource-type", "application", "--resource-id", "a1"}, "GET", "resource_id=a1")
	fail([]string{"env", "set", "--key", "PORT"})
	ok([]string{"env", "set", "--resource-type", "application", "--resource-id", "a1", "--key", "PORT", "--value", "80"}, "POST", "/api/v1/env-vars")
	envBody := body.(map[string]any)
	if envBody["key"] != "PORT" || envBody["value"] != "80" {
		t.Fatal(envBody)
	}
	ok([]string{"env", "unset", "var1"}, "DELETE", "/env-vars/var1")
	fail([]string{"env", "unset"})

	fail([]string{"domains"})
	ok([]string{"domains", "list", "sv1"}, "GET", "/services/sv1/domains")
	fail([]string{"domains", "create", "sv1"})
	ok([]string{"domains", "create", "sv1", "--host", "app.example.com"}, "POST", "/services/sv1/domains")
	if body.(map[string]any)["host"] != "app.example.com" {
		t.Fatal(body)
	}
	ok([]string{"domains", "delete", "sv1", "d1"}, "DELETE", "/domains/d1")

	ok([]string{"tokens"}, "GET", "/api/v1/api-tokens")
	fail([]string{"tokens", "pause"})
	ok([]string{"tokens", "pause", "t1"}, "PATCH", "/api-tokens/t1")
	if body.(map[string]any)["paused"] != true {
		t.Fatal(body)
	}
	ok([]string{"tokens", "resume", "t1"}, "PATCH", "/api-tokens/t1")
	if body.(map[string]any)["paused"] != false {
		t.Fatal(body)
	}
	ok([]string{"tokens", "roll", "t1"}, "POST", "/api-tokens/t1/roll")
	fail([]string{"tokens", "nope"})

	fail([]string{"maintenance", "get", "widgets", "a1"})
	ok([]string{"maintenance", "get", "applications", "a1"}, "GET", "/applications/a1/maintenance")
	ok([]string{"maintenance", "set", "services", "sv1", "--enabled", "true", "--message", "down"}, "PUT", "/services/sv1/maintenance")
	if body.(map[string]any)["enabled"] != true || body.(map[string]any)["message"] != "down" {
		t.Fatal(body)
	}

	fail([]string{"files", "list", "widgets", "a1"})
	ok([]string{"files", "list", "applications", "a1", "--path", "/app", "--container", "web"}, "GET", "/applications/a1/files?")
	ok([]string{"files", "read", "services", "sv1"}, "GET", "/files/content?")
	ok([]string{"files", "write", "databases", "db1", "--path", "/init.sql", "--content", "select 1"}, "PUT", "/files/content")
	ok([]string{"files", "mkdir", "applications", "a1", "--path", "/data"}, "POST", "/files/mkdir")
	fail([]string{"files", "rm", "applications", "a1", "--path", "/data"})
	ok([]string{"files", "rm", "applications", "a1", "--path", "/data", "--confirm", "data"}, "DELETE", "/files/content")

	fail([]string{"flush-cache"})
	ok([]string{"flush-cache", "db1"}, "POST", "/databases/db1/flush-cache")
	ok([]string{"server-health", "s1"}, "GET", "/servers/s1/health")
	ok([]string{"fleet"}, "GET", "/api/v1/fleet/metrics")
	fail([]string{"scan"})
	ok([]string{"scan", "acme/web", "--branch", "main"}, "POST", "/api/v1/git/scan")
	if body.(map[string]any)["repository"] != "acme/web" || body.(map[string]any)["branch"] != "main" {
		t.Fatal(body)
	}
	ok([]string{"vulns"}, "GET", "/api/v1/image-scans")
	ok([]string{"vulns", "run", "a1"}, "POST", "/applications/a1/image-scans")
	ok([]string{"vulns", "candidates", "scan1"}, "POST", "/image-scans/scan1/candidates")
	ok([]string{"vulns", "policy"}, "GET", "/settings/image-scanning")
	fail([]string{"vulns", "nope"})

	fail([]string{"bundle", "export", "s1"})
	ok([]string{"bundle", "export", "s1", "--passphrase", "correct horse battery"}, "POST", "/servers/s1/bundle")
	dir := t.TempDir()
	bundlePath := filepath.Join(dir, "bundle.json")
	if err := os.WriteFile(bundlePath, []byte(`{"ok":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	fail([]string{"bundle", "import", "--passphrase", "x"})
	ok([]string{"bundle", "import", "--passphrase", "correct horse battery", "--file", bundlePath}, "POST", "/api/v1/server-bundles")
	if !strings.Contains(body.(map[string]any)["bundle"].(string), "ok") {
		t.Fatal(body)
	}
	fail([]string{"bundle", "nope"})
	fail([]string{"not-a-command"})
}
