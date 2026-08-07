package httpapi

import (
	"context"
	"net"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/dockfin/dockfin/internal/store"
)

const ctxAPIAbilities ctxKey = "api_abilities"

// enforceAPITokenPolicy applies instance API enablement, IP allowlists, and
// token ability checks. Cookie sessions are unrestricted by abilities.
func (a *API) enforceAPITokenPolicy(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, _ := r.Context().Value(ctxSession).(*store.Session)
		if sess == nil || sess.ID != uuid.Nil {
			// Real browser/session auth — abilities do not apply.
			next.ServeHTTP(w, r)
			return
		}
		abilities, _ := r.Context().Value(ctxAPIAbilities).([]string)
		st, err := a.Store.GetInstanceSettings(r.Context())
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "unable to verify API policy")
			return
		}
		if !st.IsAPIEnabled {
			writeError(w, http.StatusForbidden, "API access is disabled")
			return
		}
		if !clientIPAllowed(r, st.AllowedIPs) {
			writeError(w, http.StatusForbidden, "client IP not allowed")
			return
		}
		if !apiTokenAllows(abilities, r.Method, r.URL.Path) {
			writeError(w, http.StatusForbidden, "insufficient API token abilities")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func clientIPAllowed(r *http.Request, allowlist string) bool {
	allowlist = strings.TrimSpace(allowlist)
	if allowlist == "" {
		return true
	}
	ipStr := requestClientIP(r)
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	for _, part := range strings.Split(allowlist, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if part == "0.0.0.0" || part == "::" || part == "*" {
			return true
		}
		if strings.Contains(part, "/") {
			_, network, err := net.ParseCIDR(part)
			if err == nil && network.Contains(ip) {
				return true
			}
			continue
		}
		if allowIP := net.ParseIP(part); allowIP != nil && allowIP.Equal(ip) {
			return true
		}
	}
	return false
}

func requestClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err != nil {
		return strings.TrimSpace(r.RemoteAddr)
	}
	return host
}

func apiTokenAllows(abilities []string, method, path string) bool {
	set := abilitySet(abilities)
	if set["root"] || set["*"] || set["write"] {
		return true
	}
	method = strings.ToUpper(method)
	// MCP speaks JSON-RPC over POST for both read and write tools; gate deploy
	// separately inside the MCP deploy handler.
	if isMCPAPIPath(path) {
		return set["read"] || set["read:sensitive"] || set["deploy"]
	}
	readLike := method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions
	if readLike {
		if isSensitiveAPIPath(path) {
			return set["read:sensitive"]
		}
		return set["read"] || set["read:sensitive"]
	}
	if set["deploy"] && isDeployAPIPath(path) {
		return true
	}
	return false
}

func isMCPAPIPath(path string) bool {
	p := strings.ToLower(path)
	return p == "/api/v1/mcp" || strings.HasPrefix(p, "/api/v1/mcp/")
}

func abilitySet(abilities []string) map[string]bool {
	out := make(map[string]bool, len(abilities))
	for _, a := range abilities {
		a = strings.ToLower(strings.TrimSpace(a))
		if a != "" {
			out[a] = true
		}
	}
	return out
}

func isSensitiveAPIPath(path string) bool {
	p := strings.ToLower(path)
	switch {
	case strings.Contains(p, "/env-vars"),
		strings.Contains(p, "/shared-env-vars"),
		strings.Contains(p, "/private-keys"),
		strings.Contains(p, "/cloud-tokens"),
		strings.Contains(p, "/webhook-secret"),
		strings.Contains(p, "/settings/email"),
		strings.Contains(p, "/instance/backup"):
		return true
	default:
		return false
	}
}

func isDeployAPIPath(path string) bool {
	p := strings.ToLower(path)
	if strings.HasSuffix(p, "/deploy") || strings.Contains(p, "/deploy/") {
		return true
	}
	if strings.HasSuffix(p, "/rollback") {
		return true
	}
	if strings.Contains(p, "/deployments/") && strings.HasSuffix(p, "/cancel") {
		return true
	}
	return false
}

func withAPIAbilities(ctx context.Context, abilities []string) context.Context {
	return context.WithValue(ctx, ctxAPIAbilities, abilities)
}
