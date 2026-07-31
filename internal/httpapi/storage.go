package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/goolify/goolify/internal/backup"
	"github.com/goolify/goolify/internal/sshx"
	"github.com/goolify/goolify/internal/store"
	"golang.org/x/crypto/ssh"
)

func (a *API) handleListS3Storages(w http.ResponseWriter, r *http.Request) {
	list, err := a.Store.ListS3Storages(r.Context(), currentTeamID(r))
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"s3_storages": list})
}

func (a *API) handleCreateS3Storage(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name      string `json:"name"`
		Endpoint  string `json:"endpoint"`
		Bucket    string `json:"bucket"`
		Region    string `json:"region"`
		AccessKey string `json:"access_key"`
		SecretKey string `json:"secret_key"`
		PathStyle *bool  `json:"path_style"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if body.Name == "" || body.Endpoint == "" || body.Bucket == "" || body.AccessKey == "" || body.SecretKey == "" {
		writeError(w, http.StatusBadRequest, "name, endpoint, bucket, access_key, secret_key required")
		return
	}
	pathStyle := true
	if body.PathStyle != nil {
		pathStyle = *body.PathStyle
	}
	akEnc, err := a.Store.Box.EncryptString(body.AccessKey)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	skEnc, err := a.Store.Box.EncryptString(body.SecretKey)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	st, err := a.Store.CreateS3Storage(r.Context(), currentTeamID(r), body.Name, body.Endpoint, body.Bucket, body.Region, akEnc, skEnc, pathStyle)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, st)
}

func (a *API) handleGetS3Storage(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "storageID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	st, err := a.Store.GetS3Storage(r.Context(), currentTeamID(r), id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (a *API) handleDeleteS3Storage(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "storageID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := a.Store.DeleteS3Storage(r.Context(), currentTeamID(r), id); err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (a *API) handleListScheduledBackups(w http.ResponseWriter, r *http.Request) {
	list, err := a.Store.ListScheduledBackups(r.Context(), currentTeamID(r))
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"scheduled_backups": list})
}

func (a *API) handleCreateScheduledBackup(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ResourceType string `json:"resource_type"`
		ResourceID   string `json:"resource_id"`
		S3StorageID  string `json:"s3_storage_id"`
		Frequency    string `json:"frequency"`
		Retention    int    `json:"retention"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	rid, err := uuid.Parse(body.ResourceID)
	if err != nil || body.ResourceType == "" {
		writeError(w, http.StatusBadRequest, "resource_type and resource_id required")
		return
	}
	teamID := currentTeamID(r)
	switch body.ResourceType {
	case "database":
		if _, err := a.Store.GetDatabase(r.Context(), teamID, rid); err != nil {
			mapStoreErr(w, err)
			return
		}
	case "application":
		if _, err := a.Store.GetApplication(r.Context(), teamID, rid); err != nil {
			mapStoreErr(w, err)
			return
		}
	default:
		writeError(w, http.StatusBadRequest, "unsupported resource_type")
		return
	}
	var s3ID *uuid.UUID
	if body.S3StorageID != "" {
		id, err := uuid.Parse(body.S3StorageID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid s3_storage_id")
			return
		}
		if _, err := a.Store.GetS3Storage(r.Context(), teamID, id); err != nil {
			mapStoreErr(w, err)
			return
		}
		s3ID = &id
	}
	b, err := a.Store.CreateScheduledBackup(r.Context(), teamID, body.ResourceType, rid, s3ID, body.Frequency, body.Retention)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, b)
}

func (a *API) handleListDatabaseBackups(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "dbID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	teamID := currentTeamID(r)
	if _, err := a.Store.GetDatabase(r.Context(), teamID, id); err != nil {
		mapStoreErr(w, err)
		return
	}
	list, err := a.Store.ListBackupExecutions(r.Context(), teamID, "database", id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if list == nil {
		list = []store.BackupExecution{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"backup_executions": list})
}

func (a *API) handleRunDatabaseBackup(w http.ResponseWriter, r *http.Request) {
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
	if db.Engine != "postgresql" {
		writeError(w, http.StatusBadRequest, "manual backup currently supports postgresql only")
		return
	}
	client, password, err := a.openDatabaseSSH(r, db)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			mapStoreErr(w, err)
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	filename := backup.DefaultFilename(db.Engine, id.String())
	path := backup.DumpPath(filename)
	exec, err := a.Store.CreateBackupExecution(r.Context(), teamID, "database", id, filename)
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
	container := "goolify-db-" + id.String()
	if err := backup.DumpPostgres(client, container, password, path); err != nil {
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
	out, err := a.Store.GetBackupExecution(r.Context(), teamID, exec.ID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) handleRestoreDatabaseBackup(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "dbID"))
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
	db, err := a.Store.GetDatabase(r.Context(), teamID, id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if db.Engine != "postgresql" {
		writeError(w, http.StatusBadRequest, "restore currently supports postgresql only")
		return
	}
	filename := body.Filename
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
		if ex.ResourceType != "database" || ex.ResourceID != id {
			writeError(w, http.StatusBadRequest, "execution does not belong to this database")
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
	client, password, err := a.openDatabaseSSH(r, db)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			mapStoreErr(w, err)
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	container := "goolify-db-" + id.String()
	if err := backup.RestorePostgres(client, container, password, backup.DumpPath(filename)); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "restored", "filename": filename})
}

func (a *API) openDatabaseSSH(r *http.Request, db *store.Database) (*ssh.Client, string, error) {
	if db.DestinationID == nil {
		return nil, "", fmt.Errorf("database has no destination")
	}
	teamID := currentTeamID(r)
	dest, err := a.Store.GetDestination(r.Context(), teamID, *db.DestinationID)
	if err != nil {
		return nil, "", err
	}
	client, err := a.dialServer(r, dest.ServerID)
	if err != nil {
		return nil, "", err
	}
	password, err := a.resolveDatabasePassword(r, teamID, db.ID)
	if err != nil {
		return nil, "", err
	}
	return client, password, nil
}

func safeBackupFilename(s string) bool {
	if s == "" || len(s) > 200 || strings.Contains(s, "..") {
		return false
	}
	for _, c := range s {
		ok := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') ||
			c == '-' || c == '_' || c == '.'
		if !ok {
			return false
		}
	}
	return true
}

func (a *API) resolveDatabasePassword(r *http.Request, teamID, id uuid.UUID) (string, error) {
	enc, err := a.Store.GetDatabaseCredentials(r.Context(), teamID, id)
	if err != nil {
		return "", err
	}
	if enc == "" {
		return "", nil
	}
	plain, err := a.Store.Box.Decrypt(enc)
	if err != nil {
		return "", fmt.Errorf("decrypt credentials")
	}
	var creds map[string]string
	if json.Unmarshal(plain, &creds) == nil {
		return creds["password"], nil
	}
	return "", nil
}

func (a *API) handleServerExec(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "serverID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body struct {
		Command string `json:"command"`
	}
	if err := decodeJSON(r, &body); err != nil || body.Command == "" {
		writeError(w, http.StatusBadRequest, "command required")
		return
	}
	// Non-interactive only — reject obvious interactive shells
	cmd := body.Command
	if len(cmd) > 4096 {
		writeError(w, http.StatusBadRequest, "command too long")
		return
	}
	client, err := a.dialServer(r, id)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	stdout, stderr, err := sshx.Run(client, cmd)
	status := http.StatusOK
	resp := map[string]any{
		"stdout": stdout,
		"stderr": stderr,
	}
	if err != nil {
		status = http.StatusOK // still return output
		resp["error"] = err.Error()
		resp["exit_error"] = true
	}
	writeJSON(w, status, resp)
}

func (a *API) handleListPreviews(w http.ResponseWriter, r *http.Request) {
	appID, err := uuid.Parse(chi.URLParam(r, "appID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	list, err := a.Store.ListPreviews(r.Context(), currentTeamID(r), appID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if list == nil {
		list = []store.ApplicationPreview{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"previews": list})
}

func (a *API) handleDeletePreview(w http.ResponseWriter, r *http.Request) {
	appID, err := uuid.Parse(chi.URLParam(r, "appID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var prID int
	if _, err := fmt.Sscanf(chi.URLParam(r, "prID"), "%d", &prID); err != nil || prID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid pr id")
		return
	}
	teamID := currentTeamID(r)
	app, err := a.Store.GetApplication(r.Context(), teamID, appID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	// Best-effort: remove preview container / compose project so Traefik routes disappear.
	if app.DestinationID != nil {
		if dest, err := a.Store.GetDestination(r.Context(), teamID, *app.DestinationID); err == nil {
			if client, err := a.dialServer(r, dest.ServerID); err == nil {
				cname := fmt.Sprintf("goolify-%s-pr-%d", appID.String(), prID)
				_, _, _ = sshx.RunArgs(client, "docker", "rm", "-f", cname)
				project := fmt.Sprintf("goolify-%s-pr-%d", appID.String()[:8], prID)
				workdir := fmt.Sprintf("/data/goolify/applications/%s-pr-%d", appID.String(), prID)
				composePath := workdir + "/src/docker-compose.yaml"
				// Try common compose filenames; ignore failures.
				for _, f := range []string{
					composePath,
					workdir + "/src/docker-compose.yml",
					workdir + "/src/compose.yaml",
				} {
					_, _, _ = sshx.RunArgs(client, "docker", "compose", "-p", project, "-f", f, "down", "--remove-orphans", "-v")
				}
				_, _, _ = sshx.RunArgs(client, "rm", "-rf", workdir)
			}
		}
	}
	if err := a.Store.DeletePreview(r.Context(), teamID, appID, prID); err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
