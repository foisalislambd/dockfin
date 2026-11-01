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
