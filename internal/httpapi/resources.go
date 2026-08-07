package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/dockfin/dockfin/internal/crypto"
	"github.com/dockfin/dockfin/internal/database"
	"github.com/dockfin/dockfin/internal/git"
	"github.com/dockfin/dockfin/internal/proxy"
	"github.com/dockfin/dockfin/internal/services"
	"github.com/dockfin/dockfin/internal/sshx"
	"github.com/dockfin/dockfin/internal/store"
	"github.com/dockfin/dockfin/internal/worker"
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

func (a *API) handleUpdateProject(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "projectID"))
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
	p, err := a.Store.UpdateProject(r.Context(), currentTeamID(r), id, body.Name, body.Description)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (a *API) handleDeleteProject(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	teamID := currentTeamID(r)
	p, err := a.Store.GetProject(r.Context(), teamID, id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if _, ok := a.authorizeDestructiveAction(w, r, p.Name, false); !ok {
		return
	}
	if err := a.Store.DeleteProject(r.Context(), teamID, id); err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
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

func (a *API) handleGetEnvironment(w http.ResponseWriter, r *http.Request) {
	pid, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	eid, err := uuid.Parse(chi.URLParam(r, "envID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid environment id")
		return
	}
	env, err := a.Store.GetEnvironment(r.Context(), currentTeamID(r), eid)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if env.ProjectID != pid {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, env)
}

func (a *API) handleGetEnvironmentByID(w http.ResponseWriter, r *http.Request) {
	eid, err := uuid.Parse(chi.URLParam(r, "envID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid environment id")
		return
	}
	env, err := a.Store.GetEnvironment(r.Context(), currentTeamID(r), eid)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, env)
}

func (a *API) handleUpdateEnvironment(w http.ResponseWriter, r *http.Request) {
	pid, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	eid, err := uuid.Parse(chi.URLParam(r, "envID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid environment id")
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
	env, err := a.Store.UpdateEnvironment(r.Context(), currentTeamID(r), pid, eid, body.Name, body.Description)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, env)
}

func (a *API) handleDeleteEnvironment(w http.ResponseWriter, r *http.Request) {
	pid, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	eid, err := uuid.Parse(chi.URLParam(r, "envID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid environment id")
		return
	}
	teamID := currentTeamID(r)
	env, err := a.Store.GetEnvironment(r.Context(), teamID, eid)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if env.ProjectID != pid {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if _, ok := a.authorizeDestructiveAction(w, r, env.Name, false); !ok {
		return
	}
	if err := a.Store.DeleteEnvironment(r.Context(), teamID, pid, eid); err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
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
		GitSourceID             string `json:"git_source_id"`
		PrivateKeyID            string `json:"private_key_id"`
		Dockerfile              string `json:"dockerfile"`
		DockerfileLocation      string `json:"dockerfile_location"`
		DockerComposeLocation   string `json:"docker_compose_location"`
		BaseDirectory           string `json:"base_directory"`
		DockerRegistryImageName string `json:"docker_registry_image_name"`
		DockerRegistryImageTag  string `json:"docker_registry_image_tag"`
		PortsExposes            string `json:"ports_exposes"`
		ComposePrepare          *bool  `json:"compose_prepare"`
		// When non-nil (including empty), only these env vars are seeded — Dockerfile
		// ENV auto-parse is skipped so the UI can clear vars intentionally.
		EnvironmentVariables *[]struct {
			Key         string `json:"key"`
			Value       string `json:"value"`
			IsRuntime   *bool  `json:"is_runtime"`
			IsBuildtime *bool  `json:"is_buildtime"`
		} `json:"environment_variables"`
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
	// Nixpacks was removed; map legacy clients to Railpack.
	if body.BuildPack == "nixpacks" {
		body.BuildPack = "railpack"
	}
	if body.GitBranch == "" {
		body.GitBranch = "main"
	}
	if body.DockerfileLocation == "" {
		body.DockerfileLocation = "/Dockerfile"
	}
	body.Dockerfile = strings.TrimSpace(body.Dockerfile)
	// Coolify SimpleDockerfile: inline content, no Git required.
	if body.BuildPack == "dockerfile" && body.Dockerfile != "" {
		body.GitRepository = ""
		if body.PortsExposes == "" {
			if p := services.PortFromDockerfile(body.Dockerfile); p > 0 {
				body.PortsExposes = strconv.Itoa(p)
			}
		}
	}
	body.BaseDirectory = strings.TrimSpace(body.BaseDirectory)
	if body.BaseDirectory == "" {
		body.BaseDirectory = "/"
	}
	if strings.Contains(body.BaseDirectory, "..") {
		writeError(w, http.StatusBadRequest, "invalid base_directory")
		return
	}
	if body.DockerComposeLocation == "" {
		// Empty = auto-detect on deploy / via detect-compose API.
		body.DockerComposeLocation = ""
	} else {
		body.DockerComposeLocation = services.NormalizeComposeLocation(body.DockerComposeLocation)
	}
	if body.PortsExposes == "" && body.BuildPack != "dockercompose" {
		body.PortsExposes = "3000"
	}
	if body.FQDN != "" && !isValidHostnameList(body.FQDN) {
		writeError(w, http.StatusBadRequest, "invalid fqdn")
		return
	}
	body.FQDN = proxy.NormalizeDomains(body.FQDN)
	teamID := currentTeamID(r)
	if _, err := a.Store.GetEnvironment(r.Context(), teamID, envID); err != nil {
		mapStoreErr(w, err)
		return
	}
	app := &store.Application{
		TeamID:                  teamID,
		EnvironmentID:           envID,
		Name:                    body.Name,
		Description:             body.Description,
		FQDN:                    body.FQDN,
		BuildPack:               body.BuildPack,
		GitRepository:           body.GitRepository,
		GitBranch:               body.GitBranch,
		Dockerfile:              body.Dockerfile,
		DockerfileLocation:      body.DockerfileLocation,
		DockerComposeLocation:   body.DockerComposeLocation,
		DockerRegistryImageName: body.DockerRegistryImageName,
		DockerRegistryImageTag:  body.DockerRegistryImageTag,
		PortsExposes:            body.PortsExposes,
		BaseDirectory:           body.BaseDirectory,
		ComposePrepare:          true,
	}
	if body.ComposePrepare != nil {
		app.ComposePrepare = *body.ComposePrepare
	}
	if body.GitSourceID != "" {
		id, err := uuid.Parse(body.GitSourceID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid git_source_id")
			return
		}
		if _, err := a.Store.GetGitSource(r.Context(), teamID, id); err != nil {
			mapStoreErr(w, err)
			return
		}
		app.GitSourceID = &id
	}
	if body.PrivateKeyID != "" {
		id, err := uuid.Parse(body.PrivateKeyID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid private_key_id")
			return
		}
		if _, err := a.Store.GetPrivateKey(r.Context(), teamID, id); err != nil {
			mapStoreErr(w, err)
			return
		}
		app.PrivateKeyID = &id
	}
	if body.DestinationID != "" {
		id, err := uuid.Parse(body.DestinationID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid destination_id")
			return
		}
		if _, err := a.Store.GetDestination(r.Context(), teamID, id); err != nil {
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
	// Seed env vars. If the client sent environment_variables (even []), honor that
	// list only. Otherwise auto-parse ENV from inline Dockerfile content.
	envSeed := map[string]store.UpsertEnvVarInput{}
	if body.EnvironmentVariables != nil {
		for _, ev := range *body.EnvironmentVariables {
			key := strings.TrimSpace(ev.Key)
			if key == "" {
				continue
			}
			runtime, buildtime := true, true
			if ev.IsRuntime != nil {
				runtime = *ev.IsRuntime
			}
			if ev.IsBuildtime != nil {
				buildtime = *ev.IsBuildtime
			}
			envSeed[key] = store.UpsertEnvVarInput{
				Key: key, Value: ev.Value, Runtime: runtime, Buildtime: buildtime,
			}
		}
	} else if created.Dockerfile != "" {
		for k, v := range services.EnvFromDockerfile(created.Dockerfile) {
			envSeed[k] = store.UpsertEnvVarInput{Key: k, Value: v, Runtime: true, Buildtime: true}
		}
	}
	for _, in := range envSeed {
		if _, err := a.Store.UpsertEnvVar(r.Context(), teamID, "application", created.ID, in); err != nil {
			// Non-fatal: app exists; user can fix env in UI.
			continue
		}
	}
	// Coolify: dockercompose apps load compose on create so Environment Variables
	// populate from ${VAR} / SERVICE_* without a separate Load click.
	if created.BuildPack == "dockercompose" && strings.TrimSpace(created.GitRepository) != "" {
		if _, err := a.loadApplicationCompose(r, teamID, created); err == nil {
			if fresh, err := a.Store.GetApplication(r.Context(), teamID, created.ID); err == nil {
				created = fresh
			}
		}
		// Non-fatal if clone/load fails (private repo / wrong branch) — user can Load Compose later.
	}
	// Always mint a webhook secret so production webhooks are never open by default.
	webhookSecret := ""
	if secret, err := crypto.RandomToken(24); err == nil {
		if err := a.Store.SetWebhookSecret(r.Context(), teamID, created.ID, secret); err == nil {
			webhookSecret = secret
		}
	}
	// Auto-assign free sslip.io/nip.io domain when FQDN left empty.
	if created.FQDN == "" && created.DestinationID != nil {
		if srv, err := a.resolveServerForDomain(r.Context(), created.TeamID, nil, created.DestinationID); err == nil {
			if fqdn := generateResourceFQDN(created.Name, created.ID, srv); fqdn != "" {
				created.FQDN = proxy.NormalizeDomains(fqdn)
				if err := a.Store.UpdateApplication(r.Context(), created); err != nil {
					mapStoreErr(w, err)
					return
				}
			}
		}
	}
	if webhookSecret != "" {
		writeJSON(w, http.StatusCreated, appWithWebhookSecret(created, webhookSecret))
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func appWithWebhookSecret(app *store.Application, secret string) map[string]any {
	out := appWithLinks(app)
	out["webhook_secret"] = secret
	return out
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
	writeJSON(w, http.StatusOK, appWithLinks(app))
}

func appWithLinks(app *store.Application) map[string]any {
	out := enrichApplicationCompose(app)
	out["links"] = proxy.CollectLinks(app.FQDN, "")
	return out
}

func (a *API) handleDeleteApplication(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "appID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	teamID := currentTeamID(r)
	app, err := a.Store.GetApplication(r.Context(), teamID, id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	opts, ok := a.authorizeDestructiveAction(w, r, app.Name, true)
	if !ok {
		return
	}
	// Best-effort remote container (+ optional volume/config/network) removal before DB delete.
	if app.DestinationID != nil {
		if dest, err := a.Store.GetDestination(r.Context(), teamID, *app.DestinationID); err == nil {
			if client, err := a.dialServer(r, dest.ServerID); err == nil {
				cname := "dockfin-" + id.String()
				if opts.volumes() {
					_, _, _ = sshx.RunArgs(client, "docker", "rm", "-f", "-v", cname)
				} else {
					_, _, _ = sshx.RunArgs(client, "docker", "rm", "-f", cname)
				}
				if opts.configurations() {
					_, _, _ = sshx.RunArgs(client, "rm", "-rf", "/data/dockfin/applications/"+id.String())
				}
				if opts.networks() {
					removeResourceScopedNetwork(client, id.String())
				}
				if opts.dockerCleanup() {
					runDockerCleanup(client)
				}
			}
		}
	}
	if err := a.Store.DeleteApplication(r.Context(), teamID, id); err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
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
	dep, err := a.Store.CreateDeployment(r.Context(), teamID, appID, serverID, body.CommitSHA, "", body.ForceRebuild, false, true, 0)
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
	provider := detectWebhookProvider(r, r.URL.Query().Get("provider"))
	event, err := git.ParseWebhook(provider, r, body)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if event.Action == "ping" {
		writeJSON(w, http.StatusOK, map[string]string{"status": "pong"})
		return
	}
	app, err := a.Store.GetApplicationByID(r.Context(), appID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	res := a.processWebhookEvent(r.Context(), app, event)
	status := http.StatusOK
	switch res.Status {
	case "success":
		status = http.StatusAccepted
	case "failed":
		// Queue pressure / store errors — 503 so providers may retry transient failures.
		if strings.Contains(strings.ToLower(res.Message), "queue") {
			status = http.StatusServiceUnavailable
		} else {
			status = http.StatusBadRequest
		}
	}
	out := map[string]any{
		"status":  res.Status,
		"message": res.Message,
		"commit":  res.Commit,
	}
	if res.DeploymentID != nil {
		out["deployment_id"] = *res.DeploymentID
	}
	if res.PreviewID != nil {
		out["preview_id"] = *res.PreviewID
		out["preview_fqdn"] = res.PreviewFQDN
	}
	writeJSON(w, status, out)
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
	teamID := currentTeamID(r)
	if _, err := a.Store.GetEnvironment(r.Context(), teamID, envID); err != nil {
		mapStoreErr(w, err)
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
		TeamID: teamID, EnvironmentID: envID, Name: body.Name, Description: body.Description,
		Engine: body.Engine, Image: body.Image, IsPublic: body.IsPublic, PublicPort: body.PublicPort, EngineConfig: body.EngineConfig,
	}
	if body.DestinationID != "" {
		id, err := uuid.Parse(body.DestinationID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid destination_id")
			return
		}
		if _, err := a.Store.GetDestination(r.Context(), teamID, id); err != nil {
			mapStoreErr(w, err)
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
	for _, part := range proxy.SplitDomainEntries(fqdn) {
		h := proxy.HostFromDomainEntry(part)
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
	return len(proxy.SplitDomainEntries(fqdn)) > 0 || strings.TrimSpace(fqdn) == ""
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

func (a *API) handleDeleteDatabase(w http.ResponseWriter, r *http.Request) {
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
	opts, ok := a.authorizeDestructiveAction(w, r, db.Name, true)
	if !ok {
		return
	}
	if db.DestinationID != nil {
		if dest, err := a.Store.GetDestination(r.Context(), teamID, *db.DestinationID); err == nil {
			if client, err := a.dialServer(r, dest.ServerID); err == nil {
				cname := database.ContainerName(id.String())
				_, _, _ = sshx.RunArgs(client, "docker", "rm", "-f", cname)
				if opts.volumes() {
					_ = database.RemoveData(client, id.String())
				}
				if opts.networks() {
					removeResourceScopedNetwork(client, id.String())
				}
				if opts.dockerCleanup() {
					runDockerCleanup(client)
				}
			}
		}
	}
	if err := a.Store.DeleteDatabase(r.Context(), teamID, id); err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
