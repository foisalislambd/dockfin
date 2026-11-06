package git

import (
	"net/http"
	"testing"
)

func TestNormalizeGitLabSystemHook(t *testing.T) {
	raw := []byte(`{"event_name":"push","project_id":9,"ref":"refs/heads/main","after":"abc","project":{"path_with_namespace":"acme/web"},"commits":[{"message":"hi"}]}`)
	body, err := NormalizeGitLabSystemHook(raw)
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("X-Gitlab-Event", "System Hook")
	ev, err := ParseWebhook("gitlab", req, body)
	if err != nil {
		t.Fatal(err)
	}
	if ev.Action != "push" || ev.Branch != "main" || ev.RepoFullName != "acme/web" {
		t.Fatalf("%+v", ev)
	}
}
