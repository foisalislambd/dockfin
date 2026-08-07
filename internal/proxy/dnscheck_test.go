package proxy

import (
	"context"
	"net"
	"testing"
)

func TestParseDNSResolvers(t *testing.T) {
	got := ParseDNSResolvers("1.1.1.1, 8.8.8.8, junk")
	if len(got) != 2 || got[0] != "1.1.1.1" || got[1] != "8.8.8.8" {
		t.Fatalf("got %#v", got)
	}
	if got := ParseDNSResolvers(""); len(got) != 1 || got[0] != "1.1.1.1" {
		t.Fatalf("default: %#v", got)
	}
}

func TestCheckDomainDNSMagicSkipped(t *testing.T) {
	r := CheckDomainDNS(t.Context(), "app.1.2.3.4.sslip.io", "9.9.9.9", nil)
	if !r.Matched || !r.SkipValidation {
		t.Fatalf("magic should skip: %+v", r)
	}
	r = CheckDomainDNS(t.Context(), "http://x.10.0.0.1.nip.io", "9.9.9.9", nil)
	if !r.Matched || !r.SkipValidation {
		t.Fatalf("nip should skip: %+v", r)
	}
}

func TestIsCloudflareIPv4(t *testing.T) {
	if !isCloudflareIPv4(net.ParseIP("104.16.0.1")) {
		t.Fatal("expected cloudflare")
	}
	if isCloudflareIPv4(net.ParseIP("1.2.3.4")) {
		t.Fatal("not cloudflare")
	}
}

func TestPreferPublicIPv4(t *testing.T) {
	if got := PreferPublicIPv4("127.0.0.1", "178.18.243.148"); got != "178.18.243.148" {
		t.Fatalf("got %q", got)
	}
	if got := PreferPublicIPv4("10.0.0.5", ""); got != "10.0.0.5" {
		t.Fatalf("got %q", got)
	}
	// IPv6 must not be used as A-record expected IP (PreferMagicIP would dash it).
	if got := PreferPublicIPv4("", "2001:db8::1"); got != "" {
		t.Fatalf("ipv6 should be empty for A checks, got %q", got)
	}
	if got := PreferPublicIPv4("127.0.0.1", ""); got != "" {
		t.Fatalf("loopback empty, got %q", got)
	}
}

func TestDNSRecordName(t *testing.T) {
	cases := map[string]string{
		"example.com":                 "@",
		"https://example.com":         "@",
		"app.example.com":             "app",
		"a.b.example.com":             "a.b",
		"www.example.com":             "www",
		"http://blog.example.com:443": "blog",
	}
	for host, want := range cases {
		if got := DNSRecordName(host); got != want {
			t.Fatalf("%s: got %q want %q", host, got, want)
		}
	}
}

func TestCheckDomainDNSMatchByIP(t *testing.T) {
	// Live lookup against a non-Cloudflare host — soft-skip if DNS blocked.
	host := "one.one.one.one"
	r := CheckDomainDNS(context.Background(), host, "9.9.9.9", []string{"1.1.1.1"})
	if r.Error != "" && len(r.ResolvedIPs) == 0 {
		t.Skipf("dns unavailable: %s", r.Error)
	}
	if r.Cloudflare {
		t.Skip("resolver returned unexpected Cloudflare anycast for control host")
	}
	if r.Matched {
		t.Fatalf("wrong expected IP should not match: %+v", r)
	}
	if len(r.ResolvedIPs) == 0 {
		t.Fatalf("expected resolved A records for %s", host)
	}
	ok := CheckDomainDNS(context.Background(), host, r.ResolvedIPs[0], []string{"1.1.1.1"})
	if !ok.Matched || ok.Cloudflare {
		t.Fatalf("correct IP should match without CF shortcut: %+v", ok)
	}
}
