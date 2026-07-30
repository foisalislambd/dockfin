package httpapi

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/goolify/goolify/internal/sshx"
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
	var s3ID *uuid.UUID
	if body.S3StorageID != "" {
		id, err := uuid.Parse(body.S3StorageID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid s3_storage_id")
			return
		}
		if _, err := a.Store.GetS3Storage(r.Context(), currentTeamID(r), id); err != nil {
			mapStoreErr(w, err)
			return
		}
		s3ID = &id
	}
	b, err := a.Store.CreateScheduledBackup(r.Context(), currentTeamID(r), body.ResourceType, rid, s3ID, body.Frequency, body.Retention)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, b)
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
	if err := a.Store.DeletePreview(r.Context(), currentTeamID(r), appID, prID); err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
