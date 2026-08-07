package httpapi

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/dockfin/dockfin/internal/proxy"
	"github.com/dockfin/dockfin/internal/services"
	"github.com/dockfin/dockfin/internal/worker"
)

func (a *API) handleUpdateApplication(w http.ResponseWriter, r *http.Request) {
	appID, err := uuid.Parse(chi.URLParam(r, "appID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	teamID := currentTeamID(r)
	app, err := a.Store.GetApplication(r.Context(), teamID, appID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	var body struct {
		Name                    *string `json:"name"`
		Description             *string `json:"description"`
		FQDN                    *string `json:"fqdn"`
		GitRepository           *string `json:"git_repository"`
		GitBranch               *string `json:"git_branch"`
		PortsExposes            *string `json:"ports_exposes"`
		DockerRegistryImageName *string `json:"docker_registry_image_name"`
		DockerRegistryImageTag  *string `json:"docker_registry_image_tag"`
		DockerfileLocation      *string `json:"dockerfile_location"`
		Dockerfile              *string `json:"dockerfile"`
		DockerComposeLocation   *string `json:"docker_compose_location"`
		ComposePrepare          *bool   `json:"compose_prepare"`
		BaseDirectory           *string `json:"base_directory"`
		DockerComposeCustomBuildCommand *string `json:"docker_compose_custom_build_command"`
		DockerComposeCustomStartCommand *string `json:"docker_compose_custom_start_command"`
		CustomDockerRunOptions  *string `json:"custom_docker_run_options"`
		DockerfileTargetBuild   *string `json:"dockerfile_target_build"`
		DockerComposeDomains    map[string]composeServiceDomain `json:"docker_compose_domains"`
		DestinationID           *string `json:"destination_id"`
		GitSourceID             *string `json:"git_source_id"`
		PrivateKeyID            *string `json:"private_key_id"`
		IsBuildServerEnabled    *bool   `json:"is_build_server_enabled"`
		IsForceHTTPS            *bool   `json:"is_force_https"`
		IsPreviewEnabled        *bool   `json:"is_preview_enabled"`
		IsAutoDeployEnabled     *bool   `json:"is_auto_deploy_enabled"`
		IsGitSubmodulesEnabled  *bool   `json:"is_git_submodules_enabled"`
		IsPreserveRepositoryEnabled *bool `json:"is_preserve_repository_enabled"`
		WatchPaths              *string `json:"watch_paths"`
		HealthCheckEnabled      *bool   `json:"health_check_enabled"`
		HealthCheckPath         *string `json:"health_check_path"`
		HealthCheckPort         *int    `json:"health_check_port"`
		HealthCheckMethod       *string `json:"health_check_method"`
		HealthCheckReturnCode   *int    `json:"health_check_return_code"`
		HealthCheckInterval     *int    `json:"health_check_interval"`
		HealthCheckTimeout      *int    `json:"health_check_timeout"`
		HealthCheckRetries      *int    `json:"health_check_retries"`
		LimitsMemory            *string `json:"limits_memory"`
		LimitsCpus              *string `json:"limits_cpus"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if body.Name != nil {
		app.Name = *body.Name
	}
	if body.Description != nil {
		app.Description = *body.Description
	}
	if body.FQDN != nil {
		if *body.FQDN != "" && !isValidHostnameList(*body.FQDN) {
			writeError(w, http.StatusBadRequest, "invalid fqdn")
			return
		}
		app.FQDN = proxy.NormalizeDomains(*body.FQDN)
	}
	if body.GitRepository != nil {
		app.GitRepository = *body.GitRepository
	}
	if body.GitBranch != nil {
		app.GitBranch = *body.GitBranch
	}
	if body.PortsExposes != nil {
		app.PortsExposes = *body.PortsExposes
	}
	if body.DockerRegistryImageName != nil {
		app.DockerRegistryImageName = *body.DockerRegistryImageName
	}
	if body.DockerRegistryImageTag != nil {
		app.DockerRegistryImageTag = *body.DockerRegistryImageTag
	}
	if body.DockerfileLocation != nil {
		loc := strings.TrimSpace(*body.DockerfileLocation)
		loc = filepath.ToSlash(loc)
		if loc == "" || strings.Contains(loc, "..") {
			writeError(w, http.StatusBadRequest, "invalid dockerfile_location")
			return
		}
		cleaned := filepath.ToSlash(filepath.Clean("/" + strings.TrimPrefix(loc, "/")))
		if cleaned == "/" || strings.Contains(cleaned, "..") {
			writeError(w, http.StatusBadRequest, "invalid dockerfile_location")
			return
		}
		app.DockerfileLocation = cleaned
	}
	if body.Dockerfile != nil {
		app.Dockerfile = strings.TrimSpace(*body.Dockerfile)
		if app.Dockerfile != "" && app.PortsExposes == "" {
			if p := services.PortFromDockerfile(app.Dockerfile); p > 0 {
				app.PortsExposes = strconv.Itoa(p)
			}
		}
	}
	if body.DockerComposeLocation != nil {
		raw := strings.TrimSpace(*body.DockerComposeLocation)
		if raw != "" && raw != "auto" && raw != "auto-detect" {
			norm := services.NormalizeComposeLocation(raw)
			if norm == "" {
				writeError(w, http.StatusBadRequest, "invalid docker_compose_location")
				return
			}
			app.DockerComposeLocation = norm
		} else {
			app.DockerComposeLocation = services.NormalizeComposeLocation(raw)
		}
	}
	if body.ComposePrepare != nil {
		app.ComposePrepare = *body.ComposePrepare
	}
	if body.BaseDirectory != nil {
		base := strings.TrimSpace(*body.BaseDirectory)
		if base == "" {
			base = "/"
		}
		if strings.Contains(base, "..") {
			writeError(w, http.StatusBadRequest, "invalid base_directory")
			return
		}
		app.BaseDirectory = base
	}
	if body.DockerComposeCustomBuildCommand != nil {
		app.DockerComposeCustomBuildCommand = *body.DockerComposeCustomBuildCommand
	}
	if body.DockerComposeCustomStartCommand != nil {
		app.DockerComposeCustomStartCommand = *body.DockerComposeCustomStartCommand
	}
	if body.CustomDockerRunOptions != nil {
		app.CustomDockerRunOptions = *body.CustomDockerRunOptions
	}
	if body.DockerfileTargetBuild != nil {
		app.DockerfileTargetBuild = *body.DockerfileTargetBuild
	}
	if body.DockerComposeDomains != nil {
		b, err := json.Marshal(body.DockerComposeDomains)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid docker_compose_domains")
			return
		}
		app.DockerComposeDomains = b
		// Keep top-level fqdn in sync with aggregated per-service domains.
		if agg := aggregateComposeDomains(body.DockerComposeDomains); agg != "" {
			if !isValidHostnameList(agg) {
				writeError(w, http.StatusBadRequest, "invalid fqdn in docker_compose_domains")
				return
			}
			app.FQDN = proxy.NormalizeDomains(agg)
		}
	}
	if body.DestinationID != nil && *body.DestinationID != "" {
		id, err := uuid.Parse(*body.DestinationID)
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
	if body.GitSourceID != nil {
		if *body.GitSourceID == "" {
			app.GitSourceID = nil
		} else {
			id, err := uuid.Parse(*body.GitSourceID)
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
	}
	if body.PrivateKeyID != nil {
		if *body.PrivateKeyID == "" {
			app.PrivateKeyID = nil
		} else {
			id, err := uuid.Parse(*body.PrivateKeyID)
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
	}
	if body.HealthCheckEnabled != nil {
		app.HealthCheckEnabled = *body.HealthCheckEnabled
	}
	if body.HealthCheckPath != nil {
		app.HealthCheckPath = *body.HealthCheckPath
	}
	if body.HealthCheckPort != nil {
		if *body.HealthCheckPort <= 0 {
			app.HealthCheckPort = nil
		} else if *body.HealthCheckPort > 65535 {
			writeError(w, http.StatusBadRequest, "health_check_port must be between 1 and 65535")
			return
		} else {
			app.HealthCheckPort = body.HealthCheckPort
		}
	}
	if body.HealthCheckMethod != nil {
		app.HealthCheckMethod = *body.HealthCheckMethod
	}
	if body.HealthCheckReturnCode != nil {
		app.HealthCheckReturnCode = *body.HealthCheckReturnCode
	}
	if body.HealthCheckInterval != nil {
		app.HealthCheckInterval = *body.HealthCheckInterval
	}
	if body.HealthCheckTimeout != nil {
		app.HealthCheckTimeout = *body.HealthCheckTimeout
	}
	if body.HealthCheckRetries != nil {
		app.HealthCheckRetries = *body.HealthCheckRetries
	}
	if body.LimitsMemory != nil {
		app.LimitsMemory = *body.LimitsMemory
	}
	if body.LimitsCpus != nil {
		app.LimitsCpus = *body.LimitsCpus
	}
	if body.IsForceHTTPS != nil {
		app.IsForceHTTPS = *body.IsForceHTTPS
	}
	if err := a.Store.UpdateApplication(r.Context(), app); err != nil {
		mapStoreErr(w, err)
		return
	}
	if body.IsForceHTTPS != nil {
		if err := a.Store.SetApplicationForceHTTPS(r.Context(), teamID, appID, *body.IsForceHTTPS); err != nil {
			mapStoreErr(w, err)
			return
		}
	}
	if body.IsBuildServerEnabled != nil {
		if err := a.Store.SetApplicationBuildServerEnabled(r.Context(), teamID, appID, *body.IsBuildServerEnabled); err != nil {
			mapStoreErr(w, err)
			return
		}
		app.IsBuildServerEnabled = *body.IsBuildServerEnabled
	}
	if body.IsPreviewEnabled != nil {
		if err := a.Store.SetApplicationPreviewEnabled(r.Context(), teamID, appID, *body.IsPreviewEnabled); err != nil {
			mapStoreErr(w, err)
			return
		}
		app.IsPreviewEnabled = *body.IsPreviewEnabled
	}
	if body.IsAutoDeployEnabled != nil {
		if err := a.Store.SetApplicationAutoDeployEnabled(r.Context(), teamID, appID, *body.IsAutoDeployEnabled); err != nil {
			mapStoreErr(w, err)
			return
		}
	}
	if body.IsGitSubmodulesEnabled != nil {
		if err := a.Store.SetApplicationGitSubmodulesEnabled(r.Context(), teamID, appID, *body.IsGitSubmodulesEnabled); err != nil {
			mapStoreErr(w, err)
			return
		}
	}
	if body.IsPreserveRepositoryEnabled != nil {
		if err := a.Store.SetApplicationPreserveRepositoryEnabled(r.Context(), teamID, appID, *body.IsPreserveRepositoryEnabled); err != nil {
			mapStoreErr(w, err)
			return
		}
	}
	if body.WatchPaths != nil {
		if err := a.Store.SetApplicationWatchPaths(r.Context(), teamID, appID, *body.WatchPaths); err != nil {
			mapStoreErr(w, err)
			return
		}
	}
	// Re-fetch so response includes settings COALESCE fields.
	fresh, err := a.Store.GetApplication(r.Context(), teamID, appID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, appWithLinks(fresh))
}

func (a *API) handleRollbackApplication(w http.ResponseWriter, r *http.Request) {
	appID, err := uuid.Parse(chi.URLParam(r, "appID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	teamID := currentTeamID(r)
	deps, err := a.Store.ListDeployments(r.Context(), teamID, appID, 20)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	var prev *string
	finishedSeen := 0
	for _, d := range deps {
		if d.Status == "finished" && d.CommitSHA != "" {
			finishedSeen++
			// Skip the latest finished deployment; use the previous one.
			if finishedSeen < 2 {
				continue
			}
			sha := d.CommitSHA
			prev = &sha
			break
		}
	}
	// Rollback = redeploy a chosen finished commit (default: previous finished).
	var body struct {
		ForceRebuild bool   `json:"force_rebuild"`
		CommitSHA    string `json:"commit_sha"`
	}
	_ = decodeJSON(r, &body)
	app, err := a.Store.GetApplication(r.Context(), teamID, appID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if app.DestinationID == nil {
		writeError(w, http.StatusBadRequest, "application has no destination")
		return
	}
	dest, err := a.Store.GetDestination(r.Context(), teamID, *app.DestinationID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	commit := strings.TrimSpace(body.CommitSHA)
	if commit == "" {
		if prev != nil {
			commit = *prev
		} else {
			writeError(w, http.StatusBadRequest, "no previous finished deployment to roll back to")
			return
		}
	} else {
		// Ensure the SHA was a finished deployment for this app.
		ok := false
		for _, d := range deps {
			if d.Status == "finished" && d.CommitSHA == commit {
				ok = true
				break
			}
		}
		if !ok {
			writeError(w, http.StatusBadRequest, "commit_sha is not a finished deployment for this application")
			return
		}
	}
	dep, err := a.Store.CreateDeployment(r.Context(), teamID, appID, &dest.ServerID, commit, "rollback", true, false, true, 0)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if err := a.Queue.Enqueue(worker.DeployJob{DeploymentID: dep.ID, TeamID: teamID, ForceRebuild: true}); err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, dep)
}
