package main

import (
	"strings"
	"testing"
)

func TestDeployRollbackJobsAndFreeze(t *testing.T) {
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

	ok([]string{"overview"}, "GET", "/api/v1/overview")
	ok([]string{"jobs"}, "GET", "/api/v1/jobs")
	ok([]string{"jobs", "j1"}, "GET", "/api/v1/jobs/j1")
	ok([]string{"jobs", "cancel", "j1"}, "POST", "/api/v1/jobs/j1/cancel")
	fail([]string{"jobs", "cancel"})
	fail([]string{"deploy"})
	ok([]string{"deploy", "app-1", "--force", "--override-freeze"}, "POST", "/api/v1/applications/app-1/deploy")
	got := body.(map[string]any)
	if got["force_rebuild"] != true || got["override_freeze"] != true {
		t.Fatal(got)
	}
	ok([]string{"rollback", "app-1", "--commit", "abc"}, "POST", "/api/v1/applications/app-1/rollback")
	got = body.(map[string]any)
	if got["commit_sha"] != "abc" || got["force_rebuild"] != true {
		t.Fatal(got)
	}
	fail([]string{"stop"})
	fail([]string{"stop", "--force"})
	ok([]string{"stop", "app-1"}, "POST", "/api/v1/applications/app-1/stop")
	ok([]string{"service-deploy", "svc-1", "--override-freeze"}, "POST", "/api/v1/services/svc-1/deploy?override_freeze=1")
	fail([]string{"service-deploy"})
	fail([]string{"freeze"})
	if code := run([]string{"openapi"}, envToken, ioDiscard(), ioDiscard(), call); code != 0 {
		t.Fatal(code)
	}
}
