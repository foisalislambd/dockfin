package httpapi

import (
	"testing"

	"github.com/dockfin/dockfin/internal/store"
)

func TestDomainInputMagicStaysHTTP(t *testing.T) {
	port := 8080
	in, msg := domainInputFromBody(serviceDomainBody{
		Host:         "https://App.1.2.3.4.sslip.io/path",
		Scheme:       "https",
		ForceHTTPS:   boolPtr(true),
		Redirect:     "www",
		InternalPort: &port,
		UnitName:     "wordpress",
	}, store.ServiceDomain{})
	if msg != "" {
		t.Fatal(msg)
	}
	if in.Host != "app.1.2.3.4.sslip.io" || in.Scheme != "http" || in.ForceHTTPS || in.Redirect != "both" {
		t.Fatalf("magic input = %#v", in)
	}
	if in.InternalPort == nil || *in.InternalPort != 8080 || in.UnitName != "wordpress" {
		t.Fatalf("port/unit = %#v", in)
	}
}

func TestDomainInputCustomIsHTTPS(t *testing.T) {
	force := false
	in, msg := domainInputFromBody(serviceDomainBody{
		Host:       "APP.Example.com",
		Scheme:     "http",
		ForceHTTPS: &force,
		Redirect:   "non-www",
	}, store.ServiceDomain{ForceHTTPS: true})
	if msg != "" {
		t.Fatal(msg)
	}
	if in.Host != "app.example.com" || in.Scheme != "https" || in.ForceHTTPS || in.Redirect != "non-www" {
		t.Fatalf("custom input = %#v", in)
	}
}

func TestDomainInputRejectsBadPort(t *testing.T) {
	port := 70000
	_, msg := domainInputFromBody(serviceDomainBody{Host: "app.example.com", InternalPort: &port}, store.ServiceDomain{})
	if msg == "" {
		t.Fatal("expected port error")
	}
}

func boolPtr(v bool) *bool { return &v }
