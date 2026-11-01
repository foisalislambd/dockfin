package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestSplitFlags(t *testing.T) {
	pos, flags := splitFlags([]string{"proj", "env", "--reason", "holiday freeze", "--force"})
	if pos != "proj env" || flags["reason"] != "holiday freeze" || flags["force"] != "true" {
		t.Fatalf("%q %#v", pos, flags)
	}
	pos, flags = splitFlags([]string{"app", "--override-freeze", "--commit=abc"})
	if pos != "app" || flags["override-freeze"] != "true" || flags["commit"] != "abc" {
		t.Fatalf("%q %#v", pos, flags)
	}
}

func TestFreezeCommand(t *testing.T) {
	var got methodCall
	code := run([]string{"freeze", "proj", "env", "--reason", "shipping window"}, func(string) string { return "" }, ioDiscard(), ioDiscard(), func(base, token, team, method, path string, body any) (int, string, error) {
		got = methodCall{base: base, method: method, path: path, body: body}
		return 200, `{"deploy_frozen":true}`, nil
	})
	if code != 0 {
		t.Fatal(code)
	}
	if got.method != "PUT" || got.path != "/api/v1/projects/proj/environments/env/freeze" {
		t.Fatal(got)
	}
	m := got.body.(map[string]any)
	if m["frozen"] != true || m["reason"] != "shipping window" {
		t.Fatal(m)
	}
}

func TestUnfreezeAndDeployOverride(t *testing.T) {
	code := run([]string{"unfreeze", "p", "e"}, func(string) string { return "" }, ioDiscard(), ioDiscard(), func(base, token, team, method, path string, body any) (int, string, error) {
		m := body.(map[string]any)
		if m["frozen"] != false {
			t.Fatal(m)
		}
		return 200, "ok", nil
	})
	if code != 0 {
		t.Fatal(code)
	}
	var body map[string]any
	code = run([]string{"deploy", "app-1", "--force", "--override-freeze"}, func(k string) string {
		if k == "DOCKFIN_TOKEN" {
			return "tok"
		}
		if k == "DOCKFIN_TEAM" {
			return "team-1"
		}
		return ""
	}, ioDiscard(), ioDiscard(), func(base, token, team, method, path string, b any) (int, string, error) {
		if token != "tok" || team != "team-1" {
			t.Fatalf("token %s team %s", token, team)
		}
		body = b.(map[string]any)
		if path != "/api/v1/applications/app-1/deploy" {
			t.Fatal(path)
		}
		return 423, `{"error":"frozen"}`, nil
	})
	if code != 1 {
		t.Fatal(code)
	}
	if body["force_rebuild"] != true || body["override_freeze"] != true {
		t.Fatal(body)
	}
}

func TestFormatDeployment(t *testing.T) {
	out := formatDeployment(`{"status":"finished","current_stage":"health","logs":[{"stage":"build","line":"done"}]}`)
	if !strings.Contains(out, "status: finished") || !strings.Contains(out, "[build] done") {
		t.Fatal(out)
	}
	if formatDeployment("not-json") != "not-json" {
		t.Fatal("passthrough")
	}
}

func TestResourceCRUDAndJSON(t *testing.T) {
	var method, path string
	var body any
	code := run([]string{"--json", "projects", "create", "--name", "web"}, func(string) string { return "" }, ioDiscard(), ioDiscard(), func(base, token, team, m, p string, b any) (int, string, error) {
		method, path, body = m, p, b
		return 201, `{"project":{"id":"p1","name":"web"}}`, nil
	})
	if code != 0 || method != "POST" || path != "/api/v1/projects" {
		t.Fatalf("%d %s %s", code, method, path)
	}
	m := body.(map[string]any)
	if m["name"] != "web" {
		t.Fatal(m)
	}
	code = run([]string{"apps", "delete", "app-1", "--confirm", "web", "--password", "secret"}, func(string) string { return "" }, ioDiscard(), ioDiscard(), func(base, token, team, m, p string, b any) (int, string, error) {
		path, body = p, b
		return 200, `{"status":"ok"}`, nil
	})
	if code != 0 || path != "/api/v1/applications/app-1" {
		t.Fatal(path)
	}
	del := body.(map[string]any)
	if del["confirmation_name"] != "web" || del["password"] != "secret" {
		t.Fatal(del)
	}
	code = run([]string{"env", "set", "--resource-type", "application", "--resource-id", "app-1", "--key", "PORT", "--value", "8080"}, func(string) string { return "" }, ioDiscard(), ioDiscard(), func(base, token, team, m, p string, b any) (int, string, error) {
		body = b
		if p != "/api/v1/env-vars" {
			t.Fatal(p)
		}
		return 200, `{"key":"PORT"}`, nil
	})
	if code != 0 {
		t.Fatal(code)
	}
	set := body.(map[string]any)
	if set["key"] != "PORT" || set["value"] != "8080" {
		t.Fatal(set)
	}
}

func TestHumanizeAndSSE(t *testing.T) {
	out := humanizeJSON(`{"servers":[{"id":"abc","name":"vps"}]}`)
	if !strings.Contains(out, "vps") || !strings.Contains(out, "abc") {
		t.Fatal(out)
	}
	if sseLine(`{"stage":"build","line":"done"}`) != "[build] done" {
		t.Fatal(sseLine(`{"stage":"build","line":"done"}`))
	}
	if !terminalDeployStatus("failed") || terminalDeployStatus("in_progress") {
		t.Fatal("status")
	}
}

func TestOverviewAndOpenAPI(t *testing.T) {
	var path string
	code := run([]string{"overview"}, func(string) string { return "" }, ioDiscard(), ioDiscard(), func(base, token, team, method, p string, body any) (int, string, error) {
		path = p
		return 200, `{"counts":{}}`, nil
	})
	if code != 0 || path != "/api/v1/overview" {
		t.Fatalf("%d %s", code, path)
	}
	var buf bytes.Buffer
	code = run([]string{"openapi"}, func(string) string { return "" }, &buf, ioDiscard(), nil)
	if code != 0 || !strings.Contains(buf.String(), "http://localhost:8000/openapi.json") {
		t.Fatal(buf.String())
	}
}

type methodCall struct {
	base, method, path string
	body               any
}

type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }

func ioDiscard() discard { return discard{} }
