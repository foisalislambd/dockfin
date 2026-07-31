package httpapi

import (
	"context"
	"net/url"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/goolify/goolify/internal/proxy"
	"github.com/goolify/goolify/internal/services"
)

// syncServiceCoolifyEnv mirrors Coolify: after prepare, push generated SERVICE_*
// values into the resource Environment Variables UI (URL+FQDN pair + secrets).
func (a *API) syncServiceCoolifyEnv(ctx context.Context, teamID, serviceID uuid.UUID, raw, prepared string, fullEnv map[string]string) {
	ui := services.CoolifyEnvForUI(raw, fullEnv)
	if len(ui) == 0 {
		// Prepared compose may already hold KEY=value after persist.
		ui = services.CoolifyEnvForUI(raw, services.ExtractMagicEnv(prepared))
	}
	if len(ui) == 0 {
		return
	}
	keep := map[string]bool{}
	for key, val := range ui {
		keep[key] = true
		_, _ = a.Store.UpsertEnvVar(ctx, teamID, "service", serviceID, key, val, true, false, true, "")
	}
	// Remove leftover companion domain keys from earlier mistaken syncs.
	vars, err := a.Store.ListEnvVars(ctx, teamID, "service", serviceID, false)
	if err != nil {
		return
	}
	for _, v := range vars {
		if !strings.HasPrefix(v.Key, "SERVICE_URL_") && !strings.HasPrefix(v.Key, "SERVICE_FQDN_") {
			continue
		}
		if keep[v.Key] {
			continue
		}
		_ = a.Store.DeleteEnvVar(ctx, teamID, v.ID)
	}
}

func (a *API) loadServiceMagicEnv(ctx context.Context, teamID, serviceID uuid.UUID) map[string]string {
	out := map[string]string{}
	vars, err := a.Store.ListEnvVars(ctx, teamID, "service", serviceID, true)
	if err != nil {
		return out
	}
	for _, v := range vars {
		if strings.HasPrefix(v.Key, "SERVICE_") && strings.TrimSpace(v.Value) != "" {
			out[v.Key] = strings.TrimSpace(v.Value)
		}
	}
	return out
}

// preferURLFromMagicEnv picks SERVICE_URL_* (Coolify pair) as public base URL.
// Skips unusable magic hosts (e.g. *.127.0.0.1.sslip.io) so a repaired FQDN
// cannot be overwritten by a stale env value.
func preferURLFromMagicEnv(env map[string]string) (baseURL, fqdn string) {
	keys := make([]string, 0)
	for k := range env {
		if strings.HasPrefix(k, "SERVICE_URL_") && env[k] != "" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		u, err := url.Parse(env[k])
		if err != nil || u.Host == "" {
			continue
		}
		if proxy.FQDNUsesUnusableMagicIP(u.Host) {
			continue
		}
		return strings.TrimRight(env[k], "/"), u.Host
	}
	fqKeys := make([]string, 0)
	for k, v := range env {
		if strings.HasPrefix(k, "SERVICE_FQDN_") && v != "" && !strings.Contains(v, "://") {
			fqKeys = append(fqKeys, k)
		}
	}
	sort.Strings(fqKeys)
	for _, k := range fqKeys {
		host := env[k]
		if proxy.FQDNUsesUnusableMagicIP(host) {
			continue
		}
		return "http://" + host, host
	}
	return "", ""
}
