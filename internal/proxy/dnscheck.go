package proxy

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

// Well-known Cloudflare anycast ranges (proxy mode). Matching these counts as OK
// for DNS validation when the user orange-clouds the record (Coolify parity).
var cloudflareIPv4Nets []*net.IPNet

func init() {
	cidrs := []string{
		"173.245.48.0/20", "103.21.244.0/22", "103.22.200.0/22", "103.31.4.0/22",
		"141.101.64.0/18", "108.162.192.0/18", "190.93.240.0/20", "188.114.96.0/20",
		"197.234.240.0/22", "198.41.128.0/17", "162.158.0.0/15", "104.16.0.0/13",
		"104.24.0.0/14", "172.64.0.0/13", "131.0.72.0/22",
	}
	for _, c := range cidrs {
		_, n, err := net.ParseCIDR(c)
		if err == nil {
			cloudflareIPv4Nets = append(cloudflareIPv4Nets, n)
		}
	}
}

func isCloudflareIPv4(ip net.IP) bool {
	ip4 := ip.To4()
	if ip4 == nil {
		return false
	}
	for _, n := range cloudflareIPv4Nets {
		if n.Contains(ip4) {
			return true
		}
	}
	return false
}

// DNSCheckResult is the outcome of validating a hostname's A records.
type DNSCheckResult struct {
	Host           string   `json:"host"`
	ExpectedIP     string   `json:"expected_ip"`
	ResolvedIPs    []string `json:"resolved_ips"`
	Matched        bool     `json:"matched"`
	Cloudflare     bool     `json:"cloudflare"`
	Resolvers      []string `json:"resolvers"`
	Error          string   `json:"error,omitempty"`
	SkipValidation bool     `json:"skip_validation,omitempty"`
}

// ParseDNSResolvers splits instance custom_dns_servers (default 1.1.1.1).
func ParseDNSResolvers(custom string) []string {
	var out []string
	for _, part := range strings.Split(custom, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if net.ParseIP(part) == nil {
			continue
		}
		out = append(out, part)
	}
	if len(out) == 0 {
		return []string{"1.1.1.1"}
	}
	return out
}

// CheckDomainDNS looks up A records for host and compares to expectedIP (server public IP).
// Magic free domains are skipped (always matched). Cloudflare proxy IPs count as matched.
func CheckDomainDNS(ctx context.Context, host, expectedIP string, resolvers []string) DNSCheckResult {
	host = HostFromDomainEntry(host)
	res := DNSCheckResult{
		Host:       host,
		ExpectedIP: strings.TrimSpace(expectedIP),
		Resolvers:  ParseDNSResolvers(strings.Join(resolvers, ",")),
	}
	if host == "" {
		res.Error = "empty host"
		return res
	}
	if IsMagicDomainHost(host) || host == "localhost" || host == "127.0.0.1" {
		res.Matched = true
		res.SkipValidation = true
		res.ResolvedIPs = []string{host}
		return res
	}
	resolvers = res.Resolvers
	ips, err := lookupAWithResolvers(ctx, host, resolvers)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	expect := net.ParseIP(res.ExpectedIP)
	for _, ip := range ips {
		res.ResolvedIPs = append(res.ResolvedIPs, ip.String())
		if expect != nil && ip.Equal(expect) {
			res.Matched = true
		}
		if isCloudflareIPv4(ip) {
			res.Cloudflare = true
			res.Matched = true
		}
	}
	if res.ExpectedIP == "" && len(res.ResolvedIPs) > 0 {
		// No server IP to compare — report resolved only.
		res.Error = "server public IP unknown; cannot verify match"
	} else if expect == nil && res.ExpectedIP != "" {
		res.Error = "expected IP is not a valid IPv4 address"
	}
	return res
}

// CheckDomainsDNS validates many hosts concurrently (Coolify-style async DNS).
// Each lookup is still bounded by CheckDomainDNS (~5s); the parent context should
// also have a short overall deadline so a large list cannot stall the API.
func CheckDomainsDNS(ctx context.Context, hosts []string, expectedIP string, resolvers []string) []DNSCheckResult {
	results := make([]DNSCheckResult, len(hosts))
	if len(hosts) == 0 {
		return results
	}
	var wg sync.WaitGroup
	for i, h := range hosts {
		wg.Add(1)
		go func(i int, h string) {
			defer wg.Done()
			results[i] = CheckDomainDNS(ctx, h, expectedIP, resolvers)
		}(i, h)
	}
	wg.Wait()
	return results
}

func lookupAWithResolvers(ctx context.Context, host string, resolvers []string) ([]net.IP, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var lastErr error
	for _, server := range resolvers {
		r := &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				d := net.Dialer{Timeout: 3 * time.Second}
				return d.DialContext(ctx, "udp", net.JoinHostPort(server, "53"))
			},
		}
		ips, err := r.LookupIP(ctx, "ip4", host)
		if err == nil && len(ips) > 0 {
			return ips, nil
		}
		if err != nil {
			lastErr = err
		} else {
			lastErr = fmt.Errorf("no A records")
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no A records for %s", host)
	}
	return nil, lastErr
}
