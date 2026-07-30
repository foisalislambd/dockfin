package proxy

import (
	"fmt"
	"net"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

var nonHost = regexp.MustCompile(`[^a-z0-9-]+`)

// MagicDomainBase returns the free DNS base for a server IP.
// provider is "sslip.io" (default) or "nip.io".
// Loopback / unspecified IPs are rejected — browsers would hit the client machine.
func MagicDomainBase(ip, provider string) string {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider != "nip.io" {
		provider = "sslip.io"
	}
	if IsUnusableMagicIP(ip) {
		return ""
	}
	hostIP := normalizeIPForMagic(ip)
	if hostIP == "" {
		return ""
	}
	return hostIP + "." + provider
}

// IsUnusableMagicIP reports IPs that must not be used in sslip.io / nip.io domains.
func IsUnusableMagicIP(ip string) bool {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return true
	}
	parsed := net.ParseIP(strings.Trim(ip, "[]"))
	if parsed == nil {
		return true
	}
	return parsed.IsLoopback() || parsed.IsUnspecified() || parsed.IsLinkLocalUnicast()
}

// PreferMagicIP picks a publicly reachable IP for free domains.
// Prefer explicit publicIP; fall back to SSH IP when it is not loopback.
func PreferMagicIP(sshIP, publicIP string) string {
	for _, candidate := range []string{publicIP, sshIP} {
		// Check usability on the raw IP — normalizeIPForMagic turns IPv6 into
		// dashed sslip form which net.ParseIP would reject.
		if IsUnusableMagicIP(candidate) {
			continue
		}
		if n := normalizeIPForMagic(candidate); n != "" {
			return n
		}
	}
	return ""
}

// FQDNUsesUnusableMagicIP is true when fqdn embeds loopback (e.g. *.127.0.0.1.sslip.io).
func FQDNUsesUnusableMagicIP(fqdn string) bool {
	fqdn = strings.ToLower(strings.TrimSpace(fqdn))
	fqdn = strings.TrimPrefix(strings.TrimPrefix(fqdn, "https://"), "http://")
	if i := strings.IndexByte(fqdn, '/'); i >= 0 {
		fqdn = fqdn[:i]
	}
	if strings.Contains(fqdn, ".127.0.0.1.") ||
		strings.Contains(fqdn, ".0.0.0.0.") ||
		strings.Contains(fqdn, ".localhost.") ||
		strings.Contains(fqdn, ".--1.") || // IPv6 ::1 → --1 in sslip/nip form
		strings.Contains(fqdn, ".0-0-0-0.") {
		return true
	}
	if strings.HasSuffix(fqdn, ".127.0.0.1") || fqdn == "127.0.0.1" ||
		strings.HasSuffix(fqdn, ".0.0.0.0") || fqdn == "0.0.0.0" ||
		fqdn == "localhost" {
		return true
	}
	return false
}

// WildcardHost extracts the hostname from a wildcard domain setting.
// Accepts "example.com", "http://example.com", "*.example.com".
func WildcardHost(wildcard string) string {
	w := strings.TrimSpace(wildcard)
	w = strings.TrimPrefix(w, "https://")
	w = strings.TrimPrefix(w, "http://")
	w = strings.TrimSuffix(w, "/")
	w = strings.TrimPrefix(w, "*.")
	if i := strings.IndexByte(w, '/'); i >= 0 {
		w = w[:i]
	}
	return strings.ToLower(w)
}

// GenerateFQDN builds a Coolify-style free hostname.
// Prefer server wildcard_domain; otherwise {ip}.sslip.io / nip.io.
// Example: myapp-a1b2c3d4.1.2.3.4.sslip.io
func GenerateFQDN(resourceName string, resourceID uuid.UUID, serverIP, wildcardDomain, magicProvider string) string {
	slug := sanitizeSlug(resourceName)
	short := strings.ReplaceAll(resourceID.String(), "-", "")
	if len(short) > 8 {
		short = short[:8]
	}
	prefix := slug
	if prefix != "" {
		prefix = prefix + "-" + short
	} else {
		prefix = short
	}

	if host := WildcardHost(wildcardDomain); host != "" {
		return prefix + "." + host
	}
	base := MagicDomainBase(serverIP, magicProvider)
	if base == "" {
		return ""
	}
	return prefix + "." + base
}

// IsMagicDomainHost reports sslip.io / nip.io free-DNS hostnames (Coolify magic domains).
func IsMagicDomainHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	host = strings.TrimPrefix(strings.TrimPrefix(host, "https://"), "http://")
	if i := strings.IndexByte(host, '/'); i >= 0 {
		host = host[:i]
	}
	if i := strings.IndexByte(host, ':'); i >= 0 {
		host = host[:i]
	}
	return strings.HasSuffix(host, ".sslip.io") || host == "sslip.io" ||
		strings.HasSuffix(host, ".nip.io") || host == "nip.io"
}

// PublicURL returns a browser URL for an FQDN — Coolify-compatible.
// Magic domains (sslip.io / nip.io) and localhost stay http:// (Coolify sslip() is always http;
// https+sslip is discouraged because Let's Encrypt rate-limits those public domains).
// Custom hostnames default to https://.
func PublicURL(fqdn string) string {
	fqdn = strings.TrimSpace(fqdn)
	if fqdn == "" {
		return "http://127.0.0.1"
	}
	if strings.HasPrefix(fqdn, "http://") || strings.HasPrefix(fqdn, "https://") {
		return strings.TrimRight(fqdn, "/")
	}
	host := fqdn
	if i := strings.IndexByte(host, '/'); i >= 0 {
		host = host[:i]
	}
	hostLower := strings.ToLower(host)
	if hostLower == "127.0.0.1" || hostLower == "localhost" ||
		strings.HasPrefix(hostLower, "127.0.0.1:") || strings.HasPrefix(hostLower, "localhost:") ||
		IsMagicDomainHost(hostLower) {
		return "http://" + fqdn
	}
	return "https://" + fqdn
}

// SslipHTTPSWarning is true when a domain list uses https with sslip (Coolify warning.sslipdomain).
func SslipHTTPSWarning(domains string) bool {
	for _, part := range strings.Split(domains, ",") {
		part = strings.ToLower(strings.TrimSpace(part))
		if part == "" {
			continue
		}
		if strings.Contains(part, "https") && strings.Contains(part, "sslip") {
			return true
		}
		// Bare host stored as FQDN but linked as https
		if IsMagicDomainHost(part) && strings.HasPrefix(PublicURL(part), "https://") {
			return true
		}
	}
	return false
}

func normalizeIPForMagic(ip string) string {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return ""
	}
	// Strip brackets from IPv6 literals
	ip = strings.TrimPrefix(ip, "[")
	ip = strings.TrimSuffix(ip, "]")
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return ""
	}
	if v4 := parsed.To4(); v4 != nil {
		return v4.String()
	}
	// sslip/nip use dashes for IPv6
	return strings.ReplaceAll(parsed.String(), ":", "-")
}

func sanitizeSlug(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = nonHost.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	if len(s) > 32 {
		s = s[:32]
		s = strings.Trim(s, "-")
	}
	return s
}

// TraefikComposeLabels returns a map suitable for compose `labels:` on a service.
func TraefikComposeLabels(routerName, fqdn, port, dockerNetwork string) map[string]string {
	router := sanitize(routerName)
	out := map[string]string{
		"traefik.enable": "true",
		fmt.Sprintf("traefik.http.routers.%s.rule", router):                             fmt.Sprintf("Host(`%s`)", fqdn),
		fmt.Sprintf("traefik.http.routers.%s.entrypoints", router):                      "http",
		fmt.Sprintf("traefik.http.services.%s.loadbalancer.server.port", router):        port,
	}
	if dockerNetwork != "" {
		out["traefik.docker.network"] = dockerNetwork
	}
	return out
}
