package notify

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// URLPolicy controls whether outbound notification URLs may target private networks.
type URLPolicy struct {
	AllowLocalhost bool
	AllowedHosts   []string // hostnames, IPs, or CIDRs
}

func ParseAllowedHosts(raw string) []string {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == ' '
	})
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func (p URLPolicy) ValidateHTTPURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return fmt.Errorf("invalid url")
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("url scheme must be http or https")
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("invalid url host")
	}
	if isBlockedMetadataHost(host) {
		return fmt.Errorf("url host is not allowed")
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		if ip := net.ParseIP(host); ip != nil {
			return p.checkIP(ip, host)
		}
		return fmt.Errorf("url host could not be resolved")
	}
	for _, ip := range ips {
		if err := p.checkIP(ip, host); err != nil {
			return err
		}
	}
	return nil
}

func (p URLPolicy) checkIP(ip net.IP, host string) error {
	if ip.IsLoopback() {
		if p.AllowLocalhost || p.hostAllowed(host) {
			return nil
		}
		return fmt.Errorf("localhost destinations are blocked")
	}
	if ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		if p.hostAllowed(host) || p.ipAllowed(ip) {
			return nil
		}
		return fmt.Errorf("private network destinations are blocked")
	}
	return nil
}

func (p URLPolicy) hostAllowed(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	for _, a := range p.AllowedHosts {
		a = strings.ToLower(strings.TrimSpace(a))
		if a == "" {
			continue
		}
		if a == host {
			return true
		}
	}
	return false
}

func (p URLPolicy) ipAllowed(ip net.IP) bool {
	for _, a := range p.AllowedHosts {
		a = strings.TrimSpace(a)
		if strings.Contains(a, "/") {
			_, network, err := net.ParseCIDR(a)
			if err == nil && network.Contains(ip) {
				return true
			}
			continue
		}
		if allow := net.ParseIP(a); allow != nil && allow.Equal(ip) {
			return true
		}
	}
	return false
}

func isBlockedMetadataHost(host string) bool {
	h := strings.ToLower(host)
	return h == "metadata.google.internal" || h == "metadata" || h == "169.254.169.254"
}

func safeHTTPClient(policy URLPolicy) *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, err
			}
			var last error
			for _, ipa := range ips {
				if err := policy.checkIP(ipa.IP, host); err != nil {
					last = err
					continue
				}
				c, err := dialer.DialContext(ctx, network, net.JoinHostPort(ipa.IP.String(), port))
				if err == nil {
					return c, nil
				}
				last = err
			}
			if last == nil {
				last = fmt.Errorf("no allowed addresses for %s", host)
			}
			return nil, last
		},
		TLSHandshakeTimeout: 10 * time.Second,
	}
	return &http.Client{Timeout: 15 * time.Second, Transport: transport}
}
