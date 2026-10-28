// Package traffic turns Traefik JSON access lines into stored request totals.
package traffic

import (
	"encoding/json"
	"hash/fnv"
	"net"
	"strings"
	"time"
)

const (
	maxPathLen  = 180
	maxLines    = 400
	maxLineSize = 8192
)

// Hit is one request after host, path, and client IP have been normalized.
type Hit struct {
	Host   string
	Path   string
	Method string
	Class  string
	Client string
	At     time.Time
}

// Counted is a hit with a weight. Sampled requests stand in for several real ones.
type Counted struct {
	Hit
	N int
}

// ParseLine reads one Traefik JSON access-log line. A line that is not a request returns false.
func ParseLine(line string) (Hit, bool) {
	line = strings.TrimSpace(line)
	if line == "" || len(line) > maxLineSize || !strings.HasPrefix(line, "{") {
		return Hit{}, false
	}
	var raw struct {
		ClientHost       string          `json:"ClientHost"`
		RequestMethod    string          `json:"RequestMethod"`
		RequestPath      string          `json:"RequestPath"`
		RequestHost      string          `json:"RequestHost"`
		DownstreamStatus int             `json:"DownstreamStatus"`
		StartUTC         string          `json:"StartUTC"`
		RequestHeaders   json.RawMessage `json:"RequestHeaders"`
	}
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return Hit{}, false
	}
	host := normalizeHost(raw.RequestHost)
	if host == "" {
		return Hit{}, false
	}
	return Hit{
		Host:   host,
		Path:   normalizePath(raw.RequestPath),
		Method: normalizeMethod(raw.RequestMethod),
		Class:  statusClass(raw.DownstreamStatus),
		Client: cleanIP(raw.ClientHost),
		At:     eventTime(raw.StartUTC),
	}, true
}

// ParseLines keeps at most maxLines valid requests.
func ParseLines(lines []string) []Hit {
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	out := make([]Hit, 0, len(lines))
	for _, line := range lines {
		hit, ok := ParseLine(line)
		if ok {
			out = append(out, hit)
		}
	}
	return out
}

// SplitHosts returns the bare hostnames in a Dockfin domain field.
// The field is comma-separated and each entry may include a scheme, port, or path.
func SplitHosts(raw string) []string {
	seen := map[string]bool{}
	var out []string
	for _, part := range strings.Split(raw, ",") {
		host := normalizeHost(part)
		if host == "" || seen[host] {
			continue
		}
		seen[host] = true
		out = append(out, host)
	}
	return out
}

// ApplyIPMode anonymizes or drops the client address. full keeps it, anonymized
// stores a /24 or /48, and off stores nothing. The address is the TCP peer
// Traefik accepted; request headers are not a substitute for that peer.
func ApplyIPMode(client, mode string) string {
	switch mode {
	case "off":
		return ""
	case "full":
		return client
	default:
		return anonymize(client)
	}
}

// Sample keeps every request until a host passes threshold in this batch.
// Later requests for that host are kept one in ten, and each kept request counts as ten.
// A threshold of 0 or less records every request once.
func Sample(hits []Hit, threshold int) []Counted {
	if threshold <= 0 {
		out := make([]Counted, len(hits))
		for i, hit := range hits {
			out[i] = Counted{Hit: hit, N: 1}
		}
		return out
	}
	seen := map[string]int{}
	var out []Counted
	for _, hit := range hits {
		n := seen[hit.Host]
		seen[hit.Host] = n + 1
		if n < threshold {
			out = append(out, Counted{Hit: hit, N: 1})
			continue
		}
		if hashKeep(hit, n) {
			out = append(out, Counted{Hit: hit, N: 10})
		}
	}
	return out
}

func hashKeep(hit Hit, n int) bool {
	h := fnv.New32a()
	_, _ = h.Write([]byte(hit.Host))
	_, _ = h.Write([]byte(hit.Path))
	_, _ = h.Write([]byte(hit.Method))
	sum := h.Sum32() + uint32(n)
	return sum%10 == 0
}

// Bucket is the UTC minute the request is stored in.
func Bucket(now time.Time) time.Time {
	return now.UTC().Truncate(time.Minute)
}

func eventTime(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

func normalizeHost(host string) string {
	host = strings.TrimSpace(strings.ToLower(host))
	host = strings.TrimPrefix(host, "https://")
	host = strings.TrimPrefix(host, "http://")
	if i := strings.IndexAny(host, "/?#"); i >= 0 {
		host = host[:i]
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	if host == "" || len(host) > 253 || strings.ContainsAny(host, " \t\r\n") {
		return ""
	}
	return host
}

func normalizePath(p string) string {
	p = strings.TrimSpace(p)
	if i := strings.IndexAny(p, "?#"); i >= 0 {
		p = p[:i]
	}
	if p == "" {
		p = "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	if len(p) > maxPathLen {
		p = p[:maxPathLen]
	}
	return p
}

func normalizeMethod(m string) string {
	switch strings.ToUpper(strings.TrimSpace(m)) {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS":
		return strings.ToUpper(strings.TrimSpace(m))
	default:
		return "OTHER"
	}
}

func statusClass(code int) string {
	switch {
	case code >= 200 && code < 300:
		return "2xx"
	case code >= 300 && code < 400:
		return "3xx"
	case code >= 400 && code < 500:
		return "4xx"
	case code >= 500 && code < 600:
		return "5xx"
	default:
		return "other"
	}
}

func cleanIP(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if h, _, err := net.SplitHostPort(raw); err == nil {
		raw = h
	}
	raw = strings.Trim(raw, "[]")
	ip := net.ParseIP(raw)
	if ip == nil {
		return ""
	}
	return ip.String()
}

func anonymize(raw string) string {
	ip := net.ParseIP(raw)
	if ip == nil {
		return ""
	}
	if v4 := ip.To4(); v4 != nil {
		return net.IPv4(v4[0], v4[1], v4[2], 0).String() + "/24"
	}
	mask := net.CIDRMask(48, 128)
	return ip.Mask(mask).String() + "/48"
}
