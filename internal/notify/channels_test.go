package notify

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dockfin/dockfin/internal/testkit"
)

func TestChannelsPostToTheAllowedHost(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got = r.URL.Path + " " + string(b)
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)
	policy := URLPolicy{AllowLocalhost: true}
	event := Event{Type: "deployment_failure", Title: "Deploy failed", Message: "boom", Critical: true}
	ctx := t.Context()
	if err := SendWebhook(ctx, WebhookConfig{URL: srv.URL + "/hook", Headers: map[string]string{"X-Test": "1"}}, event, policy); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "/hook") || !strings.Contains(got, "deployment_failure") {
		t.Fatal(got)
	}
	if err := SendDiscord(ctx, DiscordConfig{WebhookURL: srv.URL + "/discord", PingEnabled: true}, event, policy); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "@here") {
		t.Fatal(got)
	}
	if err := SendSlack(ctx, SlackConfig{WebhookURL: srv.URL + "/slack"}, event, policy); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "Deploy failed") {
		t.Fatal(got)
	}
	if err := SendWebhook(ctx, WebhookConfig{URL: "http://169.254.169.254/latest"}, event, policy); err == nil {
		t.Fatal("metadata webhook accepted")
	}
	short, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	if err := SendTelegram(short, TelegramConfig{BotToken: "t", ChatID: "1"}, event); err == nil {
		t.Fatal("telegram without a bot endpoint should not succeed in this test")
	}
	if err := SendPushover(short, PushoverConfig{UserKey: "u", APIToken: "a"}, event); err == nil {
		t.Fatal("pushover without network should not succeed in this test")
	}
}

func TestEmailWithoutRecipientsFails(t *testing.T) {
	st := testkit.OpenStore(t)
	ctx := t.Context()
	_, team, err := st.CreateUserWithPersonalTeam(ctx, "mail@example.com", "Mail", "hash")
	if err != nil {
		t.Fatal(err)
	}
	d := &Dispatcher{Store: st}
	err = d.SendChannel(ctx, team.ID, "email", `{"recipients":""}`, Event{Type: "test", Title: "hi"})
	if err == nil {
		t.Fatal("email with no smtp settings succeeded")
	}
	err = d.SendChannel(ctx, team.ID, "webhook", `{"url":"http://127.0.0.1/nope"}`, Event{Type: "test", Title: "hi"})
	if err == nil {
		t.Fatal("localhost webhook allowed by default")
	}
	if err := d.SendChannel(ctx, team.ID, "nope", `{}`, Event{Type: "test"}); err != nil {
		t.Fatal(err)
	}
}
