package proxy

import (
	"testing"

	"github.com/google/uuid"
)

func TestPreferMagicIP(t *testing.T) {
	if got := PreferMagicIP("127.0.0.1", "178.18.243.148"); got != "178.18.243.148" {
		t.Fatalf("got %q", got)
	}
	if got := PreferMagicIP("10.0.0.5", ""); got != "10.0.0.5" {
		t.Fatalf("got %q", got)
	}
	if got := PreferMagicIP("127.0.0.1", ""); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
	if got := PreferMagicIP("2001:db8::1", ""); got != "2001-db8--1" {
		t.Fatalf("ipv6 magic form: got %q", got)
	}
	if !FQDNUsesUnusableMagicIP("n8n-abc.127.0.0.1.sslip.io") {
		t.Fatal("expected unusable")
	}
	if FQDNUsesUnusableMagicIP("n8n-abc.178.18.243.148.sslip.io") {
		t.Fatal("public sslip should be usable")
	}
}

func TestMagicDomainBase(t *testing.T) {
	if got := MagicDomainBase("1.2.3.4", "sslip.io"); got != "1.2.3.4.sslip.io" {
		t.Fatalf("got %q", got)
	}
	if got := MagicDomainBase("1.2.3.4", "nip.io"); got != "1.2.3.4.nip.io" {
		t.Fatalf("got %q", got)
	}
	if got := MagicDomainBase("2001:db8::1", "sslip.io"); got != "2001-db8--1.sslip.io" {
		t.Fatalf("got %q", got)
	}
	if got := MagicDomainBase("127.0.0.1", "sslip.io"); got != "" {
		t.Fatalf("loopback must not produce magic base, got %q", got)
	}
}

func TestGenerateFQDN(t *testing.T) {
	id := uuid.MustParse("a1b2c3d4-e5f6-7890-abcd-ef1234567890")
	got := GenerateFQDN("My App", id, "10.0.0.5", "", "sslip.io")
	want := "my-app-a1b2c3d4.10.0.0.5.sslip.io"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	got = GenerateFQDN("blog", id, "10.0.0.5", "https://apps.example.com", "sslip.io")
	want = "blog-a1b2c3d4.apps.example.com"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestPublicURL(t *testing.T) {
	if got := PublicURL("foo.1.2.3.4.sslip.io"); got != "http://foo.1.2.3.4.sslip.io" {
		t.Fatalf("magic got %q", got)
	}
	if got := PublicURL("app.example.com"); got != "https://app.example.com" {
		t.Fatalf("custom got %q", got)
	}
	if got := PublicURL("127.0.0.1"); got != "http://127.0.0.1" {
		t.Fatalf("localhost got %q", got)
	}
	if got := PublicURL("https://a.example.com,https://b.example.com"); got != "https://a.example.com" {
		t.Fatalf("multi got %q", got)
	}
	if !SslipHTTPSWarning("https://foo.1.2.3.4.sslip.io") {
		t.Fatal("expected sslip https warning")
	}
	if SslipHTTPSWarning("http://foo.1.2.3.4.sslip.io") {
		t.Fatal("http sslip should not warn")
	}
}

func TestWantAutoHTTPS(t *testing.T) {
	if WantAutoHTTPS("app.example.com") != true {
		t.Fatal("custom domain should auto SSL")
	}
	if WantAutoHTTPS("https://app.example.com,https://www.example.com") != true {
		t.Fatal("multi custom should auto SSL")
	}
	if WantAutoHTTPS("foo.1.2.3.4.sslip.io") {
		t.Fatal("magic must not auto SSL")
	}
	if WantAutoHTTPS("http://n8n.10.0.0.1.nip.io") {
		t.Fatal("nip magic must not auto SSL")
	}
	if WantAutoHTTPS("localhost") || WantAutoHTTPS("") {
		t.Fatal("localhost/empty must not auto SSL")
	}
	if got := AutoPublicURL("app.example.com"); got != "https://app.example.com" {
		t.Fatalf("AutoPublicURL custom: %q", got)
	}
	if got := AutoPublicURL("http://app.example.com"); got != "https://app.example.com" {
		t.Fatalf("AutoPublicURL upgrades http custom: %q", got)
	}
	if got := AutoPublicURL("app.example.com:8080"); got != "https://app.example.com:8080" {
		t.Fatalf("AutoPublicURL port: %q", got)
	}
	if got := AutoPublicURL("foo.1.2.3.4.sslip.io"); got != "http://foo.1.2.3.4.sslip.io" {
		t.Fatalf("AutoPublicURL magic: %q", got)
	}
}

func TestNormalizeDomains(t *testing.T) {
	if got := NormalizeDomainEntry("domain.com"); got != "https://domain.com" {
		t.Fatalf("bare custom: %q", got)
	}
	if got := NormalizeDomainEntry("www.domain.com/app"); got != "https://www.domain.com/app" {
		t.Fatalf("bare path: %q", got)
	}
	if got := NormalizeDomainEntry("domain.com:8080"); got != "https://domain.com:8080" {
		t.Fatalf("bare port: %q", got)
	}
	if got := NormalizeDomainEntry("foo.1.2.3.4.sslip.io"); got != "http://foo.1.2.3.4.sslip.io" {
		t.Fatalf("bare magic: %q", got)
	}
	if got := NormalizeDomainEntry("http://domain.com"); got != "http://domain.com" {
		t.Fatalf("explicit http kept: %q", got)
	}
	if got := NormalizeDomainEntry("https://domain.com/"); got != "https://domain.com" {
		t.Fatalf("trim slash: %q", got)
	}
	if got := NormalizeDomains("domain.com, www.domain.com"); got != "https://domain.com,https://www.domain.com" {
		t.Fatalf("multi: %q", got)
	}
}

func TestDomainParsing(t *testing.T) {
	if got := HostFromDomainEntry("https://App.Example.com:8080/path"); got != "app.example.com" {
		t.Fatalf("host got %q", got)
	}
	if got := HostFromDomainEntry("HTTPS://App.Example.com"); got != "app.example.com" {
		t.Fatalf("case host got %q", got)
	}
	if got := PrimaryHost("https://a.example.com,https://www.example.com"); got != "a.example.com" {
		t.Fatalf("primary got %q", got)
	}
	if got := TraefikHostRule([]string{"a.example.com", "www.example.com"}); got != "Host(`a.example.com`) || Host(`www.example.com`)" {
		t.Fatalf("rule got %q", got)
	}
	if got := PrimaryPublicURL("app.example.com,www.example.com"); got != "https://app.example.com" {
		t.Fatalf("url got %q", got)
	}
	if got := PublicURL("HTTPS://App.Example.com"); got != "https://App.Example.com" {
		t.Fatalf("public case got %q", got)
	}
	if !IsMagicDomainHost("HTTPS://foo.1.2.3.4.sslip.io") {
		t.Fatal("expected magic host")
	}
}
