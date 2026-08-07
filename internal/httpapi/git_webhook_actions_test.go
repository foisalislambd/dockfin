package httpapi

import (
	"net/http"
	"testing"

	"github.com/dockfin/dockfin/internal/git"
)

func TestDetectWebhookProvider(t *testing.T) {
	r := &http.Request{Header: http.Header{}}
	if got := detectWebhookProvider(r, ""); got != "github" {
		t.Fatalf("default=%q", got)
	}
	if got := detectWebhookProvider(r, "GitLab"); got != "gitlab" {
		t.Fatalf("query=%q", got)
	}
	r2 := &http.Request{Header: http.Header{"X-Event-Key": []string{"repo:push"}}}
	if got := detectWebhookProvider(r2, ""); got != "bitbucket" {
		t.Fatalf("bb=%q", got)
	}
	r3 := &http.Request{Header: http.Header{"X-Gitea-Event": []string{"push"}}}
	if got := detectWebhookProvider(r3, ""); got != "gitea" {
		t.Fatalf("gitea=%q", got)
	}
}

func TestPushEventClosedHelpers(t *testing.T) {
	ev := &git.PushEvent{Action: "closed", PRNumber: 1}
	if !ev.IsClosed() || ev.IsPreviewOpen() {
		t.Fatal(ev)
	}
	ev2 := &git.PushEvent{Action: "opened", PRNumber: 2}
	if !ev2.IsPreviewOpen() {
		t.Fatal(ev2)
	}
}
