package httpapi

import (
	"net/http"
	"strings"
)

// publicBaseURL is the absolute origin used in webhook/deploy links (Coolify-style).
// Prefer Settings → Domain (instance public_url), then DOCKFIN_PUBLIC_URL, then the request host.
func (a *API) publicBaseURL(r *http.Request) string {
	if a != nil && a.Store != nil {
		if st, err := a.Store.GetInstanceSettings(r.Context()); err == nil {
			if u := strings.TrimSpace(st.PublicURL); u != "" {
				return strings.TrimRight(u, "/")
			}
		}
	}
	if a != nil && a.Cfg != nil {
		if u := strings.TrimSpace(a.Cfg.PublicURL); u != "" {
			return strings.TrimRight(u, "/")
		}
	}
	scheme := "http"
	if r != nil && (r.TLS != nil || (forwardedProtoTrusted(r) && r.Header.Get("X-Forwarded-Proto") == "https")) {
		scheme = "https"
	}
	host := ""
	if r != nil {
		host = r.Header.Get("X-Forwarded-Host")
		if host == "" {
			host = r.Host
		}
	}
	if host == "" {
		return "http://127.0.0.1:8000"
	}
	return scheme + "://" + host
}
