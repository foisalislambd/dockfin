package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/dockfin/dockfin/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (a *API) StartJobs(ctx context.Context) {
	ctx, a.jobCancel = context.WithCancel(ctx)
	for i := 0; i < 2; i++ {
		go a.jobLoop(ctx)
	}
}

func (a *API) StopJobs() {
	if a.jobCancel != nil {
		a.jobCancel()
	}
}

func (a *API) jobLoop(ctx context.Context) {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			if err := a.Store.RequeueExpiredJobs(ctx); err != nil && ctx.Err() == nil {
				slog.Error("requeue jobs", "err", err)
			}
			job, err := a.Store.ClaimJob(ctx)
			if err != nil || job == nil {
				continue
			}
			a.executeJob(ctx, job)
		}
	}
}

func (a *API) executeJob(ctx context.Context, job *store.BackgroundJob) {
	runCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	var result []byte
	var err error
	switch job.Kind {
	case "image_scan":
		result, err = a.jobImageScan(runCtx, job)
	case "docker_cleanup":
		result, err = a.jobDockerCleanup(runCtx, job)
	case "instance_backup":
		result, err = a.jobInstanceBackup(runCtx, job)
	case "resource_delete":
		result, err = a.jobResourceDelete(runCtx, job)
	case "resource_power":
		result, err = a.jobResourcePower(runCtx, job)
	default:
		err = errString("unknown job kind")
	}
	if err != nil {
		_ = a.Store.FailJob(context.Background(), job.ID, job.Attempts, job.MaxAttempts, err.Error())
		if job.Attempts >= job.MaxAttempts {
			if job.Kind == "resource_delete" {
				a.noteDeleteFailure(job, err)
			}
			if job.Kind == "resource_power" {
				a.notePowerFailure(job)
			}
		}
		return
	}
	_ = a.Store.FinishJob(context.Background(), job.ID, result)
}

func (a *API) jobImageScan(ctx context.Context, job *store.BackgroundJob) ([]byte, error) {
	var p struct {
		ServerID     string `json:"server_id"`
		ResourceType string `json:"resource_type"`
		ResourceID   string `json:"resource_id"`
		Image        string `json:"image"`
	}
	if err := json.Unmarshal(job.Payload, &p); err != nil {
		return nil, err
	}
	serverID, err := uuid.Parse(p.ServerID)
	if err != nil {
		return nil, err
	}
	resourceID, err := uuid.Parse(p.ResourceID)
	if err != nil {
		return nil, err
	}
	sc, err := a.scanImage(ctx, job.TeamID, serverID, p.ResourceType, resourceID, p.Image)
	raw, _ := json.Marshal(map[string]any{"scan_id": scID(sc), "error": errStringOr(err)})
	if err != nil {
		return raw, err
	}
	return raw, nil
}

func scID(sc *store.ImageScan) string {
	if sc == nil {
		return ""
	}
	return sc.ID.String()
}

func errStringOr(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func (a *API) jobDockerCleanup(ctx context.Context, job *store.BackgroundJob) ([]byte, error) {
	var p struct {
		ServerID    string `json:"server_id"`
		ExecutionID string `json:"execution_id"`
	}
	if err := json.Unmarshal(job.Payload, &p); err != nil {
		return nil, err
	}
	serverID, err := uuid.Parse(p.ServerID)
	if err != nil {
		return nil, err
	}
	execID, err := uuid.Parse(p.ExecutionID)
	if err != nil {
		return nil, err
	}
	ops, err := a.Store.GetServerOpsSettings(ctx, job.TeamID, serverID)
	if err != nil {
		_ = a.Store.FinishDockerCleanupExecution(ctx, execID, "failed", err.Error())
		return nil, err
	}
	client, err := a.dialServerTeam(ctx, job.TeamID, serverID)
	if err != nil {
		_ = a.Store.FinishDockerCleanupExecution(ctx, execID, "failed", err.Error())
		return nil, err
	}
	msg, runErr := runDockerCleanupDetailed(client, ops.ForceDockerCleanup, ops.DockerCleanupThreshold)
	status := "finished"
	if runErr != nil {
		status = "failed"
		if msg == "" {
			msg = runErr.Error()
		} else {
			msg = msg + "; " + runErr.Error()
		}
	}
	_ = a.Store.FinishDockerCleanupExecution(ctx, execID, status, msg)
	raw, _ := json.Marshal(map[string]string{"status": status, "message": msg, "execution_id": execID.String()})
	if runErr != nil {
		return raw, runErr
	}
	return raw, nil
}

func (a *API) jobInstanceBackup(ctx context.Context, job *store.BackgroundJob) ([]byte, error) {
	cfg, err := a.Store.GetInstanceBackupConfig(ctx)
	if err != nil {
		return nil, err
	}
	exec, err := a.executeInstanceBackup(ctx, job.TeamID, cfg)
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{"execution_id": exec.ID, "status": exec.Status})
}

func (a *API) handleListJobs(w http.ResponseWriter, r *http.Request) {
	list, err := a.Store.ListJobs(r.Context(), currentTeamID(r), 30)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"jobs": list})
}

func (a *API) handleGetJob(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "jobID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid job")
		return
	}
	job, err := a.Store.GetJob(r.Context(), currentTeamID(r), id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (a *API) handleCancelJob(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "jobID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid job")
		return
	}
	if err := a.Store.CancelJob(r.Context(), currentTeamID(r), id); err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}
