package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/goolify/goolify/internal/mailer"
	"github.com/goolify/goolify/internal/store"
)

// Dispatcher sends events to all enabled team notification channels.
type Dispatcher struct {
	Store *store.Store
}

func (d *Dispatcher) Send(ctx context.Context, teamID uuid.UUID, event Event) {
	if d == nil || d.Store == nil {
		return
	}
	event.TeamID = teamID.String()
	if !event.Critical {
		event.Critical = IsCritical(event.Type)
	}
	settings, err := d.Store.ListEnabledNotifications(ctx, teamID)
	if err != nil {
		return
	}
	for _, n := range settings {
		if event.Type != "test" && !EventAllowed(n.Events, event.Type) {
			continue
		}
		cfgJSON, err := d.Store.Box.DecryptString(n.ConfigEnc)
		if err != nil || cfgJSON == "" {
			cfgJSON = "{}"
		}
		_ = d.sendChannel(ctx, teamID, n.Channel, cfgJSON, event)
	}
}

func (d *Dispatcher) SendChannel(ctx context.Context, teamID uuid.UUID, channel, cfgJSON string, event Event) error {
	event.TeamID = teamID.String()
	if !event.Critical {
		event.Critical = IsCritical(event.Type)
	}
	return d.sendChannel(ctx, teamID, channel, cfgJSON, event)
}

func (d *Dispatcher) sendChannel(ctx context.Context, teamID uuid.UUID, channel, cfgJSON string, event Event) error {
	switch channel {
	case "webhook":
		var cfg WebhookConfig
		_ = json.Unmarshal([]byte(cfgJSON), &cfg)
		return SendWebhook(ctx, cfg, event)
	case "discord":
		var cfg DiscordConfig
		_ = json.Unmarshal([]byte(cfgJSON), &cfg)
		return SendDiscord(ctx, cfg, event)
	case "slack":
		var cfg SlackConfig
		_ = json.Unmarshal([]byte(cfgJSON), &cfg)
		return SendSlack(ctx, cfg, event)
	case "telegram":
		var cfg TelegramConfig
		_ = json.Unmarshal([]byte(cfgJSON), &cfg)
		return SendTelegram(ctx, cfg, event)
	case "pushover":
		var cfg PushoverConfig
		_ = json.Unmarshal([]byte(cfgJSON), &cfg)
		return SendPushover(ctx, cfg, event)
	case "email":
		var cfg EmailConfig
		_ = json.Unmarshal([]byte(cfgJSON), &cfg)
		return d.sendEmail(ctx, teamID, cfg, event)
	default:
		return nil
	}
}

func (d *Dispatcher) sendEmail(ctx context.Context, teamID uuid.UUID, cfg EmailConfig, event Event) error {
	to := parseRecipients(cfg.Recipients)
	if len(to) == 0 {
		emails, err := d.Store.TeamMemberEmails(ctx, teamID)
		if err != nil {
			return err
		}
		to = emails
	}
	if len(to) == 0 {
		return fmt.Errorf("no email recipients (add team members or set recipients)")
	}

	msg := mailer.Message{
		FromName: cfg.SMTPFromName,
		From:     cfg.SMTPFromAddress,
		To:       to,
		Subject:  "Goolify: " + event.Title,
		Body:     event.Message + "\n\n— Goolify",
	}

	if cfg.UseInstanceEmailSettings {
		inst, err := d.Store.GetInstanceSettings(ctx)
		if err != nil {
			return err
		}
		user, pass, resendKey, err := d.Store.SMTPMaterial(ctx)
		if err != nil {
			return err
		}
		msg.FromName = inst.SMTPFromName
		msg.From = inst.SMTPFromAddress
		if msg.From == "" {
			return fmt.Errorf("instance email from address is not configured")
		}
		if inst.ResendEnabled {
			if resendKey == "" {
				return fmt.Errorf("instance Resend API key is not configured")
			}
			return mailer.SendResend(ctx, resendKey, msg)
		}
		if inst.SMTPEnabled {
			if inst.SMTPHost == "" {
				return fmt.Errorf("instance SMTP host is not configured")
			}
			timeout := 30
			if inst.SMTPTimeout != nil {
				timeout = *inst.SMTPTimeout
			}
			return mailer.SendSMTP(ctx, mailer.SMTPConfig{
				Host:       inst.SMTPHost,
				Port:       inst.SMTPPort,
				Encryption: inst.SMTPEncryption,
				Username:   user,
				Password:   pass,
				TimeoutSec: timeout,
			}, msg)
		}
		return fmt.Errorf("instance email is not enabled (configure SMTP or Resend in Settings → Email)")
	}

	if msg.From == "" {
		return fmt.Errorf("from address is required")
	}
	if cfg.ResendEnabled {
		if cfg.ResendAPIKey == "" {
			return fmt.Errorf("resend api key required")
		}
		return mailer.SendResend(ctx, cfg.ResendAPIKey, msg)
	}
	if cfg.SMTPEnabled {
		port := cfg.SMTPPort
		if port == 0 {
			port = 587
		}
		timeout := cfg.SMTPTimeout
		if timeout == 0 {
			timeout = 30
		}
		enc := cfg.SMTPEncryption
		if enc == "" {
			enc = "starttls"
		}
		if cfg.SMTPHost == "" {
			return fmt.Errorf("smtp host required")
		}
		return mailer.SendSMTP(ctx, mailer.SMTPConfig{
			Host:       cfg.SMTPHost,
			Port:       port,
			Encryption: enc,
			Username:   cfg.SMTPUsername,
			Password:   cfg.SMTPPassword,
			TimeoutSec: timeout,
		}, msg)
	}
	return fmt.Errorf("enable SMTP, Resend, or use instance email settings")
}

func parseRecipients(s string) []string {
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n'
	})
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
