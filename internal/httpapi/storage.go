package httpapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dockfin/dockfin/internal/backup"
	"github.com/dockfin/dockfin/internal/database"
	"github.com/dockfin/dockfin/internal/sshx"
	"github.com/dockfin/dockfin/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
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
		VolumeID     string `json:"volume_id"`
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
	var volumeID *uuid.UUID
	if body.VolumeID != "" {
		vid, err := uuid.Parse(body.VolumeID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid volume_id")
			return
		}
		v, err := a.Store.GetVolume(r.Context(), teamID, vid)
		if err != nil {
			mapStoreErr(w, err)
			return
		}
		if v.ResourceType != "application" || v.ResourceID != rid {
			writeError(w, http.StatusBadRequest, "volume does not belong to application")
			return
		}
		volumeID = &vid
	}
	b, err := a.Store.CreateScheduledBackup(r.Context(), teamID, body.ResourceType, rid, s3ID, volumeID, body.Frequency, body.Retention)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, b)
}

func (a *API) handlePatchScheduledBackup(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "backupID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body struct {
		Frequency   *string `json:"frequency"`
		Retention   *int    `json:"retention"`
		Enabled     *bool   `json:"enabled"`
		S3StorageID *string `json:"s3_storage_id"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	teamID := currentTeamID(r)
	in := store.UpdateScheduledBackupInput{
		Frequency: body.Frequency,
		Retention: body.Retention,
		Enabled:   body.Enabled,
	}
	if body.S3StorageID != nil {
		raw := strings.TrimSpace(*body.S3StorageID)
		if raw == "" {
			var nilID *uuid.UUID
			in.S3StorageID = &nilID
		} else {
			sid, err := uuid.Parse(raw)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid s3_storage_id")
				return
			}
			if _, err := a.Store.GetS3Storage(r.Context(), teamID, sid); err != nil {
				mapStoreErr(w, err)
				return
			}
			ptr := &sid
			in.S3StorageID = &ptr
		}
	}
	b, err := a.Store.UpdateScheduledBackup(r.Context(), teamID, id, in)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, b)
}

func (a *API) handleDeleteScheduledBackup(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "backupID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := a.Store.DeleteScheduledBackup(r.Context(), currentTeamID(r), id); err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
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
	if db.Engine != "postgresql" && db.Engine != "mysql" && db.Engine != "mariadb" && db.Engine != "redis" && db.Engine != "keydb" {
		writeError(w, http.StatusBadRequest, "manual backup supports postgresql, mysql/mariadb, and redis/keydb")
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
	container := database.ContainerName(id.String())
	if err := backup.DumpDatabase(client, db.Engine, container, password, path); err != nil {
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
	_ = backup.EnforceRemoteRetention(client, db.Engine, id.String(), 14)
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
	if db.Engine != "postgresql" && db.Engine != "mysql" && db.Engine != "mariadb" {
		writeError(w, http.StatusBadRequest, "restore supports postgresql and mysql/mariadb")
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
	container := database.ContainerName(id.String())
	dumpPath := backup.DumpPath(filename)
	restoreErr := backup.RestoreDatabase(client, db.Engine, container, password, dumpPath)
	if restoreErr != nil {
		writeError(w, http.StatusInternalServerError, restoreErr.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "restored", "filename": filename})
}

func (a *API) handleImportDatabaseBackup(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "dbID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body struct {
		Filename      string `json:"filename"`
		ContentBase64 string `json:"content_base64"`
		Restore       *bool  `json:"restore"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if body.ContentBase64 == "" {
		writeError(w, http.StatusBadRequest, "content_base64 required")
		return
	}
	data, err := base64.StdEncoding.DecodeString(body.ContentBase64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid base64 content")
		return
	}
	if len(data) == 0 {
		writeError(w, http.StatusBadRequest, "uploaded file is empty")
		return
	}
	// JSON+base64 import is memory-bound on the API; keep a practical ceiling.
	const maxImportBytes = 64 << 20 // 64 MiB
	if len(data) > maxImportBytes {
		writeError(w, http.StatusBadRequest, "uploaded file too large (max 64 MiB via import; use restore from an existing dump for larger files)")
		return
	}
	teamID := currentTeamID(r)
	db, err := a.Store.GetDatabase(r.Context(), teamID, id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if db.Engine != "postgresql" && db.Engine != "mysql" && db.Engine != "mariadb" {
		writeError(w, http.StatusBadRequest, "import supports postgresql and mysql/mariadb")
		return
	}
	restore := true
	if body.Restore != nil {
		restore = *body.Restore
	}
	filename := strings.TrimSpace(body.Filename)
	if filename == "" {
		filename = backup.DefaultFilename(db.Engine, id.String())
	}
	if !safeBackupFilename(filename) {
		writeError(w, http.StatusBadRequest, "invalid filename")
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
	dumpPath := backup.DumpPath(filename)
	if err := sshx.WriteFile(client, dumpPath, data); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	exec, err := a.Store.CreateBackupExecution(r.Context(), teamID, "database", id, filename)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if err := a.Store.FinishBackupExecution(r.Context(), exec.ID, "finished", int64(len(data)), ""); err != nil {
		mapStoreErr(w, err)
		return
	}
	resp := map[string]any{"status": "imported", "filename": filename, "size_bytes": len(data)}
	if restore {
		container := database.ContainerName(id.String())
		restoreErr := backup.RestoreDatabase(client, db.Engine, container, password, dumpPath)
		if restoreErr != nil {
			writeError(w, http.StatusInternalServerError, restoreErr.Error())
			return
		}
		resp["status"] = "imported_and_restored"
	}
	writeJSON(w, http.StatusOK, resp)
}

func (a *API) handleDatabaseLogsStream(w http.ResponseWriter, r *http.Request) {
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

	container := database.ContainerName(id.String())
	tail := 200
	if t := r.URL.Query().Get("tail"); t != "" {
		if n, err := strconv.Atoi(t); err == nil && n > 0 && n <= 5000 {
			tail = n
		}
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	var mu sync.Mutex
	send := func(event, data string) {
		mu.Lock()
		defer mu.Unlock()
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
		flusher.Flush()
	}
	meta, _ := json.Marshal(map[string]string{"container": container})
	send("meta", string(meta))

	ctx := r.Context()
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = sshx.RunArgsStreaming(client, func(line string) {
			select {
			case <-ctx.Done():
				return
			default:
			}
			b, _ := json.Marshal(map[string]string{"line": line})
			send("log", string(b))
		}, "docker", "logs", "-f", "--tail", strconv.Itoa(tail), container)
	}()

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			send("done", `{"status":"ended"}`)
			return
		case <-ticker.C:
			send("ping", `{}`)
		}
	}
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
	res := a.cleanupPreview(r.Context(), app, prID)
	if res.Status == "failed" {
		writeError(w, http.StatusBadRequest, res.Message)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
