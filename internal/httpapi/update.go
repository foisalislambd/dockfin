package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/goolify/goolify/internal/worker"
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
		IsBuildServerEnabled    *bool   `json:"is_build_server_enabled"`
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
	if err := a.Store.UpdateApplication(r.Context(), app); err != nil {
		mapStoreErr(w, err)
		return
	}
	if body.IsBuildServerEnabled != nil {
		if err := a.Store.SetApplicationBuildServerEnabled(r.Context(), teamID, appID, *body.IsBuildServerEnabled); err != nil {
			mapStoreErr(w, err)
			return
		}
		app.IsBuildServerEnabled = *body.IsBuildServerEnabled
	}
	writeJSON(w, http.StatusOK, app)
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
	for _, d := range deps {
		if d.Status == "finished" && d.CommitSHA != "" {
			sha := d.CommitSHA
			prev = &sha
			break
		}
	}
	// Rollback = redeploy last finished (or force image redeploy)
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
	}
	dep, err := a.Store.CreateDeployment(r.Context(), teamID, appID, &dest.ServerID, commit, "rollback", true, false, true)
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
