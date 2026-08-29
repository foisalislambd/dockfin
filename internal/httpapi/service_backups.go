package httpapi

import (
	"context"
	"net/http"
	"strings"

	"github.com/dockfin/dockfin/internal/backup"
	"github.com/dockfin/dockfin/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (a *API) handleListServiceBackups(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "serviceID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	teamID := currentTeamID(r)
	if _, err := a.Store.GetService(r.Context(), teamID, id); err != nil {
		mapStoreErr(w, err)
		return
	}
	list, err := a.Store.ListBackupExecutions(r.Context(), teamID, "service", id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if list == nil {
		list = []store.BackupExecution{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"backup_executions": list})
}

func (a *API) handleRunServiceBackup(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "serviceID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	teamID := currentTeamID(r)
	svc, err := a.Store.GetService(r.Context(), teamID, id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	serverID, _, err := a.resolveServiceTarget(r.Context(), teamID, svc)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	client, err := a.dialServer(r, serverID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	filename := backup.DefaultServiceBackupFilename(id.String())
	path := backup.ServiceVolumeArchivePath(id.String(), filename)
	exec, err := a.Store.CreateBackupExecution(r.Context(), teamID, "service", id, filename)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	done := false
	defer func() {
		if !done {
			_ = a.Store.FinishBackupExecution(context.Background(), exec.ID, "failed", 0, "interrupted")
		}
	}()
	hostDir := "/data/dockfin/services/" + id.String()
	paths := backup.FilterExistingPaths(client, []string{hostDir})
	if len(paths) == 0 {
		_ = a.Store.FinishBackupExecution(r.Context(), exec.ID, "failed", 0, "no service files found on server")
		done = true
		writeError(w, http.StatusBadRequest, "no service files found on server — deploy first")
		return
	}
	if err := backup.TarHostPaths(client, path, paths); err != nil {
		_ = a.Store.FinishBackupExecution(r.Context(), exec.ID, "failed", 0, err.Error())
		done = true
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	size := backup.FileSize(client, path)
	if err := a.Store.FinishBackupExecution(r.Context(), exec.ID, "finished", size, ""); err != nil {
		mapStoreErr(w, err)
		done = true
		return
	}
	done = true
	_ = backup.EnforceServiceBackupRetention(client, id.String(), 14)
	out, err := a.Store.GetBackupExecution(r.Context(), teamID, exec.ID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) handleRestoreServiceBackup(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "serviceID"))
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
	svc, err := a.Store.GetService(r.Context(), teamID, id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	filename := strings.TrimSpace(body.Filename)
	if body.ExecutionID != "" {
		eid, err := uuid.Parse(body.ExecutionID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid execution_id")
			return
		}
		ex, err := a.Store.GetBackupExecution(r.Context(), teamID, eid)
		if err != nil {
			mapStoreErr(w, err)
			return
		}
		if ex.ResourceType != "service" || ex.ResourceID != id {
			writeError(w, http.StatusBadRequest, "execution does not belong to this service")
			return
		}
		if ex.Status != "finished" {
			writeError(w, http.StatusBadRequest, "only finished backups can be restored")
			return
		}
		filename = ex.Filename
	}
	if !safeBackupFilename(filename) {
		writeError(w, http.StatusBadRequest, "execution_id or safe filename required")
		return
	}
	serverID, _, err := a.resolveServiceTarget(r.Context(), teamID, svc)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	client, err := a.dialServer(r, serverID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	path := backup.ServiceVolumeArchivePath(id.String(), filename)
	if err := backup.UntarHostPaths(client, path); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "restored", "filename": filename})
}
