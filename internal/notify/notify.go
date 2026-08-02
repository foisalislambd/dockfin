package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Event struct {
	TeamID   string
	Type     string
	Title    string
	Message  string
	Critical bool
}

type WebhookConfig struct {
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
}

type DiscordConfig struct {
	WebhookURL  string `json:"webhook_url"`
	PingEnabled bool   `json:"ping_enabled"`
}

type SlackConfig struct {
	WebhookURL string `json:"webhook_url"`
}

type TelegramConfig struct {
	BotToken  string            `json:"bot_token"`
	ChatID    string            `json:"chat_id"`
	ThreadIDs map[string]string `json:"thread_ids"`
}

type PushoverConfig struct {
	UserKey  string `json:"user_key"`
	APIToken string `json:"api_token"`
}

type EmailConfig struct {
	UseInstanceEmailSettings bool   `json:"use_instance_email_settings"`
	SMTPFromName             string `json:"smtp_from_name"`
	SMTPFromAddress          string `json:"smtp_from_address"`
	SMTPEnabled              bool   `json:"smtp_enabled"`
	SMTPHost                 string `json:"smtp_host"`
	SMTPPort                 int    `json:"smtp_port"`
	SMTPEncryption           string `json:"smtp_encryption"`
	SMTPUsername             string `json:"smtp_username"`
	SMTPPassword             string `json:"smtp_password"`
	SMTPTimeout              int    `json:"smtp_timeout"`
	ResendEnabled            bool   `json:"resend_enabled"`
	ResendAPIKey             string `json:"resend_api_key"`
	Recipients               string `json:"recipients"` // comma-separated; empty = team members
}

func EventAllowed(events []string, want string) bool {
	// Empty selection means notify for nothing (test events bypass this in Dispatcher).
	if len(events) == 0 {
		return false
	}
	want = NormalizeEvent(want)
	for _, e := range events {
		if NormalizeEvent(e) == want {
			return true
		}
	}
	return false
}

// NormalizeEvent maps legacy aliases to Coolify-style event keys.
func NormalizeEvent(e string) string {
	switch strings.ToLower(strings.TrimSpace(e)) {
	case "deployment_failed":
		return "deployment_failure"
	case "backup_failed":
		return "backup_failure"
	default:
		return strings.ToLower(strings.TrimSpace(e))
	}
}

func NormalizeEvents(events []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(events))
	for _, e := range events {
		n := NormalizeEvent(e)
		if n == "" {
			continue
		}
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		out = append(out, n)
	}
	return out
}

func IsCritical(eventType string) bool {
	switch strings.ToLower(eventType) {
	case "deployment_failed", "deployment_failure", "backup_failure", "backup_failed",
		"scheduled_task_failure", "docker_cleanup_failure", "server_unreachable",
		"server_disk_usage", "server_patch", "traefik_outdated", "test":
		return true
	default:
		return false
	}
}

func DefaultEvents() []string {
	return []string{
		"deployment_failure",
		"backup_failure",
		"scheduled_task_failure",
		"docker_cleanup_failure",
		"server_disk_usage",
		"server_unreachable",
		"server_patch",
		"traefik_outdated",
	}
}

func SendWebhook(ctx context.Context, cfg WebhookConfig, event Event) error {
	if cfg.URL == "" {
		return fmt.Errorf("webhook url required")
	}
	success := event.Type == "test" || !IsCritical(event.Type)
	body, _ := json.Marshal(map[string]any{
		"success": success,
		"type":    event.Type,
		"event":   event.Type,
		"title":   event.Title,
		"message": event.Message,
		"team_id": event.TeamID,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Dockfin/0.1")
	for k, v := range cfg.Headers {
		req.Header.Set(k, v)
	}
	return doRequest(req)
}

func SendDiscord(ctx context.Context, cfg DiscordConfig, event Event) error {
	if cfg.WebhookURL == "" {
		return fmt.Errorf("discord webhook required")
	}
	content := fmt.Sprintf("**%s**\n%s", event.Title, event.Message)
	if cfg.PingEnabled && (event.Critical || IsCritical(event.Type)) {
		content = "@here\n" + content
	}
	body, _ := json.Marshal(map[string]any{"content": content})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return doRequest(req)
}

func SendSlack(ctx context.Context, cfg SlackConfig, event Event) error {
	if cfg.WebhookURL == "" {
		return fmt.Errorf("slack webhook required")
	}
	body, _ := json.Marshal(map[string]any{
		"text": fmt.Sprintf("*%s*\n%s", event.Title, event.Message),
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return doRequest(req)
}

func SendTelegram(ctx context.Context, cfg TelegramConfig, event Event) error {
	if cfg.BotToken == "" || cfg.ChatID == "" {
		return fmt.Errorf("telegram bot_token and chat_id required")
	}
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", cfg.BotToken)
	payload := map[string]any{
		"chat_id": cfg.ChatID,
		"text":    fmt.Sprintf("Dockfin: %s\n%s", event.Title, event.Message),
	}
	if cfg.ThreadIDs != nil {
		tid := strings.TrimSpace(cfg.ThreadIDs[event.Type])
		if tid == "" {
			// also try normalized / legacy keys
			tid = strings.TrimSpace(cfg.ThreadIDs[NormalizeEvent(event.Type)])
		}
		if tid != "" {
			var n int
			if _, err := fmt.Sscanf(tid, "%d", &n); err == nil {
				payload["message_thread_id"] = n
			}
		}
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return doRequest(req)
}

func SendPushover(ctx context.Context, cfg PushoverConfig, event Event) error {
	if cfg.UserKey == "" || cfg.APIToken == "" {
		return fmt.Errorf("pushover user_key and api_token required")
	}
	form := url.Values{}
	form.Set("token", cfg.APIToken)
	form.Set("user", cfg.UserKey)
	form.Set("title", event.Title)
	form.Set("message", event.Message)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.pushover.net/1/messages.json", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return doRequest(req)
}

func doRequest(req *http.Request) error {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return fmt.Errorf("%s status %d", req.URL.Host, resp.StatusCode)
	}
	return nil
}
