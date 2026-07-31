package deploy

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/goolify/goolify/internal/store"
)

func TestContainerNameForPreviewIsolation(t *testing.T) {
	id := uuid.MustParse("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")
	app := &store.Application{ID: id, Name: "web", FQDN: "prod.example.com"}
	prod := containerNameFor(Request{App: app})
	prev := containerNameFor(Request{App: app, PullRequestID: 42})
	if prod != "goolify-aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee" {
		t.Fatalf("prod name: %s", prod)
	}
	if prev != "goolify-aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee-pr-42" {
		t.Fatalf("preview name: %s", prev)
	}
	if prod == prev {
		t.Fatal("preview must not reuse production container name")
	}
}

func TestProxyLabelArgsReqNeverReusesProdFQDN(t *testing.T) {
	id := uuid.MustParse("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")
	app := &store.Application{ID: id, Name: "web", FQDN: "prod.example.com", PortsExposes: "80", IsForceHTTPS: false}
	p := &Pipeline{}
	req := Request{
		App:           app,
		Server:        &store.Server{ProxyType: "traefik"},
		PullRequestID: 7,
		PreviewFQDN:   "",
	}
	joined := strings.Join(p.proxyLabelArgsReq(req), " ")
	if strings.Contains(joined, "prod.example.com") {
		t.Fatalf("preview labels must not include production host: %s", joined)
	}
	req.PreviewFQDN = "pr-7.prod.example.com"
	joined = strings.Join(p.proxyLabelArgsReq(req), " ")
	if !strings.Contains(joined, "pr-7.prod.example.com") {
		t.Fatalf("expected preview host in labels: %s", joined)
	}
	if !strings.Contains(joined, "web-pr-7") {
		t.Fatalf("expected unique preview router name web-pr-7 in: %s", joined)
	}
}
