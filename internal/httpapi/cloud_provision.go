package httpapi

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/dockfin/dockfin/internal/cloud"
)

// handleProvisionServer creates a VPS at the cloud provider behind the given
// token and registers it as a Dockfin server once it has a public IP.
func (a *API) handleProvisionServer(w http.ResponseWriter, r *http.Request) {
	var body struct {
		CloudTokenID      string `json:"cloud_token_id"`
		Name              string `json:"name"`
		Region            string `json:"region"`
		Size              string `json:"size"`
		Image             string `json:"image"`
		PrivateKeyID      string `json:"private_key_id"`
		CloudInitScriptID string `json:"cloud_init_script_id"`
		ProxyType         string `json:"proxy_type"`
		Description       string `json:"description"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	// Route may carry the token in the path (/cloud-tokens/{tokenID}/provision).
	if v := chi.URLParam(r, "tokenID"); v != "" {
		body.CloudTokenID = v
	}
	tokenID, err := uuid.Parse(strings.TrimSpace(body.CloudTokenID))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid cloud_token_id")
		return
	}
	keyID, err := uuid.Parse(strings.TrimSpace(body.PrivateKeyID))
	if err != nil {
		writeError(w, http.StatusBadRequest, "private_key_id is required")
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		writeError(w, http.StatusBadRequest, "name required")
		return
	}
	proxyType := strings.TrimSpace(body.ProxyType)
	if proxyType == "" {
		proxyType = "traefik"
	}
	switch proxyType {
	case "traefik", "caddy", "none":
	default:
		writeError(w, http.StatusBadRequest, "proxy_type must be traefik, caddy, or none")
		return
	}

	ctx := r.Context()
	teamID := currentTeamID(r)
	tok, err := a.Store.GetCloudProviderToken(ctx, teamID, tokenID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	enc, err := a.Store.GetCloudProviderTokenMaterial(ctx, teamID, tokenID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	plainToken, err := a.Store.Box.DecryptString(enc)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	key, err := a.Store.GetPrivateKey(ctx, teamID, keyID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}

	userData := ""
	if s := strings.TrimSpace(body.CloudInitScriptID); s != "" {
		scriptID, err := uuid.Parse(s)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid cloud_init_script_id")
			return
		}
		_, scriptEnc, err := a.Store.GetCloudInitScript(ctx, teamID, scriptID)
		if err != nil {
			mapStoreErr(w, err)
			return
		}
		userData, err = a.Store.Box.DecryptString(scriptEnc)
		if err != nil {
			mapStoreErr(w, err)
			return
		}
	}

	res, err := cloud.Provision(ctx, cloud.ProvisionRequest{
		Provider:  tok.Provider,
		Token:     plainToken,
		Name:      name,
		Region:    strings.TrimSpace(body.Region),
		Size:      strings.TrimSpace(body.Size),
		Image:     strings.TrimSpace(body.Image),
		PublicKey: key.PublicKey,
		UserData:  userData,
	})
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	srv, err := a.Store.CreateServer(ctx, teamID, &keyID, name, body.Description, res.IP, "root", 22, proxyType)
	if err != nil {
		// The instance exists — surface its identifiers so it can be registered manually.
		writeJSON(w, http.StatusConflict, map[string]any{
			"error": "instance created but registering the server failed: " + err.Error(),
			"cloud": res,
		})
		return
	}
	_ = a.Store.SetServerPublicIP(ctx, teamID, srv.ID, res.IP)
	writeJSON(w, http.StatusCreated, map[string]any{"server": srv, "cloud": res})
}

// handleCloudProviderDefaults returns the provider-specific region/size/image
// defaults used when the provision request leaves those fields blank.
func (a *API) handleCloudProviderDefaults(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "tokenID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	tok, err := a.Store.GetCloudProviderToken(r.Context(), currentTeamID(r), id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	region, size, image := cloud.Defaults(tok.Provider)
	writeJSON(w, http.StatusOK, map[string]string{
		"provider": tok.Provider,
		"region":   region,
		"size":     size,
		"image":    image,
	})
}
