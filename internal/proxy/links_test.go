package proxy

import "testing"

func TestCollectLinksFromFQDNAndCompose(t *testing.T) {
	compose := `
services:
  wordpress:
    environment:
      - SERVICE_URL_WORDPRESS=http://wp.1.2.3.4.sslip.io
    labels:
      traefik.http.routers.wp.rule: Host(` + "`wp.1.2.3.4.sslip.io`" + `)
  n8n:
    environment:
      - SERVICE_URL_N8N_5678=http://n8n.1.2.3.4.sslip.io
`
	links := CollectLinks("wp.1.2.3.4.sslip.io,extra.example.com", compose)
	if len(links) < 2 {
		t.Fatalf("expected multiple links, got %#v", links)
	}
	urls := map[string]string{}
	for _, l := range links {
		urls[l.URL] = l.Label
	}
	if urls["http://wp.1.2.3.4.sslip.io"] == "" {
		t.Fatalf("missing wp url: %#v", links)
	}
	if urls["https://extra.example.com"] == "" {
		t.Fatalf("missing extra: %#v", links)
	}
	if urls["http://n8n.1.2.3.4.sslip.io"] != "N8N" {
		t.Fatalf("n8n label: %#v", links)
	}
}

func TestCollectLinksSkipsLocalhost(t *testing.T) {
	links := CollectLinks("", "SERVICE_URL_APP=http://127.0.0.1")
	if len(links) != 0 {
		t.Fatalf("got %#v", links)
	}
}
