package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/dockfin/dockfin/internal/notify"
	"github.com/dockfin/dockfin/internal/store"
)

var notificationChannels = []string{"email", "discord", "telegram", "slack", "pushover", "webhook"}

type notificationDTO struct {
	ID      string          `json:"id,omitempty"`
	Channel string          `json:"channel"`
	Enabled bool            `json:"enabled"`
	Events  []string        `json:"events"`
	Config  json.RawMessage `json:"config"`
}

func (a *API) handleListNotifications(w http.ResponseWriter, r *http.Request) {
	teamID := currentTeamID(r)
	existing, err := a.Store.ListNotificationSettings(r.Context(), teamID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	byCh := map[string]store.NotificationSetting{}
	for _, n := range existing {
		byCh[n.Channel] = n
	}
	out := make([]notificationDTO, 0, len(notificationChannels))
	for _, ch := range notificationChannels {
		dto := notificationDTO{
			Channel: ch,
			Enabled: false,
			Events:  notify.DefaultEvents(),
			Config:  json.RawMessage(defaultNotificationConfig(ch)),
		}
		if n, ok := byCh[ch]; ok {
			dto.ID = n.ID.String()
			dto.Enabled = n.Enabled
			dto.Events = notify.NormalizeEvents(n.Events)
			cfg := defaultNotificationConfig(ch)
			if n.ConfigEnc != "" {
				if plain, err := a.Store.Box.DecryptString(n.ConfigEnc); err == nil && plain != "" {
					cfg = plain
				}
			}
			dto.Config = json.RawMessage(redactNotificationConfig(ch, cfg))
		}
		out = append(out, dto)
	}
	writeJSON(w, http.StatusOK, map[string]any{"notifications": out})
}

func (a *API) handleUpsertNotification(w http.ResponseWriter, r *http.Request) {
	channel := strings.ToLower(chi.URLParam(r, "channel"))
	if !validNotificationChannel(channel) {
		writeError(w, http.StatusBadRequest, "invalid channel")
		return
	}
	var body struct {
		Enabled bool             `json:"enabled"`
		Config  json.RawMessage  `json:"config"`
		Events  *[]string        `json:"events"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	var events []string
	if body.Events == nil {
		events = notify.DefaultEvents()
	} else {
		events = notify.NormalizeEvents(*body.Events)
	}
	cfgBytes := body.Config
	if len(cfgBytes) == 0 {
		cfgBytes = json.RawMessage(defaultNotificationConfig(channel))
	}

	// Merge secrets: empty password fields keep previous values.
	cfgBytes = a.mergeNotificationSecrets(r, channel, cfgBytes)

	if body.Enabled {
		if err := validateNotificationConfig(channel, cfgBytes); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	enc, err := a.Store.Box.EncryptString(string(cfgBytes))
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	n, err := a.Store.UpsertNotificationSetting(r.Context(), currentTeamID(r), channel, body.Enabled, enc, events)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"notification": notificationDTO{
			ID:      n.ID.String(),
			Channel: n.Channel,
			Enabled: n.Enabled,
			Events:  notify.NormalizeEvents(n.Events),
			Config:  json.RawMessage(redactNotificationConfig(channel, string(cfgBytes))),
		},
	})
}

func (a *API) handleTestNotification(w http.ResponseWriter, r *http.Request) {
	channel := strings.ToLower(chi.URLParam(r, "channel"))
	if !validNotificationChannel(channel) {
		writeError(w, http.StatusBadRequest, "invalid channel")
		return
	}
	var body struct {
		Email string `json:"email"`
	}
	if err := decodeJSONOptional(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	teamID := currentTeamID(r)
	n, err := a.Store.GetNotificationSetting(r.Context(), teamID, channel)
	if err != nil {
		writeError(w, http.StatusBadRequest, "save and enable this channel before sending a test")
		return
	}
	if !n.Enabled {
		writeError(w, http.StatusBadRequest, "channel is disabled")
		return
	}
	cfgJSON, err := a.Store.Box.DecryptString(n.ConfigEnc)
	if err != nil || cfgJSON == "" {
		cfgJSON = "{}"
	}
	if channel == "email" && strings.TrimSpace(body.Email) != "" {
		var cfg notify.EmailConfig
		_ = json.Unmarshal([]byte(cfgJSON), &cfg)
		cfg.Recipients = strings.TrimSpace(body.Email)
		b, _ := json.Marshal(cfg)
		cfgJSON = string(b)
	}

	ev := notify.Event{
		Type:     "test",
		Title:    "Test Notification",
		Message:  "If you are seeing this, it means that your " + channel + " settings are correct.",
		Critical: true,
	}
	disp := &notify.Dispatcher{Store: a.Store}
	if err := disp.SendChannel(r.Context(), teamID, channel, cfgJSON, ev); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "sent"})
}

func (a *API) mergeNotificationSecrets(r *http.Request, channel string, incoming json.RawMessage) json.RawMessage {
	prev, err := a.Store.GetNotificationSetting(r.Context(), currentTeamID(r), channel)
	if err != nil || prev.ConfigEnc == "" {
		return incoming
	}
	prevPlain, err := a.Store.Box.DecryptString(prev.ConfigEnc)
	if err != nil || prevPlain == "" {
		return incoming
	}
	var prevMap, nextMap map[string]any
	if json.Unmarshal([]byte(prevPlain), &prevMap) != nil {
		return incoming
	}
	if json.Unmarshal(incoming, &nextMap) != nil {
		return incoming
	}
	for _, k := range notificationSecretKeys(channel) {
		v, ok := nextMap[k].(string)
		if !ok {
			continue
		}
		if v == "" || v == "********" {
			if old, ok := prevMap[k]; ok {
				nextMap[k] = old
			}
		}
	}
	out, err := json.Marshal(nextMap)
	if err != nil {
		return incoming
	}
	return out
}

func validNotificationChannel(ch string) bool {
	for _, c := range notificationChannels {
		if c == ch {
			return true
		}
	}
	return false
}

func notificationSecretKeys(channel string) []string {
	keys := []string{"smtp_password", "resend_api_key", "bot_token", "api_token", "user_key"}
	// webhook_url is password-masked in the UI for Discord/Slack; keep previous if blanked.
	if channel == "discord" || channel == "slack" {
		keys = append(keys, "webhook_url")
	}
	return keys
}

// redactNotificationConfig blanks secret fields so the browser never stores plaintext secrets.
// Empty values on save are merged back from the previous encrypted config.
func redactNotificationConfig(channel, cfgJSON string) string {
	var m map[string]any
	if json.Unmarshal([]byte(cfgJSON), &m) != nil {
		return cfgJSON
	}
	for _, k := range notificationSecretKeys(channel) {
		if v, ok := m[k].(string); ok && v != "" {
			m[k] = ""
		}
	}
	out, err := json.Marshal(m)
	if err != nil {
		return cfgJSON
	}
	return string(out)
}

func defaultNotificationConfig(channel string) string {
	switch channel {
	case "email":
		return `{"use_instance_email_settings":true,"smtp_from_name":"","smtp_from_address":"","smtp_enabled":false,"smtp_host":"","smtp_port":587,"smtp_encryption":"starttls","smtp_username":"","smtp_password":"","smtp_timeout":30,"resend_enabled":false,"resend_api_key":"","recipients":""}`
	case "discord":
		return `{"webhook_url":"","ping_enabled":false}`
	case "telegram":
		return `{"bot_token":"","chat_id":"","thread_ids":{}}`
	case "slack":
		return `{"webhook_url":""}`
	case "pushover":
		return `{"user_key":"","api_token":""}`
	case "webhook":
		return `{"url":"","headers":{}}`
	default:
		return `{}`
	}
}

func validateNotificationConfig(channel string, cfg json.RawMessage) error {
	switch channel {
	case "discord":
		var c notify.DiscordConfig
		_ = json.Unmarshal(cfg, &c)
		if strings.TrimSpace(c.WebhookURL) == "" {
			return errStr("Discord Webhook URL is required.")
		}
	case "slack":
		var c notify.SlackConfig
		_ = json.Unmarshal(cfg, &c)
		if strings.TrimSpace(c.WebhookURL) == "" {
			return errStr("Slack Webhook URL is required.")
		}
	case "telegram":
		var c notify.TelegramConfig
		_ = json.Unmarshal(cfg, &c)
		if strings.TrimSpace(c.BotToken) == "" || strings.TrimSpace(c.ChatID) == "" {
			return errStr("Telegram bot token and chat ID are required.")
		}
	case "pushover":
		var c notify.PushoverConfig
		_ = json.Unmarshal(cfg, &c)
		if strings.TrimSpace(c.UserKey) == "" || strings.TrimSpace(c.APIToken) == "" {
			return errStr("Pushover user key and API token are required.")
		}
	case "webhook":
		var c notify.WebhookConfig
		_ = json.Unmarshal(cfg, &c)
		if strings.TrimSpace(c.URL) == "" {
			return errStr("Webhook URL is required.")
		}
	case "email":
		var c notify.EmailConfig
		_ = json.Unmarshal(cfg, &c)
		if !c.UseInstanceEmailSettings && !c.SMTPEnabled && !c.ResendEnabled {
			return errStr("Enable SMTP, Resend, or use instance email settings.")
		}
	}
	return nil
}
