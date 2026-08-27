package httpapi

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/dockfin/dockfin/internal/proxy"
	"github.com/dockfin/dockfin/internal/services"
	"github.com/dockfin/dockfin/internal/store"
)

// syncResourceCoolifyEnv mirrors Coolify: after prepare/load, push generated SERVICE_*
// values into the resource Environment Variables UI (URL+FQDN pair + secrets).
func (a *API) syncResourceCoolifyEnv(ctx context.Context, teamID uuid.UUID, resourceType string, resourceID uuid.UUID, raw, prepared string, fullEnv map[string]string) {
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
		_, _ = a.Store.UpsertEnvVar(ctx, teamID, resourceType, resourceID, store.UpsertEnvVarInput{
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
	vars, err := a.Store.ListEnvVars(ctx, teamID, resourceType, resourceID, false)
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

// syncServiceCoolifyEnv is the service-scoped alias used by one-click create.
func (a *API) syncServiceCoolifyEnv(ctx context.Context, teamID, serviceID uuid.UUID, raw, prepared string, fullEnv map[string]string) {
	a.syncResourceCoolifyEnv(ctx, teamID, "service", serviceID, raw, prepared, fullEnv)
}

// rewriteResourceDomainEnv updates SERVICE_URL_* / SERVICE_FQDN_* pairs to match
// the resource domains field (Coolify: changing Domains rewrites magic env).
func (a *API) rewriteResourceDomainEnv(ctx context.Context, teamID uuid.UUID, resourceType string, resourceID uuid.UUID, domains string) {
	baseURL := proxy.AutoPublicURL(domains)
	host := proxy.PrimaryHost(domains)
	if host == "" {
		baseURL = ""
	}
	vars, err := a.Store.ListEnvVars(ctx, teamID, resourceType, resourceID, true)
	if err != nil {
		return
	}
	for _, v := range vars {
		switch {
		case strings.HasPrefix(v.Key, "SERVICE_URL_"):
			if baseURL == "" {
				continue
			}
			_, _ = a.Store.UpsertEnvVar(ctx, teamID, resourceType, resourceID, store.UpsertEnvVarInput{
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
			_, _ = a.Store.UpsertEnvVar(ctx, teamID, resourceType, resourceID, store.UpsertEnvVarInput{
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

func (a *API) rewriteServiceDomainEnv(ctx context.Context, teamID, serviceID uuid.UUID, domains string) {
	a.rewriteResourceDomainEnv(ctx, teamID, "service", serviceID, domains)
}

// syncResourceComposeEnvRefs creates Coolify-style UI env vars from ${VAR} / :- / :?
// references in compose environment and build.args. Existing values are preserved.
func (a *API) syncResourceComposeEnvRefs(ctx context.Context, teamID uuid.UUID, resourceType string, resourceID uuid.UUID, raw string) {
	refs := services.ComposeEnvForUI(raw)
	if len(refs) == 0 {
		return
	}
	for _, ref := range refs {
		_, _ = a.Store.UpsertEnvVar(ctx, teamID, resourceType, resourceID, store.UpsertEnvVarInput{
			Key:       ref.Key,
			Value:     ref.Value,
			Runtime:   true,
			Buildtime: true,
			Literal:   true,
			Comment:   ref.Comment,
			KeepValue: true, // firstOrCreate semantics
		})
	}
}
