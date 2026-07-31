package httpapi

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/goolify/goolify/internal/crypto"
	"github.com/goolify/goolify/internal/store"
	"github.com/goolify/goolify/internal/worker"
)

// handleDeployByUUID is Coolify-compatible: GET|POST /api/v1/deploy?uuid=&force=
// Auth: session cookie, API token (deploy/write/root/*), or the resource webhook secret as Bearer.
func (a *API) handleDeployByUUID(w http.ResponseWriter, r *http.Request) {
	raw := strings.TrimSpace(r.URL.Query().Get("uuid"))
	if raw == "" {
		writeError(w, http.StatusBadRequest, "uuid query parameter required")
		return
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid uuid")
		return
	}
	force := r.URL.Query().Get("force") == "true" || r.URL.Query().Get("force") == "1"

	teamID, ok := a.authorizeDeployWebhook(r, id)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if svc, err := a.Store.GetService(r.Context(), teamID, id); err == nil {
		a.triggerServiceDeploy(w, r, svc)
		return
	}
	if app, err := a.Store.GetApplication(r.Context(), teamID, id); err == nil {
		a.triggerApplicationDeploy(w, r, app, force)
		return
	}
	writeError(w, http.StatusNotFound, "resource not found")
}

// handleServiceDeployWebhook triggers redeploy using the service's webhook secret.
// POST|GET /api/v1/webhooks/deploy/services/{serviceID}
func (a *API) handleServiceDeployWebhook(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "serviceID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	token := bearerOrQueryToken(r)
	if token == "" {
		writeError(w, http.StatusUnauthorized, "missing bearer token")
		return
	}
	teamID, secret, err := a.Store.GetServiceWebhookSecret(r.Context(), id)
	if err != nil || secret == "" {
		// Don't leak whether the service UUID exists.
		writeError(w, http.StatusUnauthorized, "invalid webhook token")
		return
	}
	if !secureTokenEqual(secret, token) {
		writeError(w, http.StatusUnauthorized, "invalid webhook token")
		return
	}
	svc, err := a.Store.GetService(r.Context(), teamID, id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	a.triggerServiceDeploy(w, r, svc)
}

func (a *API) authorizeDeployWebhook(r *http.Request, resourceID uuid.UUID) (uuid.UUID, bool) {
	token := sessionToken(r)
	if token == "" {
		return uuid.Nil, false
	}

	if sess, err := a.Store.GetSession(r.Context(), token); err == nil {
		teamID := sess.CurrentTeamID
		if hdr := r.Header.Get("X-Team-ID"); hdr != "" {
			if id, err := uuid.Parse(hdr); err == nil {
				teamID = &id
			}
		}
		if teamID == nil {
			return uuid.Nil, false
		}
		if _, err := a.Store.UserRoleOnTeam(r.Context(), sess.UserID, *teamID); err != nil {
			return uuid.Nil, false
		}
		return *teamID, true
	}

	if apiTok, _, err := a.Store.GetApiTokenByPlain(r.Context(), token); err == nil {
		if !apiTokenCanDeploy(apiTok.Abilities) {
			return uuid.Nil, false
		}
		return apiTok.TeamID, true
	}

	if teamID, secret, err := a.Store.GetServiceWebhookSecret(r.Context(), resourceID); err == nil && secret != "" {
		if secureTokenEqual(secret, token) {
			return teamID, true
		}
	}

	if secret, err := a.Store.GetWebhookSecret(r.Context(), resourceID); err == nil && secret != "" {
		if secureTokenEqual(secret, token) {
			if app, err := a.Store.GetApplicationByID(r.Context(), resourceID); err == nil {
				return app.TeamID, true
			}
		}
	}

	return uuid.Nil, false
}

func secureTokenEqual(want, got string) bool {
	if want == "" || got == "" {
		return false
	}
	if len(want) != len(got) {
		// Length mismatch: still do a dummy compare to reduce timing signal.
		subtle.ConstantTimeCompare([]byte(want), []byte(want))
		return false
	}
	return subtle.ConstantTimeCompare([]byte(want), []byte(got)) == 1
}

func apiTokenCanDeploy(abilities []string) bool {
	if len(abilities) == 0 {
		return true
	}
	for _, a := range abilities {
		switch strings.ToLower(strings.TrimSpace(a)) {
		case "*", "root", "write", "deploy":
			return true
		}
	}
	return false
}

func bearerOrQueryToken(r *http.Request) string {
	if t := sessionToken(r); t != "" {
		return t
	}
	return strings.TrimSpace(r.URL.Query().Get("token"))
}

func (a *API) triggerServiceDeploy(w http.ResponseWriter, r *http.Request, svc *store.Service) {
	ctx := context.WithValue(r.Context(), ctxTeamID, svc.TeamID)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("serviceID", svc.ID.String())
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
	r2 := r.WithContext(ctx)
	if r2.URL.Query().Get("stream") != "1" {
		r2.Header.Set("Accept", "application/json")
	}
	a.handleDeployService(w, r2)
}

func (a *API) triggerApplicationDeploy(w http.ResponseWriter, r *http.Request, app *store.Application, force bool) {
	teamID := app.TeamID
	if app.DestinationID == nil {
		writeError(w, http.StatusBadRequest, "application has no destination")
		return
	}
	dest, err := a.Store.GetDestination(r.Context(), teamID, *app.DestinationID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	serverID := &dest.ServerID
	limit, _ := a.Store.GetDeploymentQueueLimit(r.Context(), dest.ServerID)
	active, _ := a.Store.CountActiveDeployments(r.Context(), dest.ServerID)
	if active >= limit {
		writeError(w, http.StatusTooManyRequests, "server deployment queue limit reached")
		return
	}
	dep, err := a.Store.CreateDeployment(r.Context(), teamID, app.ID, serverID, "", "", force, true, false, 0)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if a.Queue != nil {
		if err := a.Queue.Enqueue(worker.DeployJob{DeploymentID: dep.ID, TeamID: teamID, ForceRebuild: force}); err != nil {
			writeError(w, http.StatusInternalServerError, "enqueue failed")
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":        "queued",
		"deployment_id": dep.ID,
		"resource_type": "application",
		"uuid":          app.ID,
	})
}

func (a *API) handleSetServiceWebhookSecret(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "serviceID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	teamID := currentTeamID(r)
	if _, err := a.Store.GetService(r.Context(), teamID, id); err != nil {
		mapStoreErr(w, err)
		return
	}
	var body struct {
		Secret string `json:"secret"`
	}
	_ = decodeJSON(r, &body)
	if body.Secret == "" {
		body.Secret, _ = crypto.RandomToken(32)
	}
	if err := a.Store.SetServiceWebhookSecret(r.Context(), teamID, id, body.Secret); err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"secret": body.Secret})
}

func (a *API) handleGetServiceWebhookInfo(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "serviceID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	teamID := currentTeamID(r)
	if _, err := a.Store.GetService(r.Context(), teamID, id); err != nil {
		mapStoreErr(w, err)
		return
	}
	has, err := a.Store.ServiceHasWebhookSecret(r.Context(), teamID, id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	base := strings.TrimRight(publicBaseURL(r, a), "/")
	writeJSON(w, http.StatusOK, map[string]any{
		"uuid":               id,
		"has_secret":         has,
		"deploy_url":         fmt.Sprintf("%s/api/v1/deploy?uuid=%s&force=false", base, id),
		"deploy_webhook_url": fmt.Sprintf("%s/api/v1/webhooks/deploy/services/%s", base, id),
	})
}

func publicBaseURL(r *http.Request, a *API) string {
	if a.Cfg != nil {
		if u := strings.TrimSpace(a.Cfg.PublicURL); u != "" {
			return strings.TrimRight(u, "/")
		}
	}
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}
	return scheme + "://" + host
}
