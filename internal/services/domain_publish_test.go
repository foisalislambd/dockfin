package services

import (
	"strings"
	"testing"
	"time"

	"github.com/dockfin/dockfin/internal/store"
	"github.com/google/uuid"
)

func TestPublishedServiceDomainsPrefersCustom(t *testing.T) {
	now := time.Now()
	got := PublishedServiceDomains([]store.ServiceDomain{
		{Host: "app.1.2.3.4.sslip.io", CreatedAt: now},
		{Host: "app.example.com", CreatedAt: now.Add(time.Minute)},
		{Host: "www.example.com", CreatedAt: now.Add(2 * time.Minute)},
	})
	if len(got) != 2 || got[0].Host != "app.example.com" || got[1].Host != "www.example.com" {
		t.Fatalf("published = %#v", got)
	}
}

func TestPublishedServiceDomainsMagicOnly(t *testing.T) {
	got := PublishedServiceDomains([]store.ServiceDomain{
		{Host: "a.1.2.3.4.sslip.io", CreatedAt: time.Now()},
	})
	if len(got) != 1 || got[0].Host != "a.1.2.3.4.sslip.io" {
		t.Fatalf("published = %#v", got)
	}
}

func TestPrepareComposeProxyServiceAndNoIndex(t *testing.T) {
	raw := `
services:
  db:
    image: postgres:16
  wordpress:
    image: wordpress:latest
`
	out, _, err := PrepareCompose(raw, PrepareOpts{
		BaseURL:      "https://app.example.com",
		FQDN:         "app.example.com",
		ProxyService: "wordpress",
		Port:         "80",
		NoIndex:      true,
		ServiceID:    uuid.NewString(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "noindex, nofollow") {
		t.Fatalf("missing noindex:\n%s", out)
	}
	wp := strings.Index(out, "wordpress:")
	label := strings.Index(out, "traefik.enable")
	if wp < 0 || label < wp {
		t.Fatalf("traefik labels not on wordpress:\n%s", out)
	}
	if db := strings.Index(out, "\n  db:"); db > wp && db < label {
		t.Fatalf("labels attached to db:\n%s", out)
	}
}
