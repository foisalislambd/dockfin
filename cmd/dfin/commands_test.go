package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestJobsVulnsTransferAndHealth(t *testing.T) {
	var method, path string
	var body any
	call := func(base, token, team, m, p string, b any) (int, string, error) {
		method, path, body = m, p, b
		return 200, `{"status":"ok"}`, nil
	}
	if code := run([]string{"jobs"}, envToken, ioDiscard(), ioDiscard(), call); code != 0 || path != "/api/v1/jobs" || method != "GET" {
		t.Fatalf("jobs %d %s %s", code, method, path)
	}
	if code := run([]string{"jobs", "job-1"}, envToken, ioDiscard(), ioDiscard(), call); code != 0 || path != "/api/v1/jobs/job-1" {
		t.Fatal(path)
	}
	if code := run([]string{"jobs", "cancel", "job-1"}, envToken, ioDiscard(), ioDiscard(), call); code != 0 || method != "POST" || path != "/api/v1/jobs/job-1/cancel" {
		t.Fatalf("%s %s", method, path)
	}
	if code := run([]string{"jobs", "cancel"}, envToken, ioDiscard(), ioDiscard(), call); code != 1 {
		t.Fatal("cancel without id")
	}

	var errOut bytes.Buffer
	if code := run([]string{"vulns", "upgrade", "app-1", "--tag", "1.2.3"}, envToken, ioDiscard(), &errOut, call); code != 1 || !strings.Contains(errOut.String(), "UPGRADE") {
		t.Fatalf("upgrade without confirm: %d %s", code, errOut.String())
	}
	if code := run([]string{"vulns", "upgrade", "app-1", "--tag", "1.2.3", "--confirm", "UPGRADE"}, envToken, ioDiscard(), ioDiscard(), call); code != 0 || path != "/api/v1/applications/app-1/image-upgrade" {
		t.Fatal(path)
	}
	up := body.(map[string]any)
	if up["tag"] != "1.2.3" || up["confirm"] != "UPGRADE" {
		t.Fatal(up)
	}
	if code := run([]string{"vulns", "rebuild", "app-1", "--confirm", "REBUILD"}, envToken, ioDiscard(), ioDiscard(), call); code != 0 {
		t.Fatal(code)
	}
	rb := body.(map[string]any)
	if rb["confirm"] != "REBUILD" || rb["tag"] != nil {
		t.Fatal(rb)
	}
	if code := run([]string{"vulns", "policy", "--enabled", "true", "--gate", "critical"}, envToken, ioDiscard(), ioDiscard(), call); code != 0 || method != "PUT" || path != "/api/v1/settings/image-scanning" {
		t.Fatalf("%s %s", method, path)
	}
	pol := body.(map[string]any)
	if pol["enabled"] != true || pol["gate"] != "critical" {
		t.Fatal(pol)
	}

	errOut.Reset()
	if code := run([]string{"transfer", "srv-1", "team-2"}, envToken, ioDiscard(), &errOut, call); code != 1 || !strings.Contains(errOut.String(), "MOVE") {
		t.Fatal(errOut.String())
	}
	if code := run([]string{"transfer", "srv-1", "team-2", "--confirm", "MOVE"}, envToken, ioDiscard(), ioDiscard(), call); code != 0 || path != "/api/v1/servers/srv-1/transfer" {
		t.Fatal(path)
	}
	tr := body.(map[string]any)
	if tr["team_id"] != "team-2" || tr["confirm"] != "MOVE" {
		t.Fatal(tr)
	}

	if code := run([]string{"server-health", "srv-1", "--check"}, envToken, ioDiscard(), ioDiscard(), call); code != 0 || method != "POST" || path != "/api/v1/servers/srv-1/health/check" {
		t.Fatalf("%s %s", method, path)
	}
	if code := run([]string{"logs", "dep-1"}, envToken, ioDiscard(), ioDiscard(), call); code != 0 || path != "/api/v1/deployments/dep-1" {
		t.Fatal(path)
	}
	if code := run([]string{"rollback", "app-1", "--commit", "abc", "--override-freeze"}, envToken, ioDiscard(), ioDiscard(), call); code != 0 || path != "/api/v1/applications/app-1/rollback" {
		t.Fatal(path)
	}
	rbk := body.(map[string]any)
	if rbk["commit_sha"] != "abc" || rbk["override_freeze"] != true || rbk["force_rebuild"] != true {
		t.Fatal(rbk)
	}
}

func TestHealthExitAndVersion(t *testing.T) {
	var buf bytes.Buffer
	var seen string
	code := run([]string{"health"}, func(string) string { return "" }, &buf, ioDiscard(), func(base, token, team, method, path string, body any) (int, string, error) {
		seen = method + " " + base + path
		return 200, `{"status":"ok"}`, nil
	})
	if code != 0 || !strings.Contains(buf.String(), "200") || !strings.Contains(seen, "health") {
		t.Fatalf("%d %s seen %s", code, buf.String(), seen)
	}
	buf.Reset()
	code = run([]string{"version"}, func(string) string { return "" }, &buf, ioDiscard(), nil)
	if code != 0 || !strings.Contains(buf.String(), "dfin") {
		t.Fatal(buf.String())
	}
	buf.Reset()
	var errOut bytes.Buffer
	code = run([]string{"nope"}, func(string) string { return "" }, &buf, &errOut, nil)
	if code != 1 || !strings.Contains(buf.String()+errOut.String(), "Usage:") {
		t.Fatalf("unknown command %d out=%q err=%q", code, buf.String(), errOut.String())
	}
}

func envToken(k string) string {
	switch k {
	case "DOCKFIN_TOKEN":
		return "tok"
	case "DOCKFIN_TEAM":
		return "team"
	default:
		return ""
	}
}
