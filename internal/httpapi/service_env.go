package httpapi

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/dockfin/dockfin/internal/proxy"
	"github.com/dockfin/dockfin/internal/services"
	"github.com/dockfin/dockfin/internal/store"
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
		preserve := strings.HasPrefix(key, "SERVICE_PASSWORD_") ||
			strings.HasPrefix(key, "SERVICE_BASE64_") ||
			strings.HasPrefix(key, "SERVICE_HEX_") ||
			strings.HasPrefix(key, "SERVICE_USER_")
		bypassLock := strings.HasPrefix(key, "SERVICE_URL_") || strings.HasPrefix(key, "SERVICE_FQDN_")
		_, _ = a.Store.UpsertEnvVar(ctx, teamID, "service", serviceID, store.UpsertEnvVarInput{
			Key:        key,
			Value:      val,
			Runtime:    true,
			Buildtime:  true,
			Literal:    true,
			Comment:    "",
			KeepValue:  preserve,
			BypassLock: bypassLock,
		})
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
	return services.PreferURLFromMagicEnv(env)
}

// rewriteServiceDomainEnv updates SERVICE_URL_* / SERVICE_FQDN_* pairs to match
// the resource domains field (Coolify: changing Domains rewrites magic env).
func (a *API) rewriteServiceDomainEnv(ctx context.Context, teamID, serviceID uuid.UUID, domains string) {
	baseURL := proxy.AutoPublicURL(domains)
	host := proxy.PrimaryHost(domains)
	if host == "" {
		baseURL = ""
	}
	vars, err := a.Store.ListEnvVars(ctx, teamID, "service", serviceID, true)
	if err != nil {
		return
	}
	for _, v := range vars {
		switch {
		case strings.HasPrefix(v.Key, "SERVICE_URL_"):
			if baseURL == "" {
				continue
			}
			_, _ = a.Store.UpsertEnvVar(ctx, teamID, "service", serviceID, store.UpsertEnvVarInput{
				Key:        v.Key,
				Value:      baseURL,
				Runtime:    true,
				Buildtime:  true,
				Literal:    true,
				BypassLock: true,
			})
		case strings.HasPrefix(v.Key, "SERVICE_FQDN_"):
			if host == "" {
				continue
			}
			_, _ = a.Store.UpsertEnvVar(ctx, teamID, "service", serviceID, store.UpsertEnvVarInput{
				Key:        v.Key,
				Value:      host,
				Runtime:    true,
				Buildtime:  true,
				Literal:    true,
				BypassLock: true,
			})
		}
	}
}
