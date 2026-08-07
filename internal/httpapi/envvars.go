package httpapi

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/dockfin/dockfin/internal/crypto"
	"github.com/dockfin/dockfin/internal/git"
	"github.com/dockfin/dockfin/internal/store"
)

func (a *API) handleListEnvVars(w http.ResponseWriter, r *http.Request) {
	resourceType := r.URL.Query().Get("resource_type")
	rid, err := uuid.Parse(r.URL.Query().Get("resource_id"))
	if err != nil || resourceType == "" {
		writeError(w, http.StatusBadRequest, "resource_type and resource_id required")
		return
	}
	teamID := currentTeamID(r)
	// Coolify: once compose is loaded, Environment Variables auto-fill SERVICE_* keys.
	if resourceType == "application" {
		a.ensureApplicationComposeEnv(r.Context(), teamID, rid)
	}
	reveal := r.URL.Query().Get("reveal") == "1"
	vars, err := a.Store.ListEnvVars(r.Context(), teamID, resourceType, rid, reveal)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if previewQ := r.URL.Query().Get("is_preview"); previewQ == "true" || previewQ == "1" || previewQ == "false" || previewQ == "0" {
		want := previewQ == "true" || previewQ == "1"
		filtered := vars[:0]
		for _, v := range vars {
			if v.IsPreview == want {
				filtered = append(filtered, v)
			}
		}
		vars = filtered
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
		IsBuildtime  *bool  `json:"is_buildtime"`
		IsLiteral    bool   `json:"is_literal"`
		IsMultiline  bool   `json:"is_multiline"`
		IsLocked     bool   `json:"is_locked"`
		IsPreview    bool   `json:"is_preview"`
		IsBuildSecret *bool `json:"is_build_secret"`
		Comment      string `json:"comment"`
		KeepValue    bool   `json:"keep_value"`
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
	teamID := currentTeamID(r)
	if err := a.assertEnvVarResource(r, teamID, body.ResourceType, rid); err != nil {
		mapStoreErr(w, err)
		return
	}
	runtime := true
	if body.IsRuntime != nil {
		runtime = *body.IsRuntime
	}
	buildtime := true
	if body.IsBuildtime != nil {
		buildtime = *body.IsBuildtime
	}
	buildSecret := false
	if body.IsBuildSecret != nil {
		buildSecret = *body.IsBuildSecret
	}
	// Multiline implies literal (Coolify).
	literal := body.IsLiteral || body.IsMultiline
	v, err := a.Store.UpsertEnvVar(r.Context(), teamID, body.ResourceType, rid, store.UpsertEnvVarInput{
		Key:           body.Key,
		Value:         body.Value,
		Runtime:       runtime,
		Buildtime:     buildtime,
		Literal:       literal,
		Multiline:     body.IsMultiline,
		Locked:        body.IsLocked,
		IsPreview:     body.IsPreview,
		IsBuildSecret: buildSecret,
		Comment:       body.Comment,
		KeepValue:     body.KeepValue,
	})
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (a *API) handleLockEnvVar(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "varID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body struct {
		Locked *bool `json:"locked"`
	}
	if err := decodeJSON(r, &body); err != nil || body.Locked == nil {
		writeError(w, http.StatusBadRequest, "locked required")
		return
	}
	v, err := a.Store.SetEnvVarLocked(r.Context(), currentTeamID(r), id, *body.Locked)
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
	reveal := r.URL.Query().Get("reveal") == "1"
	vars, err := a.Store.ListSharedEnv(r.Context(), currentTeamID(r), scope, scopeID, reveal)
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
	teamID := currentTeamID(r)
	if err := a.assertSharedEnvScope(r, teamID, body.ScopeType, scopeID); err != nil {
		mapStoreErr(w, err)
		return
	}
	v, err := a.Store.UpsertSharedEnv(r.Context(), teamID, body.ScopeType, scopeID, body.Key, body.Value, body.IsLiteral)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (a *API) assertSharedEnvScope(r *http.Request, teamID uuid.UUID, scopeType string, scopeID *uuid.UUID) error {
	switch scopeType {
	case "team":
		if scopeID != nil {
			return store.ErrNotFound
		}
		return nil
	case "project":
		if scopeID == nil {
			return store.ErrNotFound
		}
		_, err := a.Store.GetProject(r.Context(), teamID, *scopeID)
		return err
	case "environment":
		if scopeID == nil {
			return store.ErrNotFound
		}
		_, err := a.Store.GetEnvironment(r.Context(), teamID, *scopeID)
		return err
	case "server":
		if scopeID == nil {
			return store.ErrNotFound
		}
		_, err := a.Store.GetServer(r.Context(), teamID, *scopeID)
		return err
	default:
		return store.ErrNotFound
	}
}

func (a *API) assertEnvVarResource(r *http.Request, teamID uuid.UUID, resourceType string, resourceID uuid.UUID) error {
	switch resourceType {
	case "application":
		_, err := a.Store.GetApplication(r.Context(), teamID, resourceID)
		return err
	case "database":
		_, err := a.Store.GetDatabase(r.Context(), teamID, resourceID)
		return err
	case "service":
		_, err := a.Store.GetService(r.Context(), teamID, resourceID)
		return err
	default:
		return store.ErrNotFound
	}
}

// verifyWebhookAuth requires a configured webhook secret except in development,
// where an empty secret is allowed for local testing.
func (a *API) verifyWebhookAuth(r *http.Request, appID uuid.UUID, body []byte) error {
	secret, err := a.Store.GetWebhookSecret(r.Context(), appID)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return errUnauthorizedWebhook
	}
	if secret == "" {
		if a.Cfg != nil && a.Cfg.IsDev() {
			return nil
		}
		return errUnauthorizedWebhook
	}
	if sig := r.Header.Get("X-Hub-Signature-256"); sig != "" {
		if !git.VerifyGitHubSignature(secret, body, sig) {
			return errUnauthorizedWebhook
		}
		return nil
	}
	if sig := r.Header.Get("X-Hub-Signature"); sig != "" {
		if !git.VerifyGitHubSignature(secret, body, sig) {
			return errUnauthorizedWebhook
		}
		return nil
	}
	if tok := r.Header.Get("X-Gitlab-Token"); tok != "" {
		if !secureTokenEqual(secret, tok) {
			return errUnauthorizedWebhook
		}
		return nil
	}
	if q := r.URL.Query().Get("secret"); q != "" {
		// Query secrets leak to logs; keep for compatibility but constant-time compare.
		if !secureTokenEqual(secret, q) {
			return errUnauthorizedWebhook
		}
		return nil
	}
	return errUnauthorizedWebhook
}

var errUnauthorizedWebhook = errStr("invalid webhook signature")

type errStr string

func (e errStr) Error() string { return string(e) }
