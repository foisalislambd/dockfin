package mailer

import (
	"bufio"
	"context"
	"encoding/base64"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

type smtpScript struct {
	auth       bool
	startTLS   bool
	rejectAuth bool
	rejectRcpt bool
	user       string
	pass       string
	data       string
}

func serveSMTP(t *testing.T, script *smtpScript) (host string, port int) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	var mu sync.Mutex
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
				br := bufio.NewReader(conn)
				write := func(s string) {
					_, _ = io.WriteString(conn, s+"\r\n")
				}
				write("220 localhost ESMTP")
				for {
					line, err := br.ReadString('\n')
					if err != nil {
						return
					}
					line = strings.TrimRight(line, "\r\n")
					upper := strings.ToUpper(line)
					switch {
					case strings.HasPrefix(upper, "EHLO") || strings.HasPrefix(upper, "HELO"):
						write("250-localhost")
						if script.auth {
							write("250-AUTH PLAIN")
						}
						if script.startTLS {
							write("250-STARTTLS")
						}
						write("250 OK")
					case strings.HasPrefix(upper, "AUTH "):
						fields := strings.Fields(line)
						raw, err := base64.StdEncoding.DecodeString(fields[len(fields)-1])
						if err != nil {
							write("501 bad")
							return
						}
						parts := strings.Split(string(raw), "\x00")
						if len(parts) != 3 || parts[1] != script.user || parts[2] != script.pass || script.rejectAuth {
							write("535 denied")
							return
						}
						write("235 ok")
					case strings.HasPrefix(upper, "MAIL "):
						write("250 ok")
					case strings.HasPrefix(upper, "RCPT "):
						if script.rejectRcpt {
							write("550 denied")
							return
						}
						write("250 ok")
					case upper == "DATA":
						write("354 end")
						var b strings.Builder
						for {
							l, err := br.ReadString('\n')
							if err != nil {
								return
							}
							if strings.TrimRight(l, "\r\n") == "." {
								break
							}
							b.WriteString(l)
						}
						mu.Lock()
						script.data = b.String()
						mu.Unlock()
						write("250 ok")
					case upper == "QUIT":
						write("221 bye")
						return
					default:
						write("502 unsupported")
						return
					}
				}
			}()
		}
	}()
	addr := ln.Addr().(*net.TCPAddr)
	return addr.IP.String(), addr.Port
}

func TestSendSMTPBuildsMessage(t *testing.T) {
	script := &smtpScript{auth: true, user: "mailer", pass: `p@ss'word`}
	host, port := serveSMTP(t, script)
	err := SendSMTP(context.Background(), SMTPConfig{
		Host: host, Port: port, Username: "mailer", Password: `p@ss'word`, TimeoutSec: 5,
	}, Message{
		FromName: "Dockfin Ops",
		From:     "ops@example.com",
		To:       []string{"a@example.com", "b@example.com"},
		Subject:  "Backup finished",
		Body:     "dump ok\nline 2",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"From: Dockfin Ops <ops@example.com>\r\n",
		"To: a@example.com, b@example.com\r\n",
		"Subject: Backup finished\r\n",
		"Content-Type: text/plain; charset=UTF-8\r\n",
		"\r\n\r\ndump ok\r\nline 2",
	} {
		if !strings.Contains(script.data, want) {
			t.Fatalf("missing %q in\n%s", want, script.data)
		}
	}
}

func TestSendSMTPPlainWithoutName(t *testing.T) {
	script := &smtpScript{}
	host, port := serveSMTP(t, script)
	err := SendSMTP(context.Background(), SMTPConfig{Host: host, Port: port, Encryption: "none", TimeoutSec: 5}, Message{
		From: "ops@example.com", To: []string{"a@example.com"}, Subject: "Hi", Body: "body",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(script.data, "From: ops@example.com\r\n") {
		t.Fatalf("%s", script.data)
	}
}

func TestSendSMTPRejectsIncomplete(t *testing.T) {
	msg := Message{From: "a@b.c", To: []string{"d@e.f"}, Subject: "s", Body: "b"}
	if err := SendSMTP(context.Background(), SMTPConfig{Port: 25}, msg); err == nil {
		t.Fatal("expected host")
	}
	if err := SendSMTP(context.Background(), SMTPConfig{Host: "127.0.0.1"}, msg); err == nil {
		t.Fatal("expected port")
	}
	if err := SendSMTP(context.Background(), SMTPConfig{Host: "127.0.0.1", Port: 25}, Message{To: []string{"d@e.f"}}); err == nil {
		t.Fatal("expected from")
	}
	if err := SendSMTP(context.Background(), SMTPConfig{Host: "127.0.0.1", Port: 25}, Message{From: "a@b.c"}); err == nil {
		t.Fatal("expected to")
	}
}

func TestSendSMTPAuthAndRecipientFailures(t *testing.T) {
	script := &smtpScript{auth: true, rejectAuth: true, user: "mailer", pass: "nope"}
	host, port := serveSMTP(t, script)
	err := SendSMTP(context.Background(), SMTPConfig{Host: host, Port: port, Username: "mailer", Password: "wrong", TimeoutSec: 5}, Message{
		From: "a@b.c", To: []string{"d@e.f"},
	})
	if err == nil {
		t.Fatal("expected auth failure")
	}
	script = &smtpScript{rejectRcpt: true}
	host, port = serveSMTP(t, script)
	err = SendSMTP(context.Background(), SMTPConfig{Host: host, Port: port, TimeoutSec: 5}, Message{
		From: "a@b.c", To: []string{"d@e.f"},
	})
	if err == nil {
		t.Fatal("expected recipient failure")
	}
}

func TestSendSMTPTLSRequired(t *testing.T) {
	script := &smtpScript{startTLS: true}
	host, port := serveSMTP(t, script)
	err := SendSMTP(context.Background(), SMTPConfig{Host: host, Port: port, Encryption: "TLS", TimeoutSec: 5}, Message{
		From: "a@b.c", To: []string{"d@e.f"},
	})
	if err == nil {
		t.Fatal("expected TLS handshake failure")
	}
	err = SendSMTP(context.Background(), SMTPConfig{Host: host, Port: port, Encryption: "starttls", TimeoutSec: 5}, Message{
		From: "a@b.c", To: []string{"d@e.f"},
	})
	if err == nil {
		t.Fatal("expected STARTTLS failure when advertised")
	}
}

func TestSendSMTPStartTLSSkippedWhenUnavailable(t *testing.T) {
	script := &smtpScript{}
	host, port := serveSMTP(t, script)
	err := SendSMTP(context.Background(), SMTPConfig{Host: host, Port: port, Encryption: "starttls", TimeoutSec: 5}, Message{
		From: "a@b.c", To: []string{"d@e.f"}, Subject: "s", Body: "b",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(script.data, "Subject: s\r\n") {
		t.Fatalf("%s", script.data)
	}
}

func TestSendSMTPCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := SendSMTP(ctx, SMTPConfig{Host: "127.0.0.1", Port: 1, TimeoutSec: 1}, Message{From: "a@b.c", To: []string{"d@e.f"}})
	if err == nil {
		t.Fatal("expected canceled dial")
	}
}

func TestSendResendRequiresKeyAndHonorsCancel(t *testing.T) {
	if err := SendResend(context.Background(), "", Message{From: "a@b.c", To: []string{"d@e.f"}}); err == nil || !strings.Contains(err.Error(), "api key") {
		t.Fatalf("%v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := SendResend(ctx, "re_test", Message{FromName: "Dockfin", From: "a@b.c", To: []string{"d@e.f"}, Subject: "s", Body: "b"})
	if err == nil {
		t.Fatal("expected canceled request")
	}
}
