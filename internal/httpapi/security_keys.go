package httpapi

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/dockfin/dockfin/internal/store"
	"golang.org/x/crypto/ssh"
)

func (a *API) handleGetKey(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "keyID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	key, err := a.Store.GetPrivateKey(r.Context(), currentTeamID(r), id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, key)
}

func (a *API) handleUpdateKey(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "keyID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := decodeJSON(r, &body); err != nil || body.Name == "" {
		writeError(w, http.StatusBadRequest, "name required")
		return
	}
	key, err := a.Store.UpdatePrivateKey(r.Context(), currentTeamID(r), id, body.Name, body.Description)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, key)
}

func (a *API) handleDeleteKey(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "keyID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := a.Store.DeletePrivateKey(r.Context(), currentTeamID(r), id); err != nil {
		if errors.Is(err, store.ErrConflict) {
			writeError(w, http.StatusConflict, "private key is in use by a server")
			return
		}
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (a *API) handleCleanupUnusedKeys(w http.ResponseWriter, r *http.Request) {
	n, err := a.Store.CleanupUnusedPrivateKeys(r.Context(), currentTeamID(r))
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "deleted": n})
}

func (a *API) handleGenerateKey(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Type        string `json:"type"` // ed25519 | rsa
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := decodeJSONOptional(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	typ := strings.ToLower(strings.TrimSpace(body.Type))
	if typ == "" {
		typ = "ed25519"
	}
	if typ != "ed25519" && typ != "rsa" {
		writeError(w, http.StatusBadRequest, "type must be ed25519 or rsa")
		return
	}
	privPEM, pubAuth, fp, err := generateSSHKey(typ)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		name = fmt.Sprintf("generated-%s-%s", typ, time.Now().UTC().Format("20060102-150405"))
	}
	desc := body.Description
	if desc == "" {
		desc = "Created by Dockfin"
	}
	enc, err := a.Store.Box.EncryptString(privPEM)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	key, err := a.Store.CreatePrivateKey(r.Context(), currentTeamID(r), name, desc, pubAuth, enc, fp)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, key)
}

func generateSSHKey(typ string) (privPEM, pubAuth, fingerprint string, err error) {
	var signer ssh.Signer
	switch typ {
	case "rsa":
		priv, e := rsa.GenerateKey(rand.Reader, 4096)
		if e != nil {
			return "", "", "", e
		}
		signer, err = ssh.NewSignerFromKey(priv)
		if err != nil {
			return "", "", "", err
		}
		block, e := ssh.MarshalPrivateKey(priv, "")
		if e != nil {
			return "", "", "", e
		}
		privPEM = string(pem.EncodeToMemory(block))
	default:
		_, priv, e := ed25519.GenerateKey(rand.Reader)
		if e != nil {
			return "", "", "", e
		}
		signer, err = ssh.NewSignerFromKey(priv)
		if err != nil {
			return "", "", "", err
		}
		block, e := ssh.MarshalPrivateKey(priv, "")
		if e != nil {
			return "", "", "", e
		}
		privPEM = string(pem.EncodeToMemory(block))
	}
	pubAuth = string(ssh.MarshalAuthorizedKey(signer.PublicKey()))
	sum := sha256.Sum256(signer.PublicKey().Marshal())
	fingerprint = "SHA256:" + base64.RawStdEncoding.EncodeToString(sum[:])
	return privPEM, pubAuth, fingerprint, nil
}

func (a *API) handleListCloudTokens(w http.ResponseWriter, r *http.Request) {
	list, err := a.Store.ListCloudProviderTokens(r.Context(), currentTeamID(r))
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if list == nil {
		list = []store.CloudProviderToken{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"cloud_tokens": list})
}

func (a *API) handleCreateCloudToken(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Provider    string `json:"provider"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Token       string `json:"token"`
		Validate    *bool  `json:"validate"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	provider := strings.ToLower(strings.TrimSpace(body.Provider))
	if body.Name == "" || body.Token == "" || (provider != "hetzner" && provider != "digitalocean" && provider != "vultr") {
		writeError(w, http.StatusBadRequest, "provider, name, and token required")
		return
	}
	doValidate := true
	if body.Validate != nil {
		doValidate = *body.Validate
	}
	if doValidate {
		if err := validateCloudProviderToken(provider, body.Token); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	enc, err := a.Store.Box.EncryptString(body.Token)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	tok, err := a.Store.CreateCloudProviderToken(r.Context(), currentTeamID(r), provider, body.Name, body.Description, enc)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, tok)
}

func (a *API) handleGetCloudToken(w http.ResponseWriter, r *http.Request) {
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
	writeJSON(w, http.StatusOK, tok)
}

func (a *API) handleUpdateCloudToken(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "tokenID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Token       string `json:"token"`
	}
	if err := decodeJSON(r, &body); err != nil || body.Name == "" {
		writeError(w, http.StatusBadRequest, "name required")
		return
	}
	var encPtr *string
	if strings.TrimSpace(body.Token) != "" {
		existing, err := a.Store.GetCloudProviderToken(r.Context(), currentTeamID(r), id)
		if err != nil {
			mapStoreErr(w, err)
			return
		}
		if err := validateCloudProviderToken(existing.Provider, body.Token); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		enc, err := a.Store.Box.EncryptString(body.Token)
		if err != nil {
			mapStoreErr(w, err)
			return
		}
		encPtr = &enc
	}
	tok, err := a.Store.UpdateCloudProviderToken(r.Context(), currentTeamID(r), id, body.Name, body.Description, encPtr)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tok)
}

func (a *API) handleDeleteCloudToken(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "tokenID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := a.Store.DeleteCloudProviderToken(r.Context(), currentTeamID(r), id); err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (a *API) handleValidateCloudToken(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "tokenID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	teamID := currentTeamID(r)
	tok, err := a.Store.GetCloudProviderToken(r.Context(), teamID, id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	enc, err := a.Store.GetCloudProviderTokenMaterial(r.Context(), teamID, id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	plain, err := a.Store.Box.DecryptString(enc)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if err := validateCloudProviderToken(tok.Provider, plain); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "valid"})
}

func validateCloudProviderToken(provider, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("token required")
	}
	var url, authHeader, authValue string
	switch provider {
	case "hetzner":
		url = "https://api.hetzner.cloud/v1/servers?per_page=1"
		authHeader = "Authorization"
		authValue = "Bearer " + token
	case "digitalocean":
		url = "https://api.digitalocean.com/v2/account"
		authHeader = "Authorization"
		authValue = "Bearer " + token
	case "vultr":
		url = "https://api.vultr.com/v2/account"
		authHeader = "Authorization"
		authValue = "Bearer " + token
	default:
		return fmt.Errorf("unsupported provider")
	}
	client := &http.Client{Timeout: 12 * time.Second}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set(authHeader, authValue)
	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("could not reach %s API: %v", provider, err)
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 1<<20))
	if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden {
		return fmt.Errorf("invalid %s token", provider)
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("%s API returned HTTP %d", provider, res.StatusCode)
	}
	return nil
}

func (a *API) handleListCloudInitScripts(w http.ResponseWriter, r *http.Request) {
	list, err := a.Store.ListCloudInitScripts(r.Context(), currentTeamID(r))
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if list == nil {
		list = []store.CloudInitScript{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"cloud_init_scripts": list})
}

func (a *API) handleCreateCloudInitScript(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name   string `json:"name"`
		Script string `json:"script"`
	}
	if err := decodeJSON(r, &body); err != nil || body.Name == "" || body.Script == "" {
		writeError(w, http.StatusBadRequest, "name and script required")
		return
	}
	if err := validateCloudInitScript(body.Script); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	enc, err := a.Store.Box.EncryptString(body.Script)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	sc, err := a.Store.CreateCloudInitScript(r.Context(), currentTeamID(r), body.Name, enc)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, sc)
}

func (a *API) handleGetCloudInitScript(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "scriptID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	sc, enc, err := a.Store.GetCloudInitScript(r.Context(), currentTeamID(r), id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	plain, err := a.Store.Box.DecryptString(enc)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	sc.Script = plain
	writeJSON(w, http.StatusOK, sc)
}

func (a *API) handleUpdateCloudInitScript(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "scriptID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body struct {
		Name   string `json:"name"`
		Script string `json:"script"`
	}
	if err := decodeJSON(r, &body); err != nil || body.Name == "" || body.Script == "" {
		writeError(w, http.StatusBadRequest, "name and script required")
		return
	}
	if err := validateCloudInitScript(body.Script); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	enc, err := a.Store.Box.EncryptString(body.Script)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	sc, err := a.Store.UpdateCloudInitScript(r.Context(), currentTeamID(r), id, body.Name, enc)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sc)
}

func (a *API) handleDeleteCloudInitScript(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "scriptID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := a.Store.DeleteCloudInitScript(r.Context(), currentTeamID(r), id); err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func validateCloudInitScript(script string) error {
	s := strings.TrimSpace(script)
	if s == "" {
		return fmt.Errorf("script is empty")
	}
	if strings.HasPrefix(s, "#!") || strings.HasPrefix(s, "#cloud-config") {
		return nil
	}
	// Accept plain YAML-ish cloud-config without the header (Coolify ValidCloudInitYaml).
	if strings.Contains(s, ":") {
		return nil
	}
	return fmt.Errorf("script must start with #! or #cloud-config, or be valid YAML")
}
