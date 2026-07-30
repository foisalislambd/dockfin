package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Event struct {
	TeamID  string
	Type    string
	Title   string
	Message string
}

type WebhookConfig struct {
	URL string `json:"url"`
}

type DiscordConfig struct {
	WebhookURL string `json:"webhook_url"`
}

type SlackConfig struct {
	WebhookURL string `json:"webhook_url"`
}

func SendWebhook(ctx context.Context, url string, event Event) error {
	body, _ := json.Marshal(map[string]any{
		"type":    event.Type,
		"title":   event.Title,
		"message": event.Message,
		"team_id": event.TeamID,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Goolify/0.1")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook status %d", resp.StatusCode)
	}
	return nil
}

func SendDiscord(ctx context.Context, webhookURL string, event Event) error {
	body, _ := json.Marshal(map[string]any{
		"content": fmt.Sprintf("**%s**\n%s", event.Title, event.Message),
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("discord status %d", resp.StatusCode)
	}
	return nil
}

func SendSlack(ctx context.Context, webhookURL string, event Event) error {
	body, _ := json.Marshal(map[string]any{
		"text": fmt.Sprintf("*%s*\n%s", event.Title, event.Message),
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("slack status %d", resp.StatusCode)
	}
	return nil
}
