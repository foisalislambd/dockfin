package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/goolify/goolify/internal/backup"
	"github.com/goolify/goolify/internal/store"
)

func (a *API) requireSettingsAdmin(w http.ResponseWriter, r *http.Request) bool {
	role := r.Context().Value(ctxRole).(string)
	if role != "owner" && role != "admin" {
		writeError(w, http.StatusForbidden, "admin or owner role required")
		return false
	}
	return true
}

func (a *API) handleGetInstanceBackup(w http.ResponseWriter, r *http.Request) {
	cfg, err := a.Store.GetInstanceBackupConfig(r.Context())
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	execs, err := a.Store.ListInstanceBackupExecutions(r.Context(), 50)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if execs == nil {
		execs = []store.BackupExecution{}
	}
	container := cfg.Container
	detected := ""
	if d, err := backup.DetectPostgresContainer(container); err == nil {
		detected = d
		container = d // always the running container that dumps will use
	}
	user, password, dbName, _ := backup.ParseDatabaseURL(a.Cfg.DatabaseURL)
	if cfg.DBUser == "" {
		cfg.DBUser = user
	}
	if cfg.DBName == "" {
		cfg.DBName = dbName
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"backup": cfg,
		"runtime": map[string]any{
			"container":          container,
			"detected_container": detected,
			"data_dir":           a.Cfg.DataDir,
			"backup_dir":         backup.InstanceDumpDir(a.Cfg.DataDir),
			"db_password_set":    password != "",
		},
		"executions": execs,
	})
}

func (a *API) handleConfigureInstanceBackup(w http.ResponseWriter, r *http.Request) {
	if !a.requireSettingsAdmin(w, r) {
		return
	}
	user, _, dbName, err := backup.ParseDatabaseURL(a.Cfg.DatabaseURL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invalid database url")
		return
	}
	container, err := backup.DetectPostgresContainer("")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	cfg, err := a.Store.ConfigureInstanceBackup(r.Context(), container, user, dbName, "Goolify database")
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"backup": cfg, "status": "configured"})
}

func (a *API) handlePatchInstanceBackup(w http.ResponseWriter, r *http.Request) {
	if !a.requireSettingsAdmin(w, r) {
		return
	}
	var patch store.InstanceBackupPatch
	if err := decodeJSON(r, &patch); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	cfg, err := a.Store.UpdateInstanceBackupConfig(r.Context(), patch)
	if err != nil {
		if errors.Is(err, store.ErrConflict) {
			writeConflictDetail(w, err)
			return
		}
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"backup": cfg})
}

func (a *API) handleRunInstanceBackup(w http.ResponseWriter, r *http.Request) {
	if !a.requireSettingsAdmin(w, r) {
		return
	}
	cfg, err := a.Store.GetInstanceBackupConfig(r.Context())
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if !cfg.Configured {
		writeError(w, http.StatusBadRequest, "configure instance backup first")
		return
	}
	exec, err := a.executeInstanceBackup(r.Context(), currentTeamID(r), cfg)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"execution": exec})
}

func (a *API) handleListInstanceBackupExecutions(w http.ResponseWriter, r *http.Request) {
	list, err := a.Store.ListInstanceBackupExecutions(r.Context(), 50)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if list == nil {
		list = []store.BackupExecution{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"executions": list})
}

func (a *API) executeInstanceBackup(ctx context.Context, teamID uuid.UUID, cfg *store.InstanceBackupConfig) (*store.BackupExecution, error) {
	user, password, dbName, err := backup.ParseDatabaseURL(a.Cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	if cfg.DBUser != "" {
		user = cfg.DBUser
	}
	if cfg.DBName != "" {
		dbName = cfg.DBName
	}
	container, err := backup.DetectPostgresContainer(cfg.Container)
	if err != nil {
		return nil, err
	}

	filename := backup.InstanceDumpFilename()
	execRow, err := a.Store.CreateInstanceBackupExecution(ctx, teamID, filename)
	if err != nil {
		return nil, err
	}
	done := false
	defer func() {
		if !done {
			_ = a.Store.FinishBackupExecution(context.Background(), execRow.ID, "failed", 0, "interrupted")
		}
	}()

	_, size, err := backup.DumpInstanceLocal(a.Cfg.DataDir, container, user, password, dbName, filename)
	if err != nil {
		_ = a.Store.FinishBackupExecution(ctx, execRow.ID, "failed", 0, err.Error())
		done = true
		return nil, err
	}
	if err := a.Store.FinishBackupExecution(ctx, execRow.ID, "finished", size, ""); err != nil {
		done = true
		return nil, err
	}
	done = true
	_ = backup.EnforceLocalRetention(a.Cfg.DataDir, cfg.Retention)

	out, err := a.Store.GetBackupExecution(ctx, teamID, execRow.ID)
	if err != nil {
		list, lerr := a.Store.ListInstanceBackupExecutions(ctx, 5)
		if lerr == nil {
			for i := range list {
				if list[i].ID == execRow.ID {
					return &list[i], nil
				}
			}
		}
		execRow.Filename = filename
		execRow.Status = "finished"
		execRow.SizeBytes = size
		return execRow, nil
	}
	return out, nil
}
