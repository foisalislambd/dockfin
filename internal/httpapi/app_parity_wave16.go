package httpapi

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/dockfin/dockfin/internal/backup"
	"github.com/dockfin/dockfin/internal/sshx"
	"github.com/dockfin/dockfin/internal/worker"
)

// previewFQDNFromTemplate expands Coolify-style {{pr_id}}.{{domain}} templates.
func previewFQDNFromTemplate(template string, prID int, appFQDN string) string {
	host := strings.TrimSpace(strings.Split(appFQDN, ",")[0])
	host = strings.TrimPrefix(host, "https://")
	host = strings.TrimPrefix(host, "http://")
	host = strings.Split(host, "/")[0]
	if host == "" {
		return ""
	}
	tpl := strings.TrimSpace(template)
	if tpl == "" {
		tpl = "{{pr_id}}.{{domain}}"
	}
	out := strings.ReplaceAll(tpl, "{{pr_id}}", strconv.Itoa(prID))
	out = strings.ReplaceAll(out, "{{domain}}", host)
	out = strings.ReplaceAll(out, "{{PR_ID}}", strconv.Itoa(prID))
	out = strings.ReplaceAll(out, "{{DOMAIN}}", host)
	return out
}

func (a *API) handleRestoreApplicationBackup(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "appID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body struct {
		ExecutionID string `json:"execution_id"`
		Filename    string `json:"filename"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	teamID := currentTeamID(r)
	app, err := a.Store.GetApplication(r.Context(), teamID, id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if app.DestinationID == nil {
		writeError(w, http.StatusBadRequest, "application has no destination")
		return
	}
	filename := strings.TrimSpace(body.Filename)
	if body.ExecutionID != "" {
		eid, err := uuid.Parse(body.ExecutionID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid execution_id")
			return
		}
		list, err := a.Store.ListBackupExecutions(r.Context(), teamID, "application", id)
		if err != nil {
			mapStoreErr(w, err)
			return
		}
		found := false
		for _, e := range list {
			if e.ID == eid {
				if e.Status != "finished" {
					writeError(w, http.StatusBadRequest, "backup execution is not finished")
					return
				}
				filename = e.Filename
				found = true
				break
			}
		}
		if !found {
			writeError(w, http.StatusNotFound, "backup execution not found")
			return
		}
	}
	if filename == "" || strings.Contains(filename, "..") || strings.Contains(filename, "/") {
		writeError(w, http.StatusBadRequest, "filename or execution_id required")
		return
	}
	client, err := a.dialDestination(r, *app.DestinationID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	path := backup.AppVolumeArchivePath(id.String(), filename)
	if err := backup.UntarHostPaths(client, path); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "restored", "filename": filename})
}

func (a *API) handleCloneApplication(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "appID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body struct {
		Name          string `json:"name"`
		EnvironmentID string `json:"environment_id"`
	}
	if err := decodeJSONOptional(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	teamID := currentTeamID(r)
	var envID *uuid.UUID
	if strings.TrimSpace(body.EnvironmentID) != "" {
		eid, err := uuid.Parse(body.EnvironmentID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid environment_id")
			return
		}
		envID = &eid
	}
	created, err := a.Store.CloneApplication(r.Context(), teamID, id, body.Name, envID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, appWithLinks(created))
}

func (a *API) handleManualPreviewDeploy(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "appID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body struct {
		PullRequestID    int    `json:"pull_request_id"`
		PullRequestTitle string `json:"pull_request_title"`
		GitBranch        string `json:"git_branch"`
		CommitSHA        string `json:"commit_sha"`
		ForceRebuild     bool   `json:"force_rebuild"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if body.PullRequestID <= 0 {
		writeError(w, http.StatusBadRequest, "pull_request_id required")
		return
	}
	teamID := currentTeamID(r)
	app, err := a.Store.GetApplication(r.Context(), teamID, id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if !app.IsPreviewEnabled {
		writeError(w, http.StatusBadRequest, "preview deployments disabled")
		return
	}
	branch := strings.TrimSpace(body.GitBranch)
	if branch == "" {
		branch = app.GitBranch
	}
	fqdn := previewFQDNFromTemplate(app.PreviewURLTemplate, body.PullRequestID, app.FQDN)
	preview, err := a.Store.CreatePreview(r.Context(), teamID, id, body.PullRequestID, body.PullRequestTitle, branch, fqdn)
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
	serverID := &dest.ServerID
	dep, err := a.Store.CreateDeployment(r.Context(), teamID, id, serverID, body.CommitSHA, body.PullRequestTitle, body.ForceRebuild, false, true, body.PullRequestID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if err := a.Queue.Enqueue(worker.DeployJob{DeploymentID: dep.ID, TeamID: teamID}); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"preview":       preview,
		"deployment_id": dep.ID,
		"status":        "queued",
	})
}

func (a *API) handleStopAndCleanupApplication(w http.ResponseWriter, r *http.Request) {
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
	if app.DestinationID == nil {
		writeError(w, http.StatusBadRequest, "application has no destination")
		return
	}
	dest, err := a.Store.GetDestination(r.Context(), teamID, *app.DestinationID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	client, err := a.dialServer(r, dest.ServerID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var errOut string
	if app.BuildPack == "dockercompose" {
		errOut, err = a.runAppComposeAction(client, app, "stop")
	} else {
		cname := "dockfin-" + app.ID.String()
		if app.CustomDockerStopTimeout > 0 {
			_, errOut, err = sshx.RunArgs(client, "docker", "stop", "-t", strconv.Itoa(app.CustomDockerStopTimeout), cname)
		} else {
			_, errOut, err = sshx.RunArgs(client, "docker", "stop", cname)
		}
	}
	if err != nil {
		// Idempotent when already stopped; otherwise report failure before cleanup.
		if !strings.Contains(strings.ToLower(errOut+err.Error()), "no such container") &&
			!strings.Contains(strings.ToLower(errOut), "is not running") {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("stop failed: %v %s", err, errOut))
			return
		}
	}
	_ = a.Store.UpdateApplicationStatus(r.Context(), app.ID, "exited")
	runDockerCleanup(client)
	fresh, _ := a.Store.GetApplication(r.Context(), teamID, id)
	if fresh == nil {
		fresh = app
		fresh.Status = "exited"
	}
	writeJSON(w, http.StatusOK, appWithLinks(fresh))
}

func (a *API) handleListServerImages(w http.ResponseWriter, r *http.Request) {
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
	if app.DestinationID == nil {
		writeJSON(w, http.StatusOK, map[string]any{"images": []string{}})
		return
	}
	client, err := a.dialDestination(r, *app.DestinationID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	prefix := fmt.Sprintf("dockfin/%s", id.String())
	out, err := backup.ListDockerImages(client, prefix)
	if err != nil || out == nil {
		out = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"images": out})
}
