package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/goolify/goolify/internal/crypto"
	"github.com/goolify/goolify/internal/git"
)

func (a *API) handleListEnvVars(w http.ResponseWriter, r *http.Request) {
	resourceType := r.URL.Query().Get("resource_type")
	rid, err := uuid.Parse(r.URL.Query().Get("resource_id"))
	if err != nil || resourceType == "" {
		writeError(w, http.StatusBadRequest, "resource_type and resource_id required")
		return
	}
	reveal := r.URL.Query().Get("reveal") == "1"
	vars, err := a.Store.ListEnvVars(r.Context(), currentTeamID(r), resourceType, rid, reveal)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"environment_variables": vars})
}

func (a *API) handleUpsertEnvVar(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ResourceType string `json:"resource_type"`
		ResourceID   string `json:"resource_id"`
		Key          string `json:"key"`
		Value        string `json:"value"`
		IsRuntime    *bool  `json:"is_runtime"`
		IsBuildtime  bool   `json:"is_buildtime"`
		IsLiteral    bool   `json:"is_literal"`
		Comment      string `json:"comment"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	rid, err := uuid.Parse(body.ResourceID)
	if err != nil || body.ResourceType == "" || body.Key == "" {
		writeError(w, http.StatusBadRequest, "resource_type, resource_id, key required")
		return
	}
	runtime := true
	if body.IsRuntime != nil {
		runtime = *body.IsRuntime
	}
	v, err := a.Store.UpsertEnvVar(r.Context(), currentTeamID(r), body.ResourceType, rid, body.Key, body.Value, runtime, body.IsBuildtime, body.IsLiteral, body.Comment)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (a *API) handleDeleteEnvVar(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "varID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := a.Store.DeleteEnvVar(r.Context(), currentTeamID(r), id); err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (a *API) handleCancelDeployment(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "deploymentID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := a.Store.CancelDeployment(r.Context(), currentTeamID(r), id); err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

func (a *API) handleSetWebhookSecret(w http.ResponseWriter, r *http.Request) {
	appID, err := uuid.Parse(chi.URLParam(r, "appID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body struct {
		Secret string `json:"secret"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if body.Secret == "" {
		body.Secret, _ = crypto.RandomToken(24)
	}
	if err := a.Store.SetWebhookSecret(r.Context(), currentTeamID(r), appID, body.Secret); err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"secret": body.Secret})
}

func (a *API) handleListSharedEnv(w http.ResponseWriter, r *http.Request) {
	scope := r.URL.Query().Get("scope_type")
	if scope == "" {
		scope = "team"
	}
	var scopeID *uuid.UUID
	if s := r.URL.Query().Get("scope_id"); s != "" {
		id, err := uuid.Parse(s)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid scope_id")
			return
		}
		scopeID = &id
	}
	vars, err := a.Store.ListSharedEnv(r.Context(), currentTeamID(r), scope, scopeID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"shared_environment_variables": vars})
}

func (a *API) handleUpsertSharedEnv(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ScopeType string `json:"scope_type"`
		ScopeID   string `json:"scope_id"`
		Key       string `json:"key"`
		Value     string `json:"value"`
		IsLiteral bool   `json:"is_literal"`
	}
	if err := decodeJSON(r, &body); err != nil || body.Key == "" {
		writeError(w, http.StatusBadRequest, "key required")
		return
	}
	if body.ScopeType == "" {
		body.ScopeType = "team"
	}
	var scopeID *uuid.UUID
	if body.ScopeID != "" {
		id, err := uuid.Parse(body.ScopeID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid scope_id")
			return
		}
		scopeID = &id
	}
	v, err := a.Store.UpsertSharedEnv(r.Context(), currentTeamID(r), body.ScopeType, scopeID, body.Key, body.Value, body.IsLiteral)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

// verifyWebhookAuth checks signature when a secret is configured.
func (a *API) verifyWebhookAuth(r *http.Request, appID uuid.UUID, body []byte) error {
	secret, err := a.Store.GetWebhookSecret(r.Context(), appID)
	if err != nil || secret == "" {
		return nil // no secret configured = allow (dev mode)
	}
	if sig := r.Header.Get("X-Hub-Signature-256"); sig != "" {
		if !git.VerifyGitHubSignature(secret, body, sig) {
			return errUnauthorizedWebhook
		}
		return nil
	}
	if tok := r.Header.Get("X-Gitlab-Token"); tok != "" {
		if tok != secret {
			return errUnauthorizedWebhook
		}
		return nil
	}
	if q := r.URL.Query().Get("secret"); q != "" {
		if q != secret {
			return errUnauthorizedWebhook
		}
		return nil
	}
	return errUnauthorizedWebhook
}

var errUnauthorizedWebhook = errStr("invalid webhook signature")

type errStr string

func (e errStr) Error() string { return string(e) }
