package secrets

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
)

// ValidateBaseURL checks a user-supplied Infisical or Vault address.
// Link-local and cloud-metadata targets are rejected. Private and loopback
// addresses stay allowed so a self-hosted manager on the operator's network works.
func ValidateBaseURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("base URL must be an absolute http(s) URL")
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", fmt.Errorf("base URL must use http or https")
	}
	if u.User != nil {
		return "", fmt.Errorf("base URL must not include credentials")
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if host == "" || strings.Contains(host, " ") {
		return "", fmt.Errorf("base URL host is invalid")
	}
	if blockedHostname(host) {
		return "", fmt.Errorf("base URL must not point at a metadata or link-local host")
	}
	if ip := net.ParseIP(host); ip != nil {
		if blockedIP(ip) {
			return "", fmt.Errorf("base URL must not point at a link-local or metadata address")
		}
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil || len(ips) == 0 {
			return "", fmt.Errorf("base URL host could not be resolved")
		}
		for _, ip := range ips {
			if blockedIP(ip.IP) {
				return "", fmt.Errorf("base URL must not resolve to a link-local or metadata address")
			}
		}
	}
	u.Fragment = ""
	u.RawQuery = ""
	u.Path = strings.TrimRight(u.Path, "/")
	return strings.TrimRight(u.String(), "/"), nil
}

func blockedHostname(host string) bool {
	switch host {
	case "metadata.google.internal", "metadata.google.com", "metadata":
		return true
	default:
		return false
	}
}

func blockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() {
		return true
	}
	// 169.254.0.0/16 is link-local (cloud metadata). IsLinkLocalUnicast covers it for IPv4.
	return false
}
