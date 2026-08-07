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
	host = HostFromDomainEntry(host)
	return strings.HasSuffix(host, ".sslip.io") || host == "sslip.io" ||
		strings.HasSuffix(host, ".nip.io") || host == "nip.io"
}

// SplitDomainEntries splits a Coolify-style domains field (comma-separated).
func SplitDomainEntries(domains string) []string {
	var out []string
	for _, part := range strings.Split(domains, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}

// HostFromDomainEntry extracts the bare hostname from one Coolify domain entry.
// Accepts "example.com", "https://example.com", "https://example.com:8080/path".
func HostFromDomainEntry(entry string) string {
	entry = strings.TrimSpace(entry)
	if entry == "" {
		return ""
	}
	lower := strings.ToLower(entry)
	switch {
	case strings.HasPrefix(lower, "https://"):
		entry = entry[len("https://"):]
	case strings.HasPrefix(lower, "http://"):
		entry = entry[len("http://"):]
	}
	if i := strings.IndexByte(entry, '/'); i >= 0 {
		entry = entry[:i]
	}
	if i := strings.IndexByte(entry, ':'); i >= 0 {
		// Strip :port (not IPv6 — Coolify domain entries are hostnames).
		entry = entry[:i]
	}
	return strings.ToLower(strings.TrimSpace(entry))
}

// HostsFromDomainList returns unique bare hostnames from a domains field.
func HostsFromDomainList(domains string) []string {
	seen := map[string]bool{}
	var out []string
	for _, entry := range SplitDomainEntries(domains) {
		h := HostFromDomainEntry(entry)
		if h == "" || seen[h] {
			continue
		}
		seen[h] = true
		out = append(out, h)
	}
	return out
}

// PrimaryHost is the first hostname in a domains list (Coolify primary FQDN).
func PrimaryHost(domains string) string {
	hosts := HostsFromDomainList(domains)
	if len(hosts) == 0 {
		return ""
	}
	return hosts[0]
}

// PrimaryPublicURL returns the browser URL for the first domain entry.
func PrimaryPublicURL(domains string) string {
	entries := SplitDomainEntries(domains)
	if len(entries) == 0 {
		return PublicURL("")
	}
	return PublicURL(entries[0])
}

// TraefikHostRule builds Host(`a`) or Host(`a`) || Host(`b`) for multi-domain.
func TraefikHostRule(hosts []string) string {
	var parts []string
	for _, h := range hosts {
		h = HostFromDomainEntry(h)
		if h == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("Host(`%s`)", h))
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " || ")
}

// NormalizeDomainEntry ensures a single domain entry has an explicit scheme.
// Bare custom hosts become https://…; magic/localhost become http://….
// Entries that already include http:// or https:// are left intact (path/port kept).
// Bare host:port and host/path are preserved (e.g. example.com:8080 → https://example.com:8080).
func NormalizeDomainEntry(entry string) string {
	entry = strings.TrimSpace(entry)
	if entry == "" {
		return ""
	}
	lower := strings.ToLower(entry)
	if strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "http://") {
		return strings.TrimRight(entry, "/")
	}
	host := HostFromDomainEntry(entry)
	if host == "" {
		return entry
	}
	rest := strings.TrimRight(entry, "/")
	if host == "localhost" || host == "127.0.0.1" || IsMagicDomainHost(host) {
		return "http://" + rest
	}
	return "https://" + rest
}

// NormalizeDomains applies NormalizeDomainEntry to a comma-separated domains field.
func NormalizeDomains(domains string) string {
	entries := SplitDomainEntries(domains)
	if len(entries) == 0 {
		return ""
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if n := NormalizeDomainEntry(e); n != "" {
			out = append(out, n)
		}
	}
	return strings.Join(out, ",")
}

// WantAutoHTTPS is true when domains should get Traefik TLS + Let's Encrypt
// (user custom hosts). Magic free domains (sslip.io / nip.io) and localhost never
// get auto SSL — Let's Encrypt rate-limits those public suffixes.
func WantAutoHTTPS(domains string) bool {
	hosts := HostsFromDomainList(domains)
	if len(hosts) == 0 {
		return false
	}
	for _, h := range hosts {
		if h == "" || h == "localhost" || h == "127.0.0.1" {
			return false
		}
		if IsMagicDomainHost(h) {
			return false
		}
	}
	return true
}

// AutoPublicURL returns the browser base URL with automatic HTTPS for custom
// domains (Let's Encrypt via Traefik). Magic domains stay http://.
// Port and path from the first entry are preserved (example.com:8080/app).
func AutoPublicURL(domains string) string {
	entries := SplitDomainEntries(domains)
	if len(entries) == 0 {
		return PublicURL("")
	}
	first := entries[0]
	if !WantAutoHTTPS(domains) {
		return PublicURL(first)
	}
	raw := first
	lower := strings.ToLower(raw)
	rest := raw
	switch {
	case strings.HasPrefix(lower, "https://"):
		rest = raw[len("https://"):]
	case strings.HasPrefix(lower, "http://"):
		rest = raw[len("http://"):]
	}
	return "https://" + strings.TrimRight(rest, "/")
}

// PublicURL returns a browser URL for an FQDN — Coolify-compatible.
// Magic domains (sslip.io / nip.io) and localhost stay http:// (Coolify sslip() is always http;
// https+sslip is discouraged because Let's Encrypt rate-limits those public domains).
// Custom hostnames default to https://.
// Comma-separated lists use the first entry only.
func PublicURL(fqdn string) string {
	fqdn = strings.TrimSpace(fqdn)
	if fqdn == "" {
		return "http://127.0.0.1"
	}
	if i := strings.IndexByte(fqdn, ','); i >= 0 {
		fqdn = strings.TrimSpace(fqdn[:i])
	}
	lower := strings.ToLower(fqdn)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		// Normalize scheme casing; keep path/port as typed after scheme.
		rest := fqdn
		scheme := "http"
		if strings.HasPrefix(lower, "https://") {
			scheme = "https"
			rest = fqdn[len("https://"):]
		} else {
			rest = fqdn[len("http://"):]
		}
		return scheme + "://" + strings.TrimRight(rest, "/")
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
	rule := TraefikHostRule(HostsFromDomainList(fqdn))
	if rule == "" {
		if h := HostFromDomainEntry(fqdn); h != "" {
			rule = fmt.Sprintf("Host(`%s`)", h)
		}
	}
	out := map[string]string{
		"traefik.enable": "true",
		fmt.Sprintf("traefik.http.routers.%s.rule", router):                      rule,
		fmt.Sprintf("traefik.http.routers.%s.entrypoints", router):               "http",
		fmt.Sprintf("traefik.http.services.%s.loadbalancer.server.port", router): port,
	}
	if dockerNetwork != "" {
		out["traefik.docker.network"] = dockerNetwork
	}
	return out
}
