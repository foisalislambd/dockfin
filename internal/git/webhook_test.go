package git

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"testing"
)

func TestNormalizeRepoFullName(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"owner/repo", "owner/repo"},
		{"Owner/Repo.git", "owner/repo"},
		{"https://github.com/owner/repo", "owner/repo"},
		{"https://github.com/owner/repo.git", "owner/repo"},
		{"git@github.com:owner/repo.git", "owner/repo"},
		{"github.com/owner/repo", "owner/repo"},
		{"group/sub/repo", "group/sub/repo"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := NormalizeRepoFullName(tc.in); got != tc.want {
			t.Errorf("NormalizeRepoFullName(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

func TestShouldSkipDeploy(t *testing.T) {
	if !ShouldSkipDeploy([]string{"fix [skip ci]", "chore [skip cd]"}) {
		t.Fatal("expected skip when all messages have markers")
	}
	if ShouldSkipDeploy([]string{"fix [skip ci]", "real change"}) {
		t.Fatal("should not skip when one message lacks marker")
	}
	if ShouldSkipDeploy([]string{}) {
		t.Fatal("empty should not skip")
	}
	if !ShouldSkipDeployAny([]string{"PR [skip ci]", "ok"}) {
		t.Fatal("ShouldSkipDeployAny should match any")
	}
	if ShouldSkipDeployAny([]string{"no marker"}) {
		t.Fatal("ShouldSkipDeployAny false")
	}
}

func TestParseGitHubPush(t *testing.T) {
	body := []byte(`{
		"ref":"refs/heads/main",
		"after":"abc123",
		"repository":{"full_name":"Acme/App"},
		"head_commit":{"message":"feat: hi","added":["a.go"],"removed":[],"modified":[]},
		"commits":[{"message":"feat: hi","added":["a.go"],"removed":[],"modified":["b.go"]}]
	}`)
	r := &http.Request{Header: http.Header{"X-Github-Event": []string{"push"}}}
	ev, err := ParseWebhook("github", r, body)
	if err != nil {
		t.Fatal(err)
	}
	if ev.Action != "push" || ev.Branch != "main" || ev.Commit != "abc123" {
		t.Fatalf("bad push event: %+v", ev)
	}
	if ev.RepoFullName != "acme/app" {
		t.Fatalf("repo=%q", ev.RepoFullName)
	}
	if len(ev.ChangedFiles) < 1 {
		t.Fatal("expected changed files")
	}
}

func TestParseGitHubPRClosed(t *testing.T) {
	body := []byte(`{
		"action":"closed",
		"number":42,
		"repository":{"full_name":"acme/app"},
		"pull_request":{
			"head":{"ref":"feature","sha":"def"},
			"base":{"ref":"main"},
			"title":"My PR"
		}
	}`)
	r := &http.Request{Header: http.Header{"X-Github-Event": []string{"pull_request"}}}
	ev, err := ParseWebhook("github", r, body)
	if err != nil {
		t.Fatal(err)
	}
	if !ev.IsClosed() || ev.PRNumber != 42 || ev.BaseBranch != "main" {
		t.Fatalf("bad closed: %+v", ev)
	}
}

func TestParseGitHubPROpen(t *testing.T) {
	body := []byte(`{
		"action":"opened",
		"number":7,
		"repository":{"full_name":"acme/app"},
		"pull_request":{
			"head":{"ref":"feat","sha":"aaa"},
			"base":{"ref":"main"},
			"title":"Hello"
		}
	}`)
	r := &http.Request{Header: http.Header{"X-Github-Event": []string{"pull_request"}}}
	ev, err := ParseWebhook("github", r, body)
	if err != nil {
		t.Fatal(err)
	}
	if !ev.IsPreviewOpen() || ev.Branch != "feat" || ev.BaseBranch != "main" {
		t.Fatalf("bad open: %+v", ev)
	}
}

func TestParseGitLabMR(t *testing.T) {
	body := []byte(`{
		"object_kind":"merge_request",
		"project":{"path_with_namespace":"acme/app"},
		"object_attributes":{
			"action":"open",
			"iid":9,
			"source_branch":"feat",
			"target_branch":"main",
			"title":"MR title",
			"last_commit":{"id":"c1","message":"wip"}
		}
	}`)
	r := &http.Request{Header: http.Header{"X-Gitlab-Event": []string{"Merge Request Hook"}}}
	ev, err := ParseWebhook("gitlab", r, body)
	if err != nil {
		t.Fatal(err)
	}
	if ev.Action != "opened" || ev.PRNumber != 9 || ev.BaseBranch != "main" || ev.Commit != "c1" {
		t.Fatalf("bad mr: %+v", ev)
	}
}

func TestParseGitLabMRClose(t *testing.T) {
	body := []byte(`{
		"object_kind":"merge_request",
		"project":{"path_with_namespace":"acme/app"},
		"object_attributes":{
			"action":"merge",
			"iid":9,
			"source_branch":"feat",
			"target_branch":"main",
			"title":"MR",
			"last_commit":{"id":"c1","message":"done"}
		}
	}`)
	r := &http.Request{}
	ev, err := ParseWebhook("gitlab", r, body)
	if err != nil {
		t.Fatal(err)
	}
	if !ev.IsClosed() {
		t.Fatalf("expected closed: %+v", ev)
	}
}

func TestParseGiteaPR(t *testing.T) {
	body := []byte(`{
		"action":"synchronize",
		"number":3,
		"repository":{"full_name":"acme/app"},
		"pull_request":{
			"head":{"ref":"x","sha":"s"},
			"base":{"ref":"main"},
			"title":"gitea pr"
		}
	}`)
	r := &http.Request{Header: http.Header{"X-Gitea-Event": []string{"pull_request"}}}
	ev, err := ParseWebhook("gitea", r, body)
	if err != nil {
		t.Fatal(err)
	}
	if ev.Provider != "gitea" || ev.PRNumber != 3 || ev.Action != "synchronize" {
		t.Fatalf("%+v", ev)
	}
}

func TestParseBitbucketPushAndPR(t *testing.T) {
	pushBody := []byte(`{
		"repository":{"full_name":"acme/app"},
		"push":{"changes":[{"new":{"name":"main","target":{"hash":"h1","message":"m1"}},"commits":[{"message":"m1"}]}]}
	}`)
	r := &http.Request{Header: http.Header{"X-Event-Key": []string{"repo:push"}}}
	ev, err := ParseWebhook("bitbucket", r, pushBody)
	if err != nil {
		t.Fatal(err)
	}
	if ev.Branch != "main" || ev.Commit != "h1" {
		t.Fatalf("push %+v", ev)
	}

	prBody := []byte(`{
		"repository":{"full_name":"acme/app"},
		"pullrequest":{
			"id":11,
			"title":"bb pr",
			"source":{"branch":{"name":"feat"},"commit":{"hash":"abc"}},
			"destination":{"branch":{"name":"main"}}
		}
	}`)
	r2 := &http.Request{Header: http.Header{"X-Event-Key": []string{"pullrequest:created"}}}
	ev2, err := ParseWebhook("bitbucket", r2, prBody)
	if err != nil {
		t.Fatal(err)
	}
	if ev2.PRNumber != 11 || ev2.BaseBranch != "main" || ev2.Action != "opened" {
		t.Fatalf("pr %+v", ev2)
	}

	r3 := &http.Request{Header: http.Header{"X-Event-Key": []string{"pullrequest:fulfilled"}}}
	ev3, err := ParseWebhook("bitbucket", r3, prBody)
	if err != nil {
		t.Fatal(err)
	}
	if !ev3.IsClosed() {
		t.Fatal("fulfilled should close")
	}
}

func TestDetectProvider(t *testing.T) {
	r := &http.Request{Header: http.Header{"X-Event-Key": []string{"repo:push"}}}
	if DetectProvider(r) != "bitbucket" {
		t.Fatal(DetectProvider(r))
	}
	// Gitea compatibility headers must win over X-GitHub-Event
	r2 := &http.Request{Header: http.Header{
		"X-Gitea-Event":  []string{"push"},
		"X-Github-Event": []string{"push"},
	}}
	if DetectProvider(r2) != "gitea" {
		t.Fatalf("got %q", DetectProvider(r2))
	}
}

func TestIsNullCommit(t *testing.T) {
	if !IsNullCommit("0000000000000000000000000000000000000000") {
		t.Fatal("expected null")
	}
	if IsNullCommit("abc123") || IsNullCommit("") || IsNullCommit("HEAD") {
		t.Fatal("unexpected null")
	}
}

func TestParseGitHubPRIgnoredAction(t *testing.T) {
	body := []byte(`{
		"action":"labeled",
		"number":1,
		"repository":{"full_name":"acme/app"},
		"pull_request":{
			"head":{"ref":"x","sha":"s"},
			"base":{"ref":"main"},
			"title":"t"
		}
	}`)
	r := &http.Request{Header: http.Header{"X-Github-Event": []string{"pull_request"}}}
	ev, err := ParseWebhook("github", r, body)
	if err != nil {
		t.Fatal(err)
	}
	if ev.Action != "ignored" {
		t.Fatalf("%+v", ev)
	}
}

func TestVerifyGitHubSignatureRawHex(t *testing.T) {
	secret := "s3cret"
	body := []byte(`{"ok":true}`)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	sum := hex.EncodeToString(mac.Sum(nil))
	if !VerifyGitHubSignature(secret, body, "sha256="+sum) {
		t.Fatal("prefixed")
	}
	if !VerifyGitHubSignature(secret, body, sum) {
		t.Fatal("raw hex")
	}
	if VerifyGitHubSignature(secret, body, "deadbeef") {
		t.Fatal("bad sig")
	}
}
