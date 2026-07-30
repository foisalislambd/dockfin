package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/goolify/goolify/internal/crypto"
	"github.com/goolify/goolify/internal/database"
	"github.com/goolify/goolify/internal/git"
	"github.com/goolify/goolify/internal/store"
	"github.com/goolify/goolify/internal/worker"
)

func (a *API) handleListProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := a.Store.ListProjects(r.Context(), currentTeamID(r))
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"projects": projects})
}

func (a *API) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := decodeJSON(r, &body); err != nil || body.Name == "" {
		writeError(w, http.StatusBadRequest, "name required")
		return
	}
	p, env, err := a.Store.CreateProject(r.Context(), currentTeamID(r), body.Name, body.Description)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"project": p, "environment": env})
}

func (a *API) handleGetProject(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	p, err := a.Store.GetProject(r.Context(), currentTeamID(r), id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (a *API) handleListEnvironments(w http.ResponseWriter, r *http.Request) {
	pid, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	envs, err := a.Store.ListEnvironments(r.Context(), currentTeamID(r), pid)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"environments": envs})
}

func (a *API) handleCreateEnvironment(w http.ResponseWriter, r *http.Request) {
	pid, err := uuid.Parse(chi.URLParam(r, "projectID"))
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
	env, err := a.Store.CreateEnvironment(r.Context(), currentTeamID(r), pid, body.Name, body.Description)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, env)
}

func (a *API) handleListApplications(w http.ResponseWriter, r *http.Request) {
	var envID *uuid.UUID
	if s := r.URL.Query().Get("environment_id"); s != "" {
		id, err := uuid.Parse(s)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid environment_id")
			return
		}
		envID = &id
	}
	apps, err := a.Store.ListApplications(r.Context(), currentTeamID(r), envID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"applications": apps})
}

func (a *API) handleCreateApplication(w http.ResponseWriter, r *http.Request) {
	var body struct {
		EnvironmentID           string `json:"environment_id"`
		DestinationID           string `json:"destination_id"`
		Name                    string `json:"name"`
		Description             string `json:"description"`
		FQDN                    string `json:"fqdn"`
		BuildPack               string `json:"build_pack"`
		GitRepository           string `json:"git_repository"`
		GitBranch               string `json:"git_branch"`
		DockerfileLocation      string `json:"dockerfile_location"`
		DockerComposeLocation   string `json:"docker_compose_location"`
		DockerRegistryImageName string `json:"docker_registry_image_name"`
		DockerRegistryImageTag  string `json:"docker_registry_image_tag"`
		PortsExposes            string `json:"ports_exposes"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	envID, err := uuid.Parse(body.EnvironmentID)
	if err != nil || body.Name == "" {
		writeError(w, http.StatusBadRequest, "environment_id and name required")
		return
	}
	if body.BuildPack == "" {
		body.BuildPack = "dockerfile"
	}
	if body.GitBranch == "" {
		body.GitBranch = "main"
	}
	if body.DockerfileLocation == "" {
		body.DockerfileLocation = "/Dockerfile"
	}
	if body.DockerComposeLocation == "" {
		body.DockerComposeLocation = "/docker-compose.yaml"
	}
	if body.PortsExposes == "" {
		body.PortsExposes = "3000"
	}
	if body.FQDN != "" && !isValidHostnameList(body.FQDN) {
		writeError(w, http.StatusBadRequest, "invalid fqdn")
		return
	}
	app := &store.Application{
		TeamID:                  currentTeamID(r),
		EnvironmentID:           envID,
		Name:                    body.Name,
		Description:             body.Description,
		FQDN:                    body.FQDN,
		BuildPack:               body.BuildPack,
		GitRepository:           body.GitRepository,
		GitBranch:               body.GitBranch,
		DockerfileLocation:      body.DockerfileLocation,
		DockerComposeLocation:   body.DockerComposeLocation,
		DockerRegistryImageName: body.DockerRegistryImageName,
		DockerRegistryImageTag:  body.DockerRegistryImageTag,
		PortsExposes:            body.PortsExposes,
	}
	if body.DestinationID != "" {
		id, err := uuid.Parse(body.DestinationID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid destination_id")
			return
		}
		if _, err := a.Store.GetDestination(r.Context(), currentTeamID(r), id); err != nil {
			mapStoreErr(w, err)
			return
		}
		app.DestinationID = &id
	}
	created, err := a.Store.CreateApplication(r.Context(), app)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (a *API) handleGetApplication(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "appID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	app, err := a.Store.GetApplication(r.Context(), currentTeamID(r), id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, app)
}

func (a *API) handleDeployApplication(w http.ResponseWriter, r *http.Request) {
	appID, err := uuid.Parse(chi.URLParam(r, "appID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body struct {
		ForceRebuild bool   `json:"force_rebuild"`
		CommitSHA    string `json:"commit_sha"`
	}
	_ = decodeJSON(r, &body)
	teamID := currentTeamID(r)
	app, err := a.Store.GetApplication(r.Context(), teamID, appID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if app.DestinationID == nil {
		writeError(w, http.StatusBadRequest, "application has no destination")
		return
	}
	var serverID *uuid.UUID
	dest, err := a.Store.GetDestination(r.Context(), teamID, *app.DestinationID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	serverID = &dest.ServerID
	limit, _ := a.Store.GetDeploymentQueueLimit(r.Context(), dest.ServerID)
	active, _ := a.Store.CountActiveDeployments(r.Context(), dest.ServerID)
	if active >= limit {
		writeError(w, http.StatusTooManyRequests, "server deployment queue limit reached")
		return
	}
	dep, err := a.Store.CreateDeployment(r.Context(), teamID, appID, serverID, body.CommitSHA, "", body.ForceRebuild, false, true)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if err := a.Queue.Enqueue(worker.DeployJob{DeploymentID: dep.ID, TeamID: teamID, ForceRebuild: body.ForceRebuild}); err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, dep)
}

func (a *API) handleListDeployments(w http.ResponseWriter, r *http.Request) {
	appID, err := uuid.Parse(chi.URLParam(r, "appID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	deps, err := a.Store.ListDeployments(r.Context(), currentTeamID(r), appID, 50)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deployments": deps})
}

func (a *API) handleGetDeployment(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "deploymentID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	dep, err := a.Store.GetDeployment(r.Context(), currentTeamID(r), id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dep)
}

func (a *API) handleDeploymentLogStream(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "deploymentID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if _, err := a.Store.GetDeployment(r.Context(), currentTeamID(r), id); err != nil {
		mapStoreErr(w, err)
		return
	}
	a.Hub.SSEHandler(id)(w, r)
}

func (a *API) handleGitWebhook(w http.ResponseWriter, r *http.Request) {
	appID, err := uuid.Parse(chi.URLParam(r, "appID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	body, err := git.ReadBody(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "read body")
		return
	}
	if err := a.verifyWebhookAuth(r, appID, body); err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	provider := r.URL.Query().Get("provider")
	if provider == "" {
		provider = "github"
		if r.Header.Get("X-Gitlab-Event") != "" {
			provider = "gitlab"
		}
	}
	event, err := git.ParseWebhook(provider, r, body)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	app, err := a.Store.GetApplicationByID(r.Context(), appID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	// PR preview deployments
	if event.PRNumber > 0 {
		if !app.IsPreviewEnabled {
			writeJSON(w, http.StatusOK, map[string]string{"status": "ignored", "reason": "preview deployments disabled"})
			return
		}
		fqdn := ""
		if app.FQDN != "" {
			host := strings.TrimSpace(strings.Split(app.FQDN, ",")[0])
			fqdn = fmt.Sprintf("pr-%d.%s", event.PRNumber, host)
		}
		preview, err := a.Store.CreatePreview(r.Context(), app.TeamID, appID, event.PRNumber, event.Message, event.Branch, fqdn)
		if err != nil {
			mapStoreErr(w, err)
			return
		}
		var serverID *uuid.UUID
		if app.DestinationID != nil {
			if dest, err := a.Store.GetDestination(r.Context(), app.TeamID, *app.DestinationID); err == nil {
				serverID = &dest.ServerID
			}
		}
		dep, err := a.Store.CreateDeployment(r.Context(), app.TeamID, appID, serverID, event.Commit, event.Message, false, true, false)
		if err != nil {
			mapStoreErr(w, err)
			return
		}
		if err := a.Queue.Enqueue(worker.DeployJob{DeploymentID: dep.ID, TeamID: app.TeamID}); err != nil {
			writeError(w, http.StatusServiceUnavailable, err.Error())
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]any{
			"deployment_id": dep.ID, "commit": event.Commit, "preview_id": preview.ID, "preview_fqdn": preview.FQDN,
		})
		return
	}
	if app.GitBranch != "" && event.Branch != "" && event.Branch != app.GitBranch {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignored", "reason": "branch mismatch"})
		return
	}
	var serverID *uuid.UUID
	if app.DestinationID != nil {
		if dest, err := a.Store.GetDestination(r.Context(), app.TeamID, *app.DestinationID); err == nil {
			serverID = &dest.ServerID
		}
	}
	dep, err := a.Store.CreateDeployment(r.Context(), app.TeamID, appID, serverID, event.Commit, event.Message, false, true, false)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if err := a.Queue.Enqueue(worker.DeployJob{DeploymentID: dep.ID, TeamID: app.TeamID}); err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"deployment_id": dep.ID, "commit": event.Commit})
}

func (a *API) handleListDatabases(w http.ResponseWriter, r *http.Request) {
	var envID *uuid.UUID
	if s := r.URL.Query().Get("environment_id"); s != "" {
		id, err := uuid.Parse(s)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid environment_id")
			return
		}
		envID = &id
	}
	dbs, err := a.Store.ListDatabases(r.Context(), currentTeamID(r), envID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"databases": dbs})
}

func (a *API) handleCreateDatabase(w http.ResponseWriter, r *http.Request) {
	var body struct {
		EnvironmentID string          `json:"environment_id"`
		DestinationID string          `json:"destination_id"`
		Name          string          `json:"name"`
		Description   string          `json:"description"`
		Engine        string          `json:"engine"`
		Image         string          `json:"image"`
		IsPublic      bool            `json:"is_public"`
		PublicPort    *int            `json:"public_port"`
		EngineConfig  json.RawMessage `json:"engine_config"`
		Password      string          `json:"password"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	envID, err := uuid.Parse(body.EnvironmentID)
	if err != nil || body.Name == "" || body.Engine == "" {
		writeError(w, http.StatusBadRequest, "environment_id, name, engine required")
		return
	}
	if body.Password == "" {
		pw, err := crypto.RandomToken(12)
		if err != nil {
			mapStoreErr(w, err)
			return
		}
		body.Password = pw
	}
	creds, _ := json.Marshal(map[string]string{"password": body.Password})
	enc, err := a.Store.Box.Encrypt(creds)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	db := &store.Database{
		TeamID: currentTeamID(r), EnvironmentID: envID, Name: body.Name, Description: body.Description,
		Engine: body.Engine, Image: body.Image, IsPublic: body.IsPublic, PublicPort: body.PublicPort, EngineConfig: body.EngineConfig,
	}
	if body.DestinationID != "" {
		id, err := uuid.Parse(body.DestinationID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid destination_id")
			return
		}
		db.DestinationID = &id
	}
	created, err := a.Store.CreateDatabase(r.Context(), db, enc)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"database": created, "password": body.Password})
}

func isValidHostnameList(fqdn string) bool {
	for _, part := range strings.Split(fqdn, ",") {
		h := strings.TrimSpace(part)
		if h == "" || strings.ContainsAny(h, "`'\" \t\n\\") {
			return false
		}
		if len(h) > 253 {
			return false
		}
		for _, label := range strings.Split(h, ".") {
			if label == "" || len(label) > 63 {
				return false
			}
			for i, r := range label {
				ok := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-'
				if !ok || (r == '-' && (i == 0 || i == len(label)-1)) {
					return false
				}
			}
		}
	}
	return true
}

func (a *API) handleGetDatabase(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "dbID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	db, err := a.Store.GetDatabase(r.Context(), currentTeamID(r), id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, db)
}

func (a *API) handleStartDatabase(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "dbID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	teamID := currentTeamID(r)
	db, err := a.Store.GetDatabase(r.Context(), teamID, id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if db.DestinationID == nil {
		writeError(w, http.StatusBadRequest, "database has no destination")
		return
	}
	dest, err := a.Store.GetDestination(r.Context(), teamID, *db.DestinationID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	client, err := a.dialServer(r, dest.ServerID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	enc, err := a.Store.GetDatabaseCredentials(r.Context(), teamID, id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	password := ""
	if enc != "" {
		plain, err := a.Store.Box.Decrypt(enc)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "decrypt credentials")
			return
		}
		var creds map[string]string
		if json.Unmarshal(plain, &creds) == nil {
			password = creds["password"]
		}
	}
	if err := database.Start(client, db, dest.Network, password); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := a.Store.UpdateDatabaseStatus(r.Context(), id, "running"); err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "running"})
}

func (a *API) handleStopDatabase(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "dbID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	teamID := currentTeamID(r)
	db, err := a.Store.GetDatabase(r.Context(), teamID, id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if db.DestinationID == nil {
		writeError(w, http.StatusBadRequest, "database has no destination")
		return
	}
	dest, err := a.Store.GetDestination(r.Context(), teamID, *db.DestinationID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	client, err := a.dialServer(r, dest.ServerID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := database.Stop(client, id.String()); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := a.Store.UpdateDatabaseStatus(r.Context(), id, "exited"); err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "exited"})
}
