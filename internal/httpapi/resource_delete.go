package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/dockfin/dockfin/internal/database"
	"github.com/dockfin/dockfin/internal/sshx"
	"github.com/dockfin/dockfin/internal/store"
	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"
)

func refuseWhileBusy(w http.ResponseWriter, status string) bool {
	switch status {
	case "stopping", "starting", "restarting", "deleting":
		writeError(w, http.StatusConflict, "wait until "+status+" finishes")
		return true
	default:
		return false
	}
}

func (a *API) scheduleDelete(ctx context.Context, teamID uuid.UUID, kind string, id uuid.UUID, opts DeleteOptions, applicationID uuid.UUID, prID int) (*store.BackgroundJob, error) {
	serverID, err := a.deleteServerID(ctx, teamID, kind, id)
	if err != nil {
		return nil, err
	}
	payload := store.DeletionPayload{
		ResourceType:         kind,
		ResourceID:           id.String(),
		ServerID:             serverID,
		DeleteVolumes:        opts.volumes(),
		DeleteConfigurations: opts.configurations(),
		DeleteNetworks:       opts.networks(),
		DockerCleanup:        opts.dockerCleanup(),
		PullRequestID:        prID,
	}
	if applicationID != uuid.Nil {
		payload.ApplicationID = applicationID.String()
	}
	return a.Store.MarkDeleting(ctx, teamID, kind, id, store.MarshalDeletionPayload(payload))
}

func (a *API) deleteServerID(ctx context.Context, teamID uuid.UUID, kind string, id uuid.UUID) (string, error) {
	switch kind {
	case "application":
		app, err := a.Store.GetApplicationAny(ctx, teamID, id)
		if err != nil || app.DestinationID == nil {
			return "", err
		}
		dest, err := a.Store.GetDestination(ctx, teamID, *app.DestinationID)
		if err != nil {
			return "", err
		}
		return dest.ServerID.String(), nil
	case "service":
		svc, err := a.Store.GetServiceAny(ctx, teamID, id)
		if err != nil {
			return "", err
		}
		serverID, _, err := a.resolveServiceTarget(ctx, teamID, svc)
		if err != nil {
			if svc.ServerID == nil && svc.DestinationID == nil {
				return "", nil
			}
			return "", err
		}
		return serverID.String(), nil
	case "database":
		db, err := a.Store.GetDatabaseAny(ctx, teamID, id)
		if err != nil || db.DestinationID == nil {
			return "", err
		}
		dest, err := a.Store.GetDestination(ctx, teamID, *db.DestinationID)
		if err != nil {
			return "", err
		}
		return dest.ServerID.String(), nil
	case "preview":
		preview, err := a.Store.GetPreviewByID(ctx, teamID, id)
		if err != nil {
			return "", err
		}
		app, err := a.Store.GetApplicationAny(ctx, teamID, preview.ApplicationID)
		if err != nil || app.DestinationID == nil {
			return "", err
		}
		dest, err := a.Store.GetDestination(ctx, teamID, *app.DestinationID)
		if err != nil {
			return "", err
		}
		return dest.ServerID.String(), nil
	default:
		return "", fmt.Errorf("unknown resource type")
	}
}

func (a *API) jobResourceDelete(ctx context.Context, job *store.BackgroundJob) ([]byte, error) {
	var p store.DeletionPayload
	if err := json.Unmarshal(job.Payload, &p); err != nil {
		return nil, err
	}
	id, err := uuid.Parse(p.ResourceID)
	if err != nil {
		return nil, err
	}
	switch p.ResourceType {
	case "application":
		err = a.cleanupApplication(ctx, job.TeamID, id, p)
	case "service":
		err = a.cleanupService(ctx, job.TeamID, id, p)
	case "database":
		err = a.cleanupDatabase(ctx, job.TeamID, id, p)
	case "preview":
		err = a.cleanupPreviewJob(ctx, job.TeamID, id, p)
	default:
		err = fmt.Errorf("unknown resource type")
	}
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]string{"status": "deleted", "resource_type": p.ResourceType, "resource_id": id.String()})
}

func (a *API) noteDeleteFailure(job *store.BackgroundJob, cause error) {
	var p store.DeletionPayload
	if json.Unmarshal(job.Payload, &p) != nil {
		return
	}
	id, err := uuid.Parse(p.ResourceID)
	if err != nil {
		return
	}
	_ = a.Store.SetDeleteError(context.Background(), job.TeamID, p.ResourceType, id, cause.Error())
}

func (a *API) cleanupApplication(ctx context.Context, teamID, id uuid.UUID, p store.DeletionPayload) error {
	app, err := a.Store.GetApplicationAny(ctx, teamID, id)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	client, err := a.dialPayloadServer(ctx, teamID, p.ServerID)
	if err != nil {
		return err
	}
	if client != nil {
		cname := "dockfin-" + id.String()
		if p.DeleteVolumes {
			if err := runChecked(ctx, client, "docker", "rm", "-f", "-v", cname); err != nil {
				return err
			}
		} else if err := runChecked(ctx, client, "docker", "rm", "-f", cname); err != nil {
			return err
		}
		if p.DeleteConfigurations {
			if err := runChecked(ctx, client, "rm", "-rf", "/data/dockfin/applications/"+id.String()); err != nil {
				return err
			}
		}
		if p.DeleteNetworks {
			if err := removeResourceNetwork(ctx, client, id.String()); err != nil {
				return err
			}
		}
		previews, _ := a.Store.ListPreviewsIncludingDeleted(ctx, teamID, id)
		for _, preview := range previews {
			if err := a.cleanupPreviewRemoteChecked(ctx, client, app, preview.PullRequestID, p); err != nil {
				return err
			}
			_ = a.syncManagedDNS(ctx, teamID, "application_preview", preview.ID, nil, nil)
		}
		if p.DockerCleanup {
			_ = runChecked(ctx, client, "docker", "image", "prune", "-f")
			_ = runChecked(ctx, client, "docker", "builder", "prune", "-f")
		}
	}
	_ = a.syncManagedDNS(ctx, teamID, "application", id, nil, nil)
	if err := a.Store.DeleteApplication(ctx, teamID, id); err != nil && !errors.Is(err, store.ErrNotFound) {
		return err
	}
	return nil
}

func (a *API) cleanupService(ctx context.Context, teamID, id uuid.UUID, p store.DeletionPayload) error {
	if _, err := a.Store.GetServiceAny(ctx, teamID, id); errors.Is(err, store.ErrNotFound) {
		return nil
	} else if err != nil {
		return err
	}
	client, err := a.dialPayloadServer(ctx, teamID, p.ServerID)
	if err != nil {
		return err
	}
	if client != nil {
		remoteDir := "/data/dockfin/services/" + id.String()
		composePath := remoteDir + "/docker-compose.yml"
		project := "dockfin-svc-" + id.String()[:8]
		args := []string{"docker", "compose", "-p", project, "-f", composePath, "down", "--remove-orphans"}
		if p.DeleteVolumes {
			args = append(args, "-v")
		}
		if err := runChecked(ctx, client, args...); err != nil {
			return err
		}
		if p.DeleteConfigurations {
			if err := runChecked(ctx, client, "rm", "-rf", remoteDir); err != nil {
				return err
			}
		}
		if p.DeleteNetworks {
			if err := removeResourceNetwork(ctx, client, id.String()); err != nil {
				return err
			}
		}
		if p.DockerCleanup {
			_ = runChecked(ctx, client, "docker", "image", "prune", "-f")
			_ = runChecked(ctx, client, "docker", "builder", "prune", "-f")
		}
	}
	_ = a.syncManagedDNS(ctx, teamID, "service", id, nil, nil)
	if err := a.Store.DeleteService(ctx, teamID, id); err != nil && !errors.Is(err, store.ErrNotFound) {
		return err
	}
	return nil
}

func (a *API) cleanupDatabase(ctx context.Context, teamID, id uuid.UUID, p store.DeletionPayload) error {
	if _, err := a.Store.GetDatabaseAny(ctx, teamID, id); errors.Is(err, store.ErrNotFound) {
		return nil
	} else if err != nil {
		return err
	}
	client, err := a.dialPayloadServer(ctx, teamID, p.ServerID)
	if err != nil {
		return err
	}
	if client != nil {
		if err := runChecked(ctx, client, "docker", "rm", "-f", database.ContainerName(id.String())); err != nil {
			return err
		}
		if p.DeleteVolumes {
			if err := database.RemoveData(client, id.String()); err != nil && !dockerMissingOK(err.Error()) {
				return err
			}
		}
		if p.DeleteConfigurations {
			_ = database.RemoveTLS(client, id.String())
			_ = database.RemoveInit(client, id.String())
		}
		if p.DeleteNetworks {
			if err := removeResourceNetwork(ctx, client, id.String()); err != nil {
				return err
			}
		}
		if p.DockerCleanup {
			_ = runChecked(ctx, client, "docker", "image", "prune", "-f")
			_ = runChecked(ctx, client, "docker", "builder", "prune", "-f")
		}
	}
	if err := a.Store.DeleteDatabase(ctx, teamID, id); err != nil && !errors.Is(err, store.ErrNotFound) {
		return err
	}
	return nil
}

func (a *API) cleanupPreviewJob(ctx context.Context, teamID, id uuid.UUID, p store.DeletionPayload) error {
	preview, err := a.Store.GetPreviewByID(ctx, teamID, id)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	app, err := a.Store.GetApplicationAny(ctx, teamID, preview.ApplicationID)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return err
	}
	client, err := a.dialPayloadServer(ctx, teamID, p.ServerID)
	if err != nil {
		return err
	}
	if client != nil && app != nil {
		if err := a.cleanupPreviewRemoteChecked(ctx, client, app, preview.PullRequestID, p); err != nil {
			return err
		}
	}
	_ = a.syncManagedDNS(ctx, teamID, "application_preview", preview.ID, nil, nil)
	if err := a.Store.DeletePreview(ctx, teamID, preview.ApplicationID, preview.PullRequestID); err != nil && !errors.Is(err, store.ErrNotFound) {
		return err
	}
	return nil
}

func (a *API) cleanupPreviewRemoteChecked(ctx context.Context, client *ssh.Client, app *store.Application, prID int, p store.DeletionPayload) error {
	cname := fmt.Sprintf("dockfin-%s-pr-%d", app.ID.String(), prID)
	if err := runChecked(ctx, client, "docker", "rm", "-f", cname); err != nil {
		return err
	}
	project := fmt.Sprintf("dockfin-%s-pr-%d", app.ID.String()[:8], prID)
	workdir := fmt.Sprintf("/data/dockfin/applications/%s-pr-%d", app.ID.String(), prID)
	for _, f := range []string{
		workdir + "/src/docker-compose.yaml",
		workdir + "/src/docker-compose.yml",
		workdir + "/src/compose.yaml",
	} {
		args := []string{"docker", "compose", "-p", project, "-f", f, "down", "--remove-orphans"}
		if p.DeleteVolumes {
			args = append(args, "-v")
		}
		if err := runChecked(ctx, client, args...); err != nil {
			return err
		}
	}
	if p.DeleteVolumes {
		if err := runChecked(ctx, client, "docker", "volume", "prune", "-f", "--filter", "label=com.docker.compose.project="+project); err != nil {
			return err
		}
	}
	if p.DeleteConfigurations {
		if err := runChecked(ctx, client, "rm", "-rf", workdir); err != nil {
			return err
		}
	}
	return nil
}

func (a *API) dialPayloadServer(ctx context.Context, teamID uuid.UUID, serverID string) (*ssh.Client, error) {
	if strings.TrimSpace(serverID) == "" {
		return nil, nil
	}
	id, err := uuid.Parse(serverID)
	if err != nil {
		return nil, err
	}
	return a.dialServerTeam(ctx, teamID, id)
}

func runChecked(ctx context.Context, client *ssh.Client, args ...string) error {
	if client == nil {
		return nil
	}
	_, errOut, err := sshx.RunArgsContext(ctx, client, args...)
	if err == nil || dockerMissingOK(errOut) || (err != nil && dockerMissingOK(err.Error()+" "+errOut)) {
		return nil
	}
	msg := strings.TrimSpace(errOut)
	if msg == "" {
		return err
	}
	return fmt.Errorf("%s", msg)
}

func removeResourceNetwork(ctx context.Context, client *ssh.Client, resourceID string) error {
	name := strings.TrimSpace(resourceID)
	if name == "" {
		return nil
	}
	if _, protected := protectedNetworks[name]; protected {
		return nil
	}
	_ = runChecked(ctx, client, "docker", "network", "disconnect", name, "dockfin-proxy")
	return runChecked(ctx, client, "docker", "network", "rm", name)
}

func (a *API) handleListDeletions(w http.ResponseWriter, r *http.Request) {
	list, err := a.Store.ListPendingDeletions(r.Context(), currentTeamID(r))
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deletions": list})
}

func (a *API) handleRetryDeletion(w http.ResponseWriter, r *http.Request) {
	kind, id, ok := deletionTarget(w, r)
	if !ok {
		return
	}
	teamID := currentTeamID(r)
	active, err := a.Store.DeleteJobActive(r.Context(), teamID, id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if active {
		writeError(w, http.StatusConflict, "cleanup is still running")
		return
	}
	payload, err := a.Store.LatestDeletePayload(r.Context(), teamID, id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if len(payload) == 0 {
		payload = store.MarshalDeletionPayload(store.DeletionPayload{
			ResourceType: kind, ResourceID: id.String(),
			DeleteVolumes: true, DeleteConfigurations: true, DeleteNetworks: true, DockerCleanup: true,
		})
	}
	job, err := a.Store.MarkDeleting(r.Context(), teamID, kind, id, payload)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"job": job, "status": "deleting"})
}

func (a *API) handleForgetDeletion(w http.ResponseWriter, r *http.Request) {
	kind, id, ok := deletionTarget(w, r)
	if !ok {
		return
	}
	teamID := currentTeamID(r)
	active, err := a.Store.DeleteJobActive(r.Context(), teamID, id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if active {
		writeError(w, http.StatusConflict, "cleanup is still running")
		return
	}
	if err := a.Store.CancelQueuedDeleteJobs(r.Context(), teamID, id); err != nil {
		mapStoreErr(w, err)
		return
	}
	switch kind {
	case "application":
		err = a.Store.DeleteApplication(r.Context(), teamID, id)
	case "service":
		err = a.Store.DeleteService(r.Context(), teamID, id)
	case "database":
		err = a.Store.DeleteDatabase(r.Context(), teamID, id)
	case "preview":
		preview, e := a.Store.GetPreviewByID(r.Context(), teamID, id)
		if e != nil {
			err = e
			break
		}
		err = a.Store.DeletePreview(r.Context(), teamID, preview.ApplicationID, preview.PullRequestID)
	default:
		writeError(w, http.StatusBadRequest, "unknown resource type")
		return
	}
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "forgotten"})
}

func deletionTarget(w http.ResponseWriter, r *http.Request) (string, uuid.UUID, bool) {
	var body struct {
		ResourceType string `json:"resource_type"`
		ResourceID   string `json:"resource_id"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return "", uuid.Nil, false
	}
	id, err := uuid.Parse(body.ResourceID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid resource")
		return "", uuid.Nil, false
	}
	switch body.ResourceType {
	case "application", "service", "database", "preview":
		return body.ResourceType, id, true
	default:
		writeError(w, http.StatusBadRequest, "unknown resource type")
		return "", uuid.Nil, false
	}
}
