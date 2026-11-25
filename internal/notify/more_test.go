package notify

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWebhookPayloadHeadersAndMetadata(t *testing.T) {
	var last struct {
		path    string
		agent   string
		hook    string
		ctype   string
		payload map[string]any
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		last.path = r.URL.Path
		last.agent = r.Header.Get("User-Agent")
		last.hook = r.Header.Get("X-Hook")
		last.ctype = r.Header.Get("Content-Type")
		last.payload = map[string]any{}
		_ = json.Unmarshal(body, &last.payload)
		if strings.HasSuffix(r.URL.Path, "/fail") {
			http.Error(w, "nope", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)
	policy := URLPolicy{AllowLocalhost: true}
	ctx := t.Context()

	if err := SendWebhook(ctx, WebhookConfig{
		URL: srv.URL + "/hook", Headers: map[string]string{"X-Hook": "yes"},
	}, Event{TeamID: "team-1", Type: "deployment_failure", Title: "Deploy failed", Message: "boom"}, policy); err != nil {
		t.Fatal(err)
	}
	if last.path != "/hook" || last.agent != "Dockfin/0.1" || last.hook != "yes" || last.ctype != "application/json" {
		t.Fatalf("%+v", last)
	}
	if last.payload["success"] != false || last.payload["type"] != "deployment_failure" || last.payload["event"] != "deployment_failure" || last.payload["title"] != "Deploy failed" || last.payload["team_id"] != "team-1" {
		t.Fatalf("payload %#v", last.payload)
	}

	if err := SendWebhook(ctx, WebhookConfig{URL: srv.URL + "/ok"}, Event{Type: "deployment_success", Title: "ok", Message: "up"}, policy); err != nil {
		t.Fatal(err)
	}
	if last.payload["success"] != true || last.payload["type"] != "deployment_success" {
		t.Fatalf("success payload %#v", last.payload)
	}
	if err := SendWebhook(ctx, WebhookConfig{URL: srv.URL + "/test"}, Event{Type: "test", Title: "ping", Message: "hi"}, policy); err != nil {
		t.Fatal(err)
	}
	if last.payload["success"] != true || last.payload["type"] != "test" {
		t.Fatalf("test payload %#v", last.payload)
	}
	if err := SendWebhook(ctx, WebhookConfig{URL: srv.URL + "/fail"}, Event{Type: "test", Title: "ping"}, policy); err == nil || !strings.Contains(err.Error(), "500") {
		t.Fatal(err)
	}
	if err := SendWebhook(ctx, WebhookConfig{}, Event{Type: "test"}, policy); err == nil || !strings.Contains(err.Error(), "required") {
		t.Fatal(err)
	}

	for _, raw := range []string{
		"http://169.254.169.254/latest/meta-data",
		"http://metadata.google.internal/computeMetadata/v1/",
		"http://metadata/latest",
	} {
		if err := SendWebhook(ctx, WebhookConfig{URL: raw}, Event{Type: "test", Title: "nope"}, policy); err == nil {
			t.Fatalf("metadata webhook accepted %s", raw)
		}
		if err := SendDiscord(ctx, DiscordConfig{WebhookURL: raw}, Event{Type: "test", Title: "nope"}, policy); err == nil {
			t.Fatalf("metadata discord accepted %s", raw)
		}
		if err := SendSlack(ctx, SlackConfig{WebhookURL: raw}, Event{Type: "test", Title: "nope"}, policy); err == nil {
			t.Fatalf("metadata slack accepted %s", raw)
		}
	}

	if err := SendDiscord(ctx, DiscordConfig{WebhookURL: srv.URL + "/discord", PingEnabled: true}, Event{Type: "deployment_success", Title: "ok", Message: "up"}, policy); err != nil {
		t.Fatal(err)
	}
	if content, _ := last.payload["content"].(string); strings.Contains(content, "@here") || !strings.Contains(content, "ok") {
		t.Fatalf("discord content %q", last.payload["content"])
	}
	if err := SendDiscord(ctx, DiscordConfig{WebhookURL: srv.URL + "/discord", PingEnabled: true}, Event{Type: "deployment_failure", Title: "down", Message: "boom"}, policy); err != nil {
		t.Fatal(err)
	}
	if content, _ := last.payload["content"].(string); !strings.HasPrefix(content, "@here") || !strings.Contains(content, "boom") {
		t.Fatalf("discord ping %q", last.payload["content"])
	}
	if err := SendDiscord(ctx, DiscordConfig{}, Event{Type: "test"}, policy); err == nil || !strings.Contains(err.Error(), "required") {
		t.Fatal(err)
	}

	if err := SendSlack(ctx, SlackConfig{WebhookURL: srv.URL + "/slack"}, Event{Type: "backup_failure", Title: "Backup failed", Message: "disk"}, policy); err != nil {
		t.Fatal(err)
	}
	text, _ := last.payload["text"].(string)
	if !strings.Contains(text, "*Backup failed*") || !strings.Contains(text, "disk") {
		t.Fatalf("slack %q", text)
	}
	if err := SendSlack(ctx, SlackConfig{}, Event{Type: "test"}, policy); err == nil || !strings.Contains(err.Error(), "required") {
		t.Fatal(err)
	}
}
