package mailer

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/smtp"
	"strings"
	"time"
)

type Message struct {
	FromName string
	From     string
	To       []string
	Subject  string
	Body     string
}

type SMTPConfig struct {
	Host       string
	Port       int
	Encryption string // starttls | tls | none
	Username   string
	Password   string
	TimeoutSec int
}

func SendSMTP(ctx context.Context, cfg SMTPConfig, msg Message) error {
	if cfg.Host == "" || cfg.Port == 0 {
		return fmt.Errorf("smtp host/port required")
	}
	if msg.From == "" || len(msg.To) == 0 {
		return fmt.Errorf("from and to required")
	}
	timeout := time.Duration(cfg.TimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	fromHeader := msg.From
	if msg.FromName != "" {
		fromHeader = fmt.Sprintf("%s <%s>", msg.FromName, msg.From)
	}
	payload := []byte(fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		fromHeader, strings.Join(msg.To, ", "), msg.Subject, msg.Body,
	))

	dialer := &net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))

	var client *smtp.Client
	enc := strings.ToLower(cfg.Encryption)
	if enc == "tls" {
		tlsConn := tls.Client(conn, &tls.Config{ServerName: cfg.Host, MinVersion: tls.VersionTLS12})
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			return err
		}
		client, err = smtp.NewClient(tlsConn, cfg.Host)
	} else {
		client, err = smtp.NewClient(conn, cfg.Host)
	}
	if err != nil {
		return err
	}
	defer client.Close()

	if enc == "starttls" {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(&tls.Config{ServerName: cfg.Host, MinVersion: tls.VersionTLS12}); err != nil {
				return err
			}
		}
	}
	if cfg.Username != "" {
		auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
		if err := client.Auth(auth); err != nil {
			return err
		}
	}
	if err := client.Mail(msg.From); err != nil {
		return err
	}
	for _, to := range msg.To {
		if err := client.Rcpt(to); err != nil {
			return err
		}
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(payload); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return client.Quit()
}

func SendResend(ctx context.Context, apiKey string, msg Message) error {
	if apiKey == "" {
		return fmt.Errorf("resend api key required")
	}
	from := msg.From
	if msg.FromName != "" {
		from = fmt.Sprintf("%s <%s>", msg.FromName, msg.From)
	}
	body, _ := json.Marshal(map[string]any{
		"from":    from,
		"to":      msg.To,
		"subject": msg.Subject,
		"text":    msg.Body,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	res, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode >= 300 {
		return fmt.Errorf("resend HTTP %d: %s", res.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}
