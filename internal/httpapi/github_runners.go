package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/dockfin/dockfin/internal/git/githubapp"
	"github.com/dockfin/dockfin/internal/githubrunner"
	"github.com/dockfin/dockfin/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (a *API) handleGetGithubRunners(w http.ResponseWriter, r *http.Request) {
	serverID, err := uuid.Parse(chi.URLParam(r, "serverID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	teamID := currentTeamID(r)
	cfg, err := a.Store.GetGithubRunnerConfig(r.Context(), teamID, serverID)
	if err != nil {
		if err == store.ErrNotFound {
			writeJSON(w, http.StatusOK, map[string]any{"config": nil, "executions": []any{}})
			return
		}
		mapStoreErr(w, err)
		return
	}
	execs, err := a.Store.ListGithubRunnerExecutions(r.Context(), teamID, serverID, 30)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"config": cfg, "executions": execs})
}

func (a *API) handlePutGithubRunners(w http.ResponseWriter, r *http.Request) {
	if !isTeamAdmin(r) {
		writeError(w, http.StatusForbidden, "admin or owner role required")
		return
	}
	serverID, err := uuid.Parse(chi.URLParam(r, "serverID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body struct {
		GitSourceID        string `json:"git_source_id"`
		Labels             string `json:"labels"`
		Enabled            bool   `json:"enabled"`
		AllowPullRequests  bool   `json:"allow_pull_requests"`
		MaxRunners         int    `json:"max_runners"`
		DockerMode         string `json:"docker_mode"`
		RunnerImage        string `json:"runner_image"`
		CPULimit           string `json:"cpu_limit"`
		MemoryLimit        string `json:"memory_limit"`
		QueueWaitMinutes   int    `json:"queue_wait_minutes"`
		IdleTimeoutMinutes int    `json:"idle_timeout_minutes"`
		JobTimeoutMinutes  int    `json:"job_timeout_minutes"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	sourceID, err := uuid.Parse(body.GitSourceID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "choose an organization GitHub App")
		return
	}
	labels, err := githubrunner.NormalizeLabels(body.Labels)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if body.DockerMode != "none" && body.DockerMode != "dind" {
		writeError(w, http.StatusBadRequest, "docker mode must be none or dind")
		return
	}
	if body.MaxRunners < 1 || body.MaxRunners > 32 {
		writeError(w, http.StatusBadRequest, "parallel runners must be from 1 to 32")
		return
	}
	if body.QueueWaitMinutes < 1 || body.QueueWaitMinutes > 1440 || body.IdleTimeoutMinutes < 1 || body.IdleTimeoutMinutes > 1440 || body.JobTimeoutMinutes < 1 || body.JobTimeoutMinutes > 7200 {
		writeError(w, http.StatusBadRequest, "timeouts are in minutes, within the allowed range")
		return
	}
	if err := githubrunner.ValidateImage(body.RunnerImage); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := githubrunner.ValidateCPU(body.CPULimit); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := githubrunner.ValidateMemory(body.MemoryLimit); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	sec, err := a.Store.GetGitSourceSecrets(r.Context(), currentTeamID(r), sourceID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if strings.TrimSpace(sec.Organization) == "" || strings.TrimSpace(sec.InstallationID) == "" {
		writeError(w, http.StatusBadRequest, "runners need an installed GitHub App that belongs to an organization")
		return
	}
	cfg, err := a.Store.SaveGithubRunnerConfig(r.Context(), store.GithubRunnerConfig{
		TeamID: currentTeamID(r), ServerID: serverID, GitSourceID: sourceID, Labels: labels,
		Enabled: body.Enabled, AllowPullRequests: body.AllowPullRequests, MaxRunners: body.MaxRunners,
		DockerMode: body.DockerMode, RunnerImage: strings.TrimSpace(body.RunnerImage),
		CPULimit: strings.TrimSpace(body.CPULimit), MemoryLimit: strings.ToLower(strings.TrimSpace(body.MemoryLimit)),
		QueueWaitMinutes: body.QueueWaitMinutes, IdleTimeoutMinutes: body.IdleTimeoutMinutes, JobTimeoutMinutes: body.JobTimeoutMinutes,
	})
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (a *API) handleWorkflowJob(w http.ResponseWriter, r *http.Request, gs *store.GitSource, sec *store.GitSourceSecrets, body []byte) {
	var payload struct {
		Action       string `json:"action"`
		Installation struct {
			ID int64 `json:"id"`
		} `json:"installation"`
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
		WorkflowJob struct {
			ID           int64    `json:"id"`
			RunID        int64    `json:"run_id"`
			Name         string   `json:"name"`
			WorkflowName string   `json:"workflow_name"`
			HTMLURL      string   `json:"html_url"`
			RunnerName   string   `json:"runner_name"`
			Conclusion   string   `json:"conclusion"`
			Labels       []string `json:"labels"`
		} `json:"workflow_job"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || payload.WorkflowJob.ID <= 0 {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignored", "reason": "no workflow job"})
		return
	}
	if sec.InstallationID == "" || sec.InstallationID != jsonNumber(payload.Installation.ID) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignored", "reason": "different installation"})
		return
	}
	ctx := r.Context()
	switch payload.Action {
	case "queued":
		configs, err := a.Store.EnabledRunnerConfigsForSource(ctx, gs.ID)
		if err != nil {
			mapStoreErr(w, err)
			return
		}
		var matching []store.GithubRunnerConfig
		refusesPR := false
		allowsPR := false
		wait := 0
		for _, cfg := range configs {
			if !githubrunner.Matches(cfg.Labels, payload.WorkflowJob.Labels) {
				continue
			}
			matching = append(matching, cfg)
			if !cfg.AllowPullRequests {
				refusesPR = true
			} else {
				allowsPR = true
			}
			if wait == 0 || cfg.QueueWaitMinutes < wait {
				wait = cfg.QueueWaitMinutes
			}
		}
		if len(matching) == 0 {
			writeJSON(w, http.StatusOK, map[string]string{"status": "ignored", "reason": "no matching runner"})
			return
		}
		isPR := false
		if refusesPR {
			app := a.githubApp(sec)
			event, err := app.WorkflowRunEvent(sec.InstallationID, payload.Repository.FullName, payload.WorkflowJob.RunID)
			if err == nil {
				isPR = githubrunner.IsPullRequestEvent(event)
			}
		}
		if isPR && !allowsPR {
			writeJSON(w, http.StatusOK, map[string]string{"status": "ignored", "reason": "pull request jobs are not allowed"})
			return
		}
		id, created, err := a.Store.CreateRunnerExecution(ctx, store.RunnerExecutionInput{
			TeamID: gs.TeamID, GitSourceID: gs.ID, TriggerJobID: payload.WorkflowJob.ID,
			HTMLURL: limitURL(payload.WorkflowJob.HTMLURL), WorkflowName: limitText(payload.WorkflowJob.WorkflowName),
			JobName: limitText(payload.WorkflowJob.Name), Repository: limitText(payload.Repository.FullName),
			Labels: payload.WorkflowJob.Labels, IsPullRequest: isPR, QueueWaitMinutes: wait,
		})
		if err != nil {
			mapStoreErr(w, err)
			return
		}
		if created && a.Queue != nil {
			go func() {
				_ = githubrunner.Provision(context.Background(), a.Store, a.Queue.SSH, id)
			}()
		}
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "queued"})
	case "in_progress":
		if payload.WorkflowJob.RunnerName == "" {
			writeJSON(w, http.StatusOK, map[string]string{"status": "ignored"})
			return
		}
		requeue, err := a.Store.MarkRunnerRunning(ctx, gs.ID, payload.WorkflowJob.RunnerName, payload.WorkflowJob.ID,
			limitURL(payload.WorkflowJob.HTMLURL), limitText(payload.WorkflowJob.WorkflowName), limitText(payload.WorkflowJob.Name), limitText(payload.Repository.FullName))
		if err != nil {
			mapStoreErr(w, err)
			return
		}
		if requeue != uuid.Nil && a.Queue != nil {
			go func() {
				_ = githubrunner.Provision(context.Background(), a.Store, a.Queue.SSH, requeue)
			}()
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "running"})
	case "completed":
		if payload.WorkflowJob.RunnerName != "" {
			exec, err := a.Store.FinishRunnerByName(ctx, gs.ID, payload.WorkflowJob.RunnerName, limitText(payload.WorkflowJob.Conclusion))
			if err != nil {
				mapStoreErr(w, err)
				return
			}
			if exec != nil && a.Queue != nil {
				serverID := uuid.Nil
				if exec.ServerID != nil {
					serverID = *exec.ServerID
				}
				go githubrunner.Cleanup(context.Background(), a.Store, a.Queue.SSH, exec.TeamID, serverID, exec.GitSourceID, exec.ID)
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "completed"})
			return
		}
		_ = a.Store.CancelQueuedRunner(ctx, gs.ID, payload.WorkflowJob.ID, "the job ended before a runner started")
		writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
	default:
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignored"})
	}
}

func (a *API) githubApp(sec *store.GitSourceSecrets) *githubapp.App {
	key, err := a.Store.Box.DecryptString(sec.PrivateKeyEnc)
	if err != nil {
		key = ""
	}
	return &githubapp.App{
		AppID: sec.AppID, ClientID: sec.ClientID, PrivateKeyPEM: key,
		HTMLURL: sec.HTMLURL, APIURL: sec.APIURL, Name: sec.Name, Organization: sec.Organization,
	}
}

func jsonNumber(n int64) string {
	return strconv.FormatInt(n, 10)
}

func limitText(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 255 {
		return s[:255]
	}
	return s
}

func limitURL(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 500 || !strings.HasPrefix(s, "https://") {
		return ""
	}
	return s
}
