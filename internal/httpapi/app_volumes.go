package httpapi

import (
	"fmt"
	"net/http"
	"path"
	"path/filepath"
	"strings"

	"github.com/dockfin/dockfin/internal/database"
	"github.com/dockfin/dockfin/internal/sshx"
	"github.com/dockfin/dockfin/internal/storagemount"
	"github.com/dockfin/dockfin/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"
)

func (a *API) handleListAppVolumes(w http.ResponseWriter, r *http.Request) {
	a.handleListVolumes(w, r, "application", "appID")
}

func (a *API) handleListDatabaseVolumes(w http.ResponseWriter, r *http.Request) {
	a.handleListVolumes(w, r, "database", "dbID")
}

func (a *API) handleListVolumes(w http.ResponseWriter, r *http.Request, resourceType, param string) {
	id, err := uuid.Parse(chi.URLParam(r, param))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	teamID := currentTeamID(r)
	if err := a.ensureVolumeResource(r, teamID, resourceType, id, false); err != nil {
		writeStored(w, err)
		return
	}
	list, err := a.Store.ListVolumes(r.Context(), teamID, resourceType, id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if list == nil {
		list = []store.Volume{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"volumes": list})
}

func (a *API) handleGetAppVolume(w http.ResponseWriter, r *http.Request) {
	a.handleGetVolume(w, r, "application", "appID")
}

func (a *API) handleGetDatabaseVolume(w http.ResponseWriter, r *http.Request) {
	a.handleGetVolume(w, r, "database", "dbID")
}

func (a *API) handleGetVolume(w http.ResponseWriter, r *http.Request, resourceType, param string) {
	id, volID, ok := volumeIDs(w, r, param)
	if !ok {
		return
	}
	teamID := currentTeamID(r)
	if err := a.ensureVolumeResource(r, teamID, resourceType, id, false); err != nil {
		writeStored(w, err)
		return
	}
	v, err := a.ownedVolume(r, teamID, resourceType, id, volID)
	if err != nil {
		writeStored(w, err)
		return
	}
	content := ""
	if v.IsFile {
		content, err = a.Store.VolumeContent(r.Context(), teamID, v.ID)
		if err != nil {
			mapStoreErr(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id": v.ID, "name": v.Name, "mount_path": v.MountPath, "host_path": v.HostPath,
		"is_file": v.IsFile, "has_content": v.HasContent, "content": content,
	})
}

func (a *API) handleUpsertAppVolume(w http.ResponseWriter, r *http.Request) {
	a.handleUpsertVolume(w, r, "application", "appID")
}

func (a *API) handleUpsertDatabaseVolume(w http.ResponseWriter, r *http.Request) {
	a.handleUpsertVolume(w, r, "database", "dbID")
}

func (a *API) handleUpsertVolume(w http.ResponseWriter, r *http.Request, resourceType, param string) {
	id, err := uuid.Parse(chi.URLParam(r, param))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	teamID := currentTeamID(r)
	if err := a.ensureVolumeResource(r, teamID, resourceType, id, true); err != nil {
		writeStored(w, err)
		return
	}
	var body struct {
		Name      string  `json:"name"`
		MountPath string  `json:"mount_path"`
		HostPath  string  `json:"host_path"`
		IsFile    bool    `json:"is_file"`
		Content   *string `json:"content"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := storagemount.ValidateMountPath(body.MountPath); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if resourceType == "database" {
		db, err := a.Store.GetDatabase(r.Context(), teamID, id)
		if err != nil {
			mapStoreErr(w, err)
			return
		}
		if database.CoversDataDir(db.Engine, body.MountPath) {
			writeError(w, http.StatusBadRequest, "mount path covers the database data directory")
			return
		}
	}
	if body.IsFile && body.Content != nil {
		if err := storagemount.ValidateContent(*body.Content); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	host, err := storagemount.ResolveHost(resourceType, id.String(), body.Name, body.HostPath, body.IsFile)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	body.HostPath = host
	if err := validateVolumeHostPath(body.HostPath, isTeamAdmin(r)); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	existing, err := a.Store.ListVolumes(r.Context(), teamID, resourceType, id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	for _, v := range existing {
		if v.Name == strings.TrimSpace(body.Name) {
			continue
		}
		if v.MountPath == strings.TrimSpace(body.MountPath) {
			writeError(w, http.StatusBadRequest, "another mount already uses "+body.MountPath)
			return
		}
		other := strings.TrimSpace(v.HostPath)
		if other == "" {
			other = storagemount.DefaultHost(resourceType, id.String(), v.Name, v.IsFile)
		}
		if path.Clean(other) == body.HostPath {
			writeError(w, http.StatusBadRequest, "another mount already uses this host path")
			return
		}
	}
	v, err := a.Store.UpsertVolume(r.Context(), teamID, resourceType, id, body.Name, body.MountPath, body.HostPath, body.IsFile, body.Content)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, v)
}

func (a *API) handleDeleteAppVolume(w http.ResponseWriter, r *http.Request) {
	a.handleDeleteVolume(w, r, "application", "appID")
}

func (a *API) handleDeleteDatabaseVolume(w http.ResponseWriter, r *http.Request) {
	a.handleDeleteVolume(w, r, "database", "dbID")
}

func (a *API) handleDeleteVolume(w http.ResponseWriter, r *http.Request, resourceType, param string) {
	id, volID, ok := volumeIDs(w, r, param)
	if !ok {
		return
	}
	teamID := currentTeamID(r)
	if err := a.ensureVolumeResource(r, teamID, resourceType, id, false); err != nil {
		writeStored(w, err)
		return
	}
	v, err := a.ownedVolume(r, teamID, resourceType, id, volID)
	if err != nil {
		writeStored(w, err)
		return
	}
	if v.IsFile && storagemount.ManagedFile(resourceType, id.String(), v.HostPath) {
		if err := a.removeManagedVolumeFile(r, teamID, resourceType, id, v.HostPath); err != nil {
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
	}
	if err := a.Store.DeleteVolume(r.Context(), teamID, volID); err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (a *API) databaseVolumeMounts(r *http.Request, client *ssh.Client, teamID uuid.UUID, db *store.Database) ([]database.BindMount, error) {
	vols, err := a.Store.ListVolumes(r.Context(), teamID, "database", db.ID)
	if err != nil {
		return nil, err
	}
	var out []database.BindMount
	for _, v := range vols {
		host, err := storagemount.ResolveHost("database", db.ID.String(), v.Name, v.HostPath, v.IsFile)
		if err != nil {
			return nil, err
		}
		if database.CoversDataDir(db.Engine, v.MountPath) {
			return nil, fmt.Errorf("mount path %s covers the database data directory", v.MountPath)
		}
		content := ""
		if v.IsFile {
			content, err = a.Store.VolumeContent(r.Context(), teamID, v.ID)
			if err != nil {
				return nil, err
			}
		}
		if err := storagemount.Prepare(client, host, v.IsFile, content); err != nil {
			return nil, err
		}
		mount := v.MountPath
		if !strings.HasPrefix(mount, "/") {
			mount = "/" + mount
		}
		out = append(out, database.BindMount{Host: host, Container: mount})
	}
	return out, nil
}

func (a *API) ensureVolumeResource(r *http.Request, teamID uuid.UUID, resourceType string, id uuid.UUID, writing bool) error {
	switch resourceType {
	case "application":
		app, err := a.Store.GetApplication(r.Context(), teamID, id)
		if err != nil {
			return err
		}
		if writing && app.BuildPack == "dockercompose" {
			return errString("edit volumes in the compose file for dockercompose apps")
		}
		return nil
	case "database":
		_, err := a.Store.GetDatabase(r.Context(), teamID, id)
		return err
	default:
		return errString("unsupported resource")
	}
}

func (a *API) ownedVolume(r *http.Request, teamID uuid.UUID, resourceType string, resourceID, volumeID uuid.UUID) (*store.Volume, error) {
	v, err := a.Store.GetVolume(r.Context(), teamID, volumeID)
	if err != nil {
		return nil, err
	}
	if v.ResourceType != resourceType || v.ResourceID != resourceID {
		return nil, store.ErrNotFound
	}
	return v, nil
}

func (a *API) removeManagedVolumeFile(r *http.Request, teamID uuid.UUID, resourceType string, resourceID uuid.UUID, host string) error {
	var destID *uuid.UUID
	switch resourceType {
	case "application":
		app, err := a.Store.GetApplication(r.Context(), teamID, resourceID)
		if err != nil {
			return err
		}
		destID = app.DestinationID
	case "database":
		db, err := a.Store.GetDatabase(r.Context(), teamID, resourceID)
		if err != nil {
			return err
		}
		destID = db.DestinationID
	}
	if destID == nil {
		return nil
	}
	dest, err := a.Store.GetDestination(r.Context(), teamID, *destID)
	if err != nil {
		return err
	}
	client, err := a.dialServer(r, dest.ServerID)
	if err != nil {
		return err
	}
	canonical, kind, err := storagemount.Inspect(client, host)
	if err != nil {
		return err
	}
	if err := storagemount.AcceptCanonical(host, canonical); err != nil {
		return err
	}
	if kind == "missing" {
		return nil
	}
	if kind != "file" {
		return fmt.Errorf("%s is not a file", host)
	}
	_, errOut, err := sshx.RunArgs(client, "rm", "-f", host)
	if err != nil {
		return fmt.Errorf("remove %s: %v %s", host, err, errOut)
	}
	return nil
}

func volumeIDs(w http.ResponseWriter, r *http.Request, param string) (uuid.UUID, uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, param))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return uuid.Nil, uuid.Nil, false
	}
	volID, err := uuid.Parse(chi.URLParam(r, "volumeID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid volume id")
		return uuid.Nil, uuid.Nil, false
	}
	return id, volID, true
}

func writeStored(w http.ResponseWriter, err error) {
	if _, ok := err.(errString); ok {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	mapStoreErr(w, err)
}

// validateVolumeHostPath blocks path traversal, docker.sock, and (for members)
// bind-mounts outside /data/dockfin.
func validateVolumeHostPath(hostPath string, admin bool) error {
	p := strings.TrimSpace(hostPath)
	if p == "" {
		return fmt.Errorf("host_path required")
	}
	if strings.Contains(p, "..") || strings.ContainsAny(p, ":\n\r") {
		return fmt.Errorf("host_path must not contain .. or a colon")
	}
	cleaned := filepath.Clean(p)
	if !filepath.IsAbs(cleaned) {
		return fmt.Errorf("host_path must be an absolute path")
	}
	if cleaned == "/" {
		return fmt.Errorf("host_path must not be the root directory")
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
