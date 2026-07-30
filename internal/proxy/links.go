package proxy

import (
	"regexp"
	"sort"
	"strings"
)

// ResourceLink is a clickable public URL for a deployed resource.
type ResourceLink struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

var (
	reServiceURLLine   = regexp.MustCompile(`(?m)SERVICE_URL_([A-Z0-9_]+)=([^\s"']+)`)
	reServiceURLQuoted = regexp.MustCompile(`(?m)SERVICE_URL_([A-Z0-9_]+)="([^"]+)"`)
	reTraefikHost      = regexp.MustCompile("Host\\(`([^`]+)`\\)")
)

// CollectLinks builds unique visit URLs from FQDN + prepared compose (SERVICE_URL_* / Traefik Host).
func CollectLinks(fqdn, compose string) []ResourceLink {
	seen := map[string]string{} // url -> label

	add := func(label, raw string) {
		u := normalizeLinkURL(raw)
		if u == "" || u == "http://127.0.0.1" || u == "http://localhost" {
			return
		}
		if prev, ok := seen[u]; ok {
			if prev == "Web" && label != "" && label != "Web" {
				seen[u] = label
			}
			return
		}
		if label == "" {
			label = hostFromURL(u)
		}
		seen[u] = label
	}

	// Prefer named SERVICE_URL_* labels first, then FQDN / Traefik Host.
	for _, m := range reServiceURLLine.FindAllStringSubmatch(compose, -1) {
		if len(m) == 3 {
			add(prettyServiceLabel(m[1]), m[2])
		}
	}
	for _, m := range reServiceURLQuoted.FindAllStringSubmatch(compose, -1) {
		if len(m) == 3 {
			add(prettyServiceLabel(m[1]), m[2])
		}
	}
	for _, part := range strings.Split(fqdn, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		add("Web", part)
	}
	for _, m := range reTraefikHost.FindAllStringSubmatch(compose, -1) {
		if len(m) == 2 {
			add("Web", m[1])
		}
	}

	urls := make([]string, 0, len(seen))
	for u := range seen {
		urls = append(urls, u)
	}
	sort.Strings(urls)
	out := make([]ResourceLink, 0, len(urls))
	for _, u := range urls {
		out = append(out, ResourceLink{Label: seen[u], URL: u})
	}
	// Prefer "Web" / shorter labels first by stable label sort when URLs differ
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Label == out[j].Label {
			return out[i].URL < out[j].URL
		}
		if out[i].Label == "Web" {
			return true
		}
		if out[j].Label == "Web" {
			return false
		}
		return out[i].Label < out[j].Label
	})
	return out
}

func normalizeLinkURL(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.Trim(raw, `"'`)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		return strings.TrimRight(raw, "/")
	}
	// host or host:port
	return "http://" + strings.TrimRight(raw, "/")
}

func hostFromURL(u string) string {
	u = strings.TrimPrefix(strings.TrimPrefix(u, "https://"), "http://")
	if i := strings.IndexByte(u, '/'); i >= 0 {
		u = u[:i]
	}
	return u
}

func prettyServiceLabel(suffix string) string {
	// Strip trailing _PORT (e.g. N8N_5678 → N8N)
	parts := strings.Split(suffix, "_")
	if len(parts) >= 2 {
		last := parts[len(parts)-1]
		allDigits := true
		for _, r := range last {
			if r < '0' || r > '9' {
				allDigits = false
				break
			}
		}
		if allDigits {
			suffix = strings.Join(parts[:len(parts)-1], "_")
		}
	}
	suffix = strings.ReplaceAll(suffix, "_", " ")
	return strings.TrimSpace(suffix)
}
