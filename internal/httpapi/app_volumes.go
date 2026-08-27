package httpapi

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (a *API) handleListAppVolumes(w http.ResponseWriter, r *http.Request) {
	appID, err := uuid.Parse(chi.URLParam(r, "appID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	teamID := currentTeamID(r)
	if _, err := a.Store.GetApplication(r.Context(), teamID, appID); err != nil {
		mapStoreErr(w, err)
		return
	}
	list, err := a.Store.ListVolumes(r.Context(), teamID, "application", appID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"volumes": list})
}

func (a *API) handleUpsertAppVolume(w http.ResponseWriter, r *http.Request) {
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
	if app.BuildPack == "dockercompose" {
		writeError(w, http.StatusBadRequest, "edit volumes in the compose file for dockercompose apps")
		return
	}
	var body struct {
		Name      string `json:"name"`
		MountPath string `json:"mount_path"`
		HostPath  string `json:"host_path"`
		IsFile    bool   `json:"is_file"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if strings.TrimSpace(body.HostPath) == "" {
		safe := strings.Map(func(r rune) rune {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
				return r
			}
			return '-'
		}, body.Name)
		body.HostPath = "/data/dockfin/applications/" + appID.String() + "/volumes/" + safe
	}
	if err := validateVolumeHostPath(body.HostPath, isTeamAdmin(r)); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	v, err := a.Store.UpsertVolume(r.Context(), teamID, "application", appID, body.Name, body.MountPath, body.HostPath, body.IsFile)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, v)
}

func (a *API) handleDeleteAppVolume(w http.ResponseWriter, r *http.Request) {
	appID, err := uuid.Parse(chi.URLParam(r, "appID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	volID, err := uuid.Parse(chi.URLParam(r, "volumeID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid volume id")
		return
	}
	teamID := currentTeamID(r)
	if _, err := a.Store.GetApplication(r.Context(), teamID, appID); err != nil {
		mapStoreErr(w, err)
		return
	}
	v, err := a.Store.GetVolume(r.Context(), teamID, volID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if v.ResourceType != "application" || v.ResourceID != appID {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if err := a.Store.DeleteVolume(r.Context(), teamID, volID); err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// validateVolumeHostPath blocks path traversal, docker.sock, and (for members)
// bind-mounts outside /data/dockfin.
func validateVolumeHostPath(hostPath string, admin bool) error {
	p := strings.TrimSpace(hostPath)
	if p == "" {
		return fmt.Errorf("host_path required")
	}
	if strings.Contains(p, "..") {
		return fmt.Errorf("host_path must not contain ..")
	}
	cleaned := filepath.Clean(p)
	if !filepath.IsAbs(cleaned) {
		return fmt.Errorf("host_path must be an absolute path")
	}
	lower := strings.ToLower(cleaned)
	if lower == "/var/run/docker.sock" || strings.HasSuffix(lower, "/docker.sock") {
		return fmt.Errorf("docker.sock bind mounts are not allowed")
	}
	for _, prefix := range []string{"/etc", "/root", "/proc", "/sys", "/boot"} {
		if cleaned == prefix || strings.HasPrefix(cleaned, prefix+"/") {
			if !admin {
				return fmt.Errorf("host path %s requires admin or owner role", cleaned)
			}
		}
	}
	if !admin && cleaned != "/data/dockfin" && !strings.HasPrefix(cleaned, "/data/dockfin/") {
		return fmt.Errorf("custom host paths outside /data/dockfin require admin or owner role")
	}
	return nil
}
