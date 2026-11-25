package netsafe

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// ValidateHTTPURL accepts an absolute http(s) URL.
// Link-local and cloud-metadata addresses are always rejected.
// Private and loopback addresses are rejected unless allowPrivate is set,
// which is how a self-hosted registry or MinIO on the operator network is reached.
func ValidateHTTPURL(raw string, allowPrivate bool) (string, error) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.Hostname() == "" {
		return "", fmt.Errorf("invalid url")
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", fmt.Errorf("url scheme must be http or https")
	}
	if u.User != nil {
		return "", fmt.Errorf("url must not include credentials")
	}
	host := strings.ToLower(u.Hostname())
	for strings.HasSuffix(host, ".") {
		host = strings.TrimSuffix(host, ".")
	}
	if host == "" || strings.Contains(host, "..") || strings.ContainsAny(host, " \t\r\n/\\") {
		return "", fmt.Errorf("invalid url host")
	}
	if blockedMetadataHost(host) {
		return "", fmt.Errorf("url host is not allowed")
	}
	if ip := ParseIPLoose(host); ip != nil {
		if err := checkIP(ip, allowPrivate); err != nil {
			return "", err
		}
	}
	port := u.Port()
	u.Fragment = ""
	u.Host = host
	if port != "" {
		u.Host = net.JoinHostPort(host, port)
	}
	u.Path = strings.TrimRight(u.Path, "/")
	return strings.TrimRight(u.String(), "/"), nil
}

func checkIP(ip net.IP, allowPrivate bool) error {
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}
	if ip == nil || ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return fmt.Errorf("url host is not allowed")
	}
	if ip.IsLoopback() || ip.IsPrivate() {
		if allowPrivate {
			return nil
		}
		return fmt.Errorf("private network destinations are blocked")
	}
	return nil
}

// ParseIPLoose parses a host that is an IP address, including IPv4 written with leading zeros.
// net.ParseIP rejects those, and a metadata address hidden that way would otherwise pass.
func ParseIPLoose(host string) net.IP {
	if ip := net.ParseIP(host); ip != nil {
		return ip
	}
	parts := strings.Split(host, ".")
	if len(parts) != 4 {
		return nil
	}
	var octets [4]byte
	for i, part := range parts {
		if part == "" || len(part) > 8 {
			return nil
		}
		n := 0
		for _, r := range part {
			if r < '0' || r > '9' {
				return nil
			}
			n = n*10 + int(r-'0')
			if n > 255 {
				return nil
			}
		}
		octets[i] = byte(n)
	}
	return net.IPv4(octets[0], octets[1], octets[2], octets[3])
}

func blockedMetadataHost(host string) bool {
	if strings.Contains(host, "169.254.169.254") {
		return true
	}
	switch host {
	case "169.254.169.254", "metadata", "metadata.google.internal", "metadata.google.com", "metadata.goog", "metadata.internal":
		return true
	default:
		return false
	}
}
