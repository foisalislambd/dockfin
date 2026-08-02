package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
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
		DestinationID           *string `json:"destination_id"`
		GitSourceID             *string `json:"git_source_id"`
		PrivateKeyID            *string `json:"private_key_id"`
		IsBuildServerEnabled    *bool   `json:"is_build_server_enabled"`
		IsForceHTTPS            *bool   `json:"is_force_https"`
		IsPreviewEnabled        *bool   `json:"is_preview_enabled"`
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
		app.FQDN = *body.FQDN
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
		app.DockerfileLocation = *body.DockerfileLocation
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
	// Re-fetch so response includes settings COALESCE fields.
	fresh, err := a.Store.GetApplication(r.Context(), teamID, appID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, fresh)
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
	// Rollback = redeploy previous finished commit (or force image redeploy)
	var body struct {
		ForceRebuild bool `json:"force_rebuild"`
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
	commit := ""
	if prev != nil {
		commit = *prev
	} else {
		writeError(w, http.StatusBadRequest, "no previous finished deployment to roll back to")
		return
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
