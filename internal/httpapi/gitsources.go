package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/dockfin/dockfin/internal/crypto"
	"github.com/dockfin/dockfin/internal/git"
	"github.com/dockfin/dockfin/internal/git/githubapp"
	"github.com/dockfin/dockfin/internal/store"
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
		Name         string `json:"name"`
		Provider     string `json:"provider"`
		Organization string `json:"organization"`
		AppID        string `json:"app_id"`
		Slug         string `json:"slug"`
		PrivateKey   string `json:"private_key"`
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
		WebhookSecret string `json:"webhook_secret"`
		HTMLURL      string `json:"html_url"`
		APIURL       string `json:"api_url"`
		CustomUser   string `json:"custom_user"`
		CustomPort   int    `json:"custom_port"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if strings.TrimSpace(body.Name) == "" {
		writeError(w, http.StatusBadRequest, "name required")
		return
	}
	if body.Provider == "" {
		body.Provider = "github"
	}
	if body.HTMLURL == "" {
		body.HTMLURL = "https://github.com"
	}
	if body.APIURL == "" {
		body.APIURL = githubapp.APIURLFromHTML(body.HTMLURL)
	}
	name := body.Name
	if body.Slug != "" {
		name = body.Slug
	}
	var pkEnc, csEnc, whEnc string
	var err error
	if body.PrivateKey != "" {
		pkEnc, err = a.Store.Box.EncryptString(body.PrivateKey)
		if err != nil {
			mapStoreErr(w, err)
			return
		}
	}
	if body.ClientSecret != "" {
		csEnc, err = a.Store.Box.EncryptString(body.ClientSecret)
		if err != nil {
			mapStoreErr(w, err)
			return
		}
	}
	if body.WebhookSecret != "" {
		whEnc, err = a.Store.Box.EncryptString(body.WebhookSecret)
		if err != nil {
			mapStoreErr(w, err)
			return
		}
	}
	gs, err := a.Store.CreateGitSource(
		r.Context(), currentTeamID(r), body.Provider, name, strings.TrimSpace(body.Organization),
		body.AppID, body.ClientID, csEnc, pkEnc, whEnc, body.HTMLURL, body.APIURL, body.CustomUser, body.CustomPort,
	)
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

func (a *API) handleUpdateGitSource(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "sourceID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body struct {
		Name           *string `json:"name"`
		Organization   *string `json:"organization"`
		AppID          *string `json:"app_id"`
		InstallationID *string `json:"installation_id"`
		ClientID       *string `json:"client_id"`
		ClientSecret   *string `json:"client_secret"`
		WebhookSecret  *string `json:"webhook_secret"`
		PrivateKey     *string `json:"private_key"`
		HTMLURL        *string `json:"html_url"`
		APIURL         *string `json:"api_url"`
		CustomUser     *string `json:"custom_user"`
		CustomPort     *int    `json:"custom_port"`
		IsSystemWide   *bool   `json:"is_system_wide"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	in := store.UpdateGitSourceInput{
		Name: body.Name, Organization: body.Organization, AppID: body.AppID,
		InstallationID: body.InstallationID, ClientID: body.ClientID,
		HTMLURL: body.HTMLURL, APIURL: body.APIURL, CustomUser: body.CustomUser,
		CustomPort: body.CustomPort, IsSystemWide: body.IsSystemWide,
	}
	if body.HTMLURL != nil && (body.APIURL == nil || *body.APIURL == "") {
		derived := githubapp.APIURLFromHTML(*body.HTMLURL)
		in.APIURL = &derived
	}
	if body.ClientSecret != nil && strings.TrimSpace(*body.ClientSecret) != "" {
		enc, err := a.Store.Box.EncryptString(*body.ClientSecret)
		if err != nil {
			mapStoreErr(w, err)
			return
		}
		in.ClientSecretEnc = &enc
	}
	if body.WebhookSecret != nil && strings.TrimSpace(*body.WebhookSecret) != "" {
		enc, err := a.Store.Box.EncryptString(*body.WebhookSecret)
		if err != nil {
			mapStoreErr(w, err)
			return
		}
		in.WebhookSecretEnc = &enc
	}
	if body.PrivateKey != nil && strings.TrimSpace(*body.PrivateKey) != "" {
		enc, err := a.Store.Box.EncryptString(*body.PrivateKey)
		if err != nil {
			mapStoreErr(w, err)
			return
		}
		in.PrivateKeyEnc = &enc
	}
	gs, err := a.Store.UpdateGitSource(r.Context(), currentTeamID(r), id, in)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	// Completing configuration requires App ID + private key (new or already stored).
	if gs.AppID != "" && gs.AppID != "0" && !gs.HasPrivateKey {
		writeError(w, http.StatusBadRequest, "private_key (PEM) is required to finish GitHub App configuration")
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
		if errors.Is(err, store.ErrConflict) {
			writeError(w, http.StatusConflict, "This source is being used by an application. Please delete or reassign those applications first.")
			return
		}
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (a *API) handleGitSourceApps(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "sourceID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	list, err := a.Store.ListAppsUsingGitSource(r.Context(), currentTeamID(r), id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"applications": list})
}

func (a *API) handleGitSourceManifest(w http.ResponseWriter, r *http.Request) {
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
	preview := true
	if v := r.URL.Query().Get("preview"); v == "0" || v == "false" {
		preview = false
	}
	endpoint := strings.TrimSpace(r.URL.Query().Get("endpoint"))
	if endpoint == "" {
		endpoint = a.publicBaseURL(r)
	}
	endpoint = strings.TrimRight(endpoint, "/")

	state, err := crypto.RandomToken(32)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	expires := time.Now().UTC().Add(60 * time.Minute)
	if err := a.Store.SaveGitSetupState(r.Context(), state, teamID, id, expires); err != nil {
		mapStoreErr(w, err)
		return
	}

	app := &githubapp.App{
		Name:         gs.Name,
		HTMLURL:      gs.HTMLURL,
		Organization: gs.Organization,
	}
	manifest := githubapp.BuildManifest(
		gs.Name, endpoint,
		"/github/app/events",
		"/github/app/manifest",
		"/github/app/callback",
		id.String(),
		preview,
	)

	writeJSON(w, http.StatusOK, map[string]any{
		"state":      state,
		"action_url": app.ManifestFormAction(state),
		"manifest":   manifest,
		"endpoint":   endpoint,
	})
}

func (a *API) handleGitHubAppManifestCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	if code == "" || state == "" {
		writeError(w, http.StatusBadRequest, "code and state required")
		return
	}
	teamID, sourceID, err := a.Store.ConsumeGitSetupState(r.Context(), state)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid or expired state")
		return
	}
	gs, err := a.Store.GetGitSource(r.Context(), teamID, sourceID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	apiURL := gs.APIURL
	if apiURL == "" {
		apiURL = githubapp.APIURLFromHTML(gs.HTMLURL)
	}
	conv, err := githubapp.ConvertManifest(apiURL, code)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	csEnc, err := a.Store.Box.EncryptString(conv.ClientSecret)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	pkEnc, err := a.Store.Box.EncryptString(conv.PEM)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	whEnc, err := a.Store.Box.EncryptString(conv.WebhookSecret)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	appID := strconv.FormatInt(conv.ID, 10)
	if err := a.Store.ApplyManifestCredentials(r.Context(), teamID, sourceID, appID, conv.Slug, conv.ClientID, csEnc, pkEnc, whEnc); err != nil {
		mapStoreErr(w, err)
		return
	}
	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "text/html") || r.URL.Query().Get("redirect") != "0" {
		http.Redirect(w, r, "/git-sources/"+sourceID.String()+"?registered=1", http.StatusFound)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"status":        "registered",
		"git_source_id": sourceID.String(),
		"app_id":        appID,
	})
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
	if !gs.Configured {
		writeError(w, http.StatusBadRequest, "finish GitHub App configuration before installing")
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
	// Prefer live slug from GitHub so renaming the display name does not break install URLs.
	if slug, err := app.AppSlug(); err == nil && slug != "" {
		app.Name = slug
		if slug != gs.Name {
			_, _ = a.Store.UpdateGitSource(r.Context(), teamID, id, store.UpdateGitSourceInput{Name: &slug})
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"install_url": app.InstallURL(state),
		"state":       state,
	})
}

func (a *API) handleGitHubAppCallback(w http.ResponseWriter, r *http.Request) {
	installationID := r.URL.Query().Get("installation_id")
	if installationID == "" {
		writeError(w, http.StatusBadRequest, "installation_id required")
		return
	}
	state := r.URL.Query().Get("state")
	sourceParam := r.URL.Query().Get("source_id")

	var teamID, sourceID uuid.UUID
	if state != "" {
		var err error
		teamID, sourceID, err = a.Store.ConsumeGitSetupState(r.Context(), state)
		if err != nil && sourceParam == "" {
			writeError(w, http.StatusBadRequest, "invalid or expired state")
			return
		}
	}
	if sourceID == uuid.Nil {
		id, err := uuid.Parse(sourceParam)
		if err != nil {
			writeError(w, http.StatusBadRequest, "state or source_id required")
			return
		}
		gs, err := a.Store.GetGitSourceByID(r.Context(), id)
		if err != nil {
			mapStoreErr(w, err)
			return
		}
		teamID, sourceID = gs.TeamID, gs.ID
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
	if !gs.Installed {
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
	all := r.URL.Query().Get("all") == "1" || r.URL.Query().Get("all") == "true"
	var repos []map[string]any
	if all {
		repos, err = app.ListAllRepositories(gs.InstallationID)
	} else {
		repos, err = app.ListRepos(gs.InstallationID, page)
	}
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"repositories": repos, "page": page, "count": len(repos)})
}

func (a *API) handleGitSourceBranches(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "sourceID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	owner := chi.URLParam(r, "owner")
	repo := chi.URLParam(r, "repo")
	if owner == "" || repo == "" {
		writeError(w, http.StatusBadRequest, "owner and repo required")
		return
	}
	teamID := currentTeamID(r)
	gs, err := a.Store.GetGitSource(r.Context(), teamID, id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if !gs.Installed {
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
	app := &githubapp.App{
		AppID:         sec.AppID,
		ClientID:      sec.ClientID,
		PrivateKeyPEM: pk,
		HTMLURL:       gs.HTMLURL,
		APIURL:        gs.APIURL,
		Name:          gs.Name,
	}
	branches, err := app.ListBranches(gs.InstallationID, owner, repo)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"branches": branches})
}

// Placeholder so GitHub App webhook URL from the manifest is reachable.
func (a *API) handleGitHubAppEvents(w http.ResponseWriter, r *http.Request) {
	body, err := git.ReadBody(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "read body")
		return
	}
	eventName := strings.ToLower(r.Header.Get("X-GitHub-Event"))
	if eventName == "ping" {
		writeJSON(w, http.StatusOK, map[string]string{"status": "pong"})
		return
	}
	switch eventName {
	case "installation", "installation_repositories", "github_app_authorization":
		w.WriteHeader(http.StatusNoContent)
		return
	case "push", "pull_request":
		// handled below
	default:
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignored", "reason": "unsupported event"})
		return
	}

	appIDHeader := strings.TrimSpace(r.Header.Get("X-GitHub-Hook-Installation-Target-Id"))
	if appIDHeader == "" {
		writeError(w, http.StatusBadRequest, "missing installation target id")
		return
	}
	gs, sec, err := a.Store.GetGitSourceByAppID(r.Context(), appIDHeader)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	whSecret := ""
	if sec.WebhookSecretEnc != "" {
		whSecret, err = a.Store.Box.DecryptString(sec.WebhookSecretEnc)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "decrypt webhook secret")
			return
		}
	}
	sig := r.Header.Get("X-Hub-Signature-256")
	if sig == "" {
		sig = r.Header.Get("X-Hub-Signature")
	}
	if whSecret == "" {
		if a.Cfg == nil || !a.Cfg.IsDev() {
			writeError(w, http.StatusUnauthorized, "webhook secret not configured")
			return
		}
	} else if !git.VerifyGitHubSignature(whSecret, body, sig) {
		writeError(w, http.StatusUnauthorized, "invalid webhook signature")
		return
	}

	event, err := git.ParseWebhook("github", r, body)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	matchBranch := event.Branch
	if event.PRNumber > 0 && event.BaseBranch != "" {
		matchBranch = event.BaseBranch
	}
	apps, err := a.Store.ListApplicationsForGitWebhook(r.Context(), gs.ID, event.RepoFullName, matchBranch)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if len(apps) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "ignored",
			"reason":  "no matching applications",
			"repo":    event.RepoFullName,
			"branch":  matchBranch,
			"results": []any{},
		})
		return
	}

	results := make([]webhookActionResult, 0, len(apps))
	accepted := false
	for i := range apps {
		res := a.processWebhookEvent(r.Context(), &apps[i], event)
		results = append(results, res)
		if res.Status == "success" {
			accepted = true
		}
	}
	status := http.StatusOK
	if accepted {
		status = http.StatusAccepted
	}
	writeJSON(w, status, map[string]any{
		"status":  "ok",
		"results": results,
	})
}
