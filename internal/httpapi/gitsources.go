package httpapi

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/goolify/goolify/internal/crypto"
	"github.com/goolify/goolify/internal/git/githubapp"
)

func (a *API) handleListGitSources(w http.ResponseWriter, r *http.Request) {
	list, err := a.Store.ListGitSources(r.Context(), currentTeamID(r))
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"git_sources": list})
}

func (a *API) handleCreateGitSource(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name       string `json:"name"`
		Provider   string `json:"provider"`
		AppID      string `json:"app_id"`
		Slug       string `json:"slug"`
		PrivateKey string `json:"private_key"`
		ClientID   string `json:"client_id"`
		HTMLURL    string `json:"html_url"`
		APIURL     string `json:"api_url"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if body.Name == "" || body.AppID == "" || body.PrivateKey == "" {
		writeError(w, http.StatusBadRequest, "name, app_id, and private_key required")
		return
	}
	if body.Provider == "" {
		body.Provider = "github"
	}
	slug := body.Slug
	if slug == "" {
		slug = body.Name
	}
	pkEnc, err := a.Store.Box.EncryptString(body.PrivateKey)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	gs, err := a.Store.CreateGitSource(r.Context(), currentTeamID(r), body.Provider, slug, body.AppID, body.ClientID, "", pkEnc, "", body.HTMLURL, body.APIURL)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, gs)
}

func (a *API) handleGetGitSource(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "sourceID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	gs, err := a.Store.GetGitSource(r.Context(), currentTeamID(r), id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, gs)
}

func (a *API) handleDeleteGitSource(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "sourceID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := a.Store.DeleteGitSource(r.Context(), currentTeamID(r), id); err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (a *API) handleGitSourceInstallURL(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "sourceID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	teamID := currentTeamID(r)
	gs, err := a.Store.GetGitSource(r.Context(), teamID, id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	sec, err := a.Store.GetGitSourceSecrets(r.Context(), teamID, id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	pk, err := a.Store.Box.DecryptString(sec.PrivateKeyEnc)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "decrypt private key")
		return
	}
	state, err := crypto.RandomToken(24)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	expires := time.Now().UTC().Add(15 * time.Minute)
	if err := a.Store.SaveGitSetupState(r.Context(), state, teamID, id, expires); err != nil {
		mapStoreErr(w, err)
		return
	}
	app := &githubapp.App{
		AppID:         sec.AppID,
		ClientID:      sec.ClientID,
		PrivateKeyPEM: pk,
		HTMLURL:       gs.HTMLURL,
		APIURL:        gs.APIURL,
		Name:          gs.Name,
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"install_url": app.InstallURL(state),
		"state":       state,
	})
}

func (a *API) handleGitHubAppCallback(w http.ResponseWriter, r *http.Request) {
	installationID := r.URL.Query().Get("installation_id")
	state := r.URL.Query().Get("state")
	if installationID == "" || state == "" {
		writeError(w, http.StatusBadRequest, "installation_id and state required")
		return
	}
	teamID, sourceID, err := a.Store.ConsumeGitSetupState(r.Context(), state)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid or expired state")
		return
	}
	if err := a.Store.UpdateGitSourceInstallation(r.Context(), teamID, sourceID, installationID); err != nil {
		mapStoreErr(w, err)
		return
	}
	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "text/html") || r.URL.Query().Get("redirect") != "0" {
		http.Redirect(w, r, "/git-sources/"+sourceID.String()+"?installed=1", http.StatusFound)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"status":          "installed",
		"installation_id": installationID,
		"git_source_id":   sourceID.String(),
	})
}

func (a *API) handleGitSourceRepositories(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "sourceID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	teamID := currentTeamID(r)
	gs, err := a.Store.GetGitSource(r.Context(), teamID, id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if gs.InstallationID == "" {
		writeError(w, http.StatusBadRequest, "git source is not installed yet")
		return
	}
	sec, err := a.Store.GetGitSourceSecrets(r.Context(), teamID, id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	pk, err := a.Store.Box.DecryptString(sec.PrivateKeyEnc)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "decrypt private key")
		return
	}
	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n > 0 {
			page = n
		}
	}
	app := &githubapp.App{
		AppID:         sec.AppID,
		ClientID:      sec.ClientID,
		PrivateKeyPEM: pk,
		HTMLURL:       gs.HTMLURL,
		APIURL:        gs.APIURL,
		Name:          gs.Name,
	}
	repos, err := app.ListRepos(gs.InstallationID, page)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"repositories": repos, "page": page})
}
