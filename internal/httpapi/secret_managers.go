package httpapi

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"github.com/dockfin/dockfin/internal/envvars"
	"github.com/dockfin/dockfin/internal/secrets"
	"github.com/dockfin/dockfin/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (a *API) handleListIntegrationTokens(w http.ResponseWriter, r *http.Request) {
	list, err := a.Store.ListIntegrationTokens(r.Context(), currentTeamID(r))
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if list == nil {
		list = []store.IntegrationToken{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"integration_tokens": list})
}

func (a *API) handleCreateIntegrationToken(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Provider string           `json:"provider"`
		Name     string           `json:"name"`
		Token    string           `json:"token"`
		Metadata secrets.Metadata `json:"metadata"`
		Validate *bool            `json:"validate"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	provider := strings.ToLower(strings.TrimSpace(body.Provider))
	name := strings.TrimSpace(body.Name)
	if name == "" || strings.TrimSpace(body.Token) == "" {
		writeError(w, http.StatusBadRequest, "name and token are required")
		return
	}
	meta, err := secrets.NormalizeMeta(provider, body.Metadata)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	doValidate := true
	if body.Validate != nil {
		doValidate = *body.Validate
	}
	if doValidate {
		if err := secrets.Validate(r.Context(), provider, body.Token, meta); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	raw, err := json.Marshal(meta)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid metadata")
		return
	}
	enc, err := a.Store.Box.EncryptString(body.Token)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	tok, err := a.Store.CreateIntegrationToken(r.Context(), currentTeamID(r), provider, name, enc, raw)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, tok)
}

func (a *API) handleUpdateIntegrationToken(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "tokenID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body struct {
		Name     string           `json:"name"`
		Token    string           `json:"token"`
		Metadata secrets.Metadata `json:"metadata"`
	}
	if err := decodeJSON(r, &body); err != nil || strings.TrimSpace(body.Name) == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	existing, err := a.Store.GetIntegrationToken(r.Context(), currentTeamID(r), id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	meta, err := secrets.NormalizeMeta(existing.Provider, body.Metadata)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	raw, err := json.Marshal(meta)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid metadata")
		return
	}
	var encPtr *string
	token := strings.TrimSpace(body.Token)
	if token != "" {
		if err := secrets.Validate(r.Context(), existing.Provider, token, meta); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		enc, err := a.Store.Box.EncryptString(token)
		if err != nil {
			mapStoreErr(w, err)
			return
		}
		encPtr = &enc
	} else if existing.Provider != "doppler" {
		// Re-validate stored credentials when only metadata changes.
		_, stored, _, err := a.Store.IntegrationTokenSecret(r.Context(), currentTeamID(r), id)
		if err != nil {
			mapStoreErr(w, err)
			return
		}
		if err := secrets.Validate(r.Context(), existing.Provider, stored, meta); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	tok, err := a.Store.UpdateIntegrationToken(r.Context(), currentTeamID(r), id, strings.TrimSpace(body.Name), encPtr, raw)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tok)
}

func (a *API) handleDeleteIntegrationToken(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "tokenID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := a.Store.DeleteIntegrationToken(r.Context(), currentTeamID(r), id); err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (a *API) handleValidateIntegrationToken(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "tokenID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	provider, token, raw, err := a.Store.IntegrationTokenSecret(r.Context(), currentTeamID(r), id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	var meta secrets.Metadata
	_ = json.Unmarshal(raw, &meta)
	if err := secrets.Validate(r.Context(), provider, token, meta); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *API) handleIntegrationTokenKeys(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "tokenID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body struct {
		Settings secrets.Settings `json:"settings"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	provider, token, raw, err := a.Store.IntegrationTokenSecret(r.Context(), currentTeamID(r), id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	settings, err := secrets.NormalizeSettings(provider, body.Settings)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var meta secrets.Metadata
	_ = json.Unmarshal(raw, &meta)
	values, err := secrets.Fetch(r.Context(), provider, token, meta, settings)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"keys": secretKeyNames(values)})
}

func (a *API) handleGetSecretManagerLink(w http.ResponseWriter, r *http.Request) {
	resourceType, resourceID, ok := a.secretManagerResource(w, r)
	if !ok {
		return
	}
	link, err := a.Store.GetSecretManagerLink(r.Context(), currentTeamID(r), resourceType, resourceID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, link)
}

func (a *API) handlePutSecretManagerLink(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ResourceType       string           `json:"resource_type"`
		ResourceID         string           `json:"resource_id"`
		IntegrationTokenID string           `json:"integration_token_id"`
		Settings           secrets.Settings `json:"settings"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	resourceID, err := uuid.Parse(body.ResourceID)
	tokenID, tokErr := uuid.Parse(body.IntegrationTokenID)
	if err != nil || tokErr != nil || body.ResourceType == "" {
		writeError(w, http.StatusBadRequest, "resource_type, resource_id, and integration_token_id are required")
		return
	}
	teamID := currentTeamID(r)
	if err := a.assertEnvVarResource(r, teamID, body.ResourceType, resourceID); err != nil {
		mapStoreErr(w, err)
		return
	}
	tok, err := a.Store.GetIntegrationToken(r.Context(), teamID, tokenID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	settings, err := secrets.NormalizeSettings(tok.Provider, body.Settings)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	raw, err := json.Marshal(settings)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid settings")
		return
	}
	link, err := a.Store.UpsertSecretManagerLink(r.Context(), teamID, body.ResourceType, resourceID, tokenID, raw)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, link)
}

func (a *API) handleDeleteSecretManagerLink(w http.ResponseWriter, r *http.Request) {
	resourceType, resourceID, ok := a.secretManagerResource(w, r)
	if !ok {
		return
	}
	if err := a.Store.DeleteSecretManagerLink(r.Context(), currentTeamID(r), resourceType, resourceID); err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (a *API) handleImportSecretManager(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ResourceType string `json:"resource_type"`
		ResourceID   string `json:"resource_id"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	resourceID, err := uuid.Parse(body.ResourceID)
	if err != nil || body.ResourceType == "" {
		writeError(w, http.StatusBadRequest, "resource_type and resource_id are required")
		return
	}
	teamID := currentTeamID(r)
	if err := a.assertEnvVarResource(r, teamID, body.ResourceType, resourceID); err != nil {
		mapStoreErr(w, err)
		return
	}
	mat, err := a.Store.SecretManagerMaterial(r.Context(), teamID, body.ResourceType, resourceID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	var meta secrets.Metadata
	var settings secrets.Settings
	_ = json.Unmarshal(mat.Metadata, &meta)
	_ = json.Unmarshal(mat.Settings, &settings)
	values, err := secrets.Fetch(r.Context(), mat.Provider, mat.Token, meta, settings)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	existing, err := a.Store.ListEnvVars(r.Context(), teamID, body.ResourceType, resourceID, false)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	have := map[string]bool{}
	for _, v := range existing {
		have[v.Key] = true
	}
	imported := []string{}
	skipped := []string{}
	already := []string{}
	names := secretKeyNames(values)
	for _, key := range names {
		if !envvars.ValidEnvKey(key) {
			skipped = append(skipped, key)
			continue
		}
		if have[key] {
			already = append(already, key)
			continue
		}
		if _, err := a.Store.UpsertEnvVar(r.Context(), teamID, body.ResourceType, resourceID, store.UpsertEnvVarInput{
			Key:       key,
			Value:     "{{vault." + key + "}}",
			Runtime:   true,
			Buildtime: true,
			Comment:   "Remote secret",
		}); err != nil {
			mapStoreErr(w, err)
			return
		}
		imported = append(imported, key)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"imported": imported,
		"skipped":  skipped,
		"existing": already,
	})
}

func (a *API) secretManagerResource(w http.ResponseWriter, r *http.Request) (string, uuid.UUID, bool) {
	resourceType := r.URL.Query().Get("resource_type")
	resourceID, err := uuid.Parse(r.URL.Query().Get("resource_id"))
	if err != nil || resourceType == "" {
		writeError(w, http.StatusBadRequest, "resource_type and resource_id are required")
		return "", uuid.Nil, false
	}
	if err := a.assertEnvVarResource(r, currentTeamID(r), resourceType, resourceID); err != nil {
		mapStoreErr(w, err)
		return "", uuid.Nil, false
	}
	return resourceType, resourceID, true
}

func secretKeyNames(values map[string]string) []string {
	names := make([]string, 0, len(values))
	for key := range values {
		names = append(names, key)
	}
	sort.Strings(names)
	return names
}
