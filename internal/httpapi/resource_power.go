package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/dockfin/dockfin/internal/database"
	"github.com/dockfin/dockfin/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (a *API) queueResourcePower(w http.ResponseWriter, r *http.Request, kind, param, action, interim, statusOnOK string) {
	id, err := uuid.Parse(chi.URLParam(r, param))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	teamID := currentTeamID(r)
	status, err := a.resourceStatus(r.Context(), teamID, kind, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrConflict) {
			mapStoreErr(w, err)
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if refuseWhileBusy(w, status) {
		return
	}
	payload, _ := json.Marshal(map[string]string{
		"resource_type": kind,
		"resource_id":   id.String(),
		"action":        action,
		"status_on_ok":  statusOnOK,
	})
	job, err := a.Store.EnqueueJob(r.Context(), teamID, "resource_power", payload)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	_ = a.setResourceStatus(r.Context(), kind, id, interim)
	writeJSON(w, http.StatusAccepted, map[string]any{"job": job, "status": interim})
}

func (a *API) resourceStatus(ctx context.Context, teamID uuid.UUID, kind string, id uuid.UUID) (string, error) {
	switch kind {
	case "application":
		app, err := a.Store.GetApplication(ctx, teamID, id)
		if err != nil {
			return "", err
		}
		if app.DestinationID == nil {
			return "", fmt.Errorf("application has no destination")
		}
		return app.Status, nil
	case "service":
		svc, err := a.Store.GetService(ctx, teamID, id)
		if err != nil {
			return "", err
		}
		if _, _, err := a.resolveServiceTarget(ctx, teamID, svc); err != nil {
			return "", err
		}
		return svc.Status, nil
	case "database":
		db, err := a.Store.GetDatabase(ctx, teamID, id)
		if err != nil {
			return "", err
		}
		if db.DestinationID == nil {
			return "", fmt.Errorf("database has no destination")
		}
		return db.Status, nil
	default:
		return "", fmt.Errorf("unknown resource type")
	}
}

func (a *API) setResourceStatus(ctx context.Context, kind string, id uuid.UUID, status string) error {
	switch kind {
	case "application":
		return a.Store.UpdateApplicationStatus(ctx, id, status)
	case "service":
		return a.Store.UpdateServiceStatus(ctx, id, status)
	case "database":
		return a.Store.UpdateDatabaseStatus(ctx, id, status)
	default:
		return fmt.Errorf("unknown resource type")
	}
}

func (a *API) jobResourcePower(ctx context.Context, job *store.BackgroundJob) ([]byte, error) {
	var p struct {
		ResourceType string `json:"resource_type"`
		ResourceID   string `json:"resource_id"`
		Action       string `json:"action"`
		StatusOnOK   string `json:"status_on_ok"`
	}
	if err := json.Unmarshal(job.Payload, &p); err != nil {
		return nil, err
	}
	id, err := uuid.Parse(p.ResourceID)
	if err != nil {
		return nil, err
	}
	switch p.ResourceType {
	case "application":
		app, err := a.Store.GetApplication(ctx, job.TeamID, id)
		if errors.Is(err, store.ErrNotFound) {
			return json.Marshal(map[string]string{"status": "skipped"})
		}
		if err != nil {
			return nil, err
		}
		if err := a.applyApplicationPower(ctx, job.TeamID, app, p.Action, p.StatusOnOK); err != nil {
			return nil, err
		}
	case "service":
		svc, err := a.Store.GetService(ctx, job.TeamID, id)
		if errors.Is(err, store.ErrNotFound) {
			return json.Marshal(map[string]string{"status": "skipped"})
		}
		if err != nil {
			return nil, err
		}
		if err := a.applyServicePower(ctx, job.TeamID, svc, p.Action, p.StatusOnOK); err != nil {
			return nil, err
		}
	case "database":
		db, err := a.Store.GetDatabase(ctx, job.TeamID, id)
		if errors.Is(err, store.ErrNotFound) {
			return json.Marshal(map[string]string{"status": "skipped"})
		}
		if err != nil {
			return nil, err
		}
		if p.Action == "stop" {
			if err := a.applyDatabaseStop(ctx, job.TeamID, db); err != nil {
				return nil, err
			}
		} else if err := a.applyDatabaseStart(ctx, job.TeamID, db); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unknown resource type")
	}
	return json.Marshal(map[string]string{"status": p.StatusOnOK, "resource_id": id.String()})
}

func (a *API) notePowerFailure(job *store.BackgroundJob) {
	var p struct {
		ResourceType string `json:"resource_type"`
		ResourceID   string `json:"resource_id"`
	}
	if json.Unmarshal(job.Payload, &p) != nil {
		return
	}
	id, err := uuid.Parse(p.ResourceID)
	if err != nil {
		return
	}
	_ = a.setResourceStatus(context.Background(), p.ResourceType, id, "error")
}

func (a *API) applyDatabaseStop(ctx context.Context, teamID uuid.UUID, db *store.Database) error {
	if db.DestinationID == nil {
		return fmt.Errorf("database has no destination")
	}
	dest, err := a.Store.GetDestination(ctx, teamID, *db.DestinationID)
	if err != nil {
		return err
	}
	client, err := a.dialServerTeam(ctx, teamID, dest.ServerID)
	if err != nil {
		return err
	}
	if err := database.Stop(client, db.ID.String()); err != nil && !dockerMissingOK(err.Error()) {
		return err
	}
	return a.Store.UpdateDatabaseStatus(ctx, db.ID, "exited")
}

func (a *API) applyDatabaseStart(ctx context.Context, teamID uuid.UUID, db *store.Database) error {
	if db.DestinationID == nil {
		return fmt.Errorf("database has no destination")
	}
	dest, err := a.Store.GetDestination(ctx, teamID, *db.DestinationID)
	if err != nil {
		return err
	}
	client, err := a.dialServerTeam(ctx, teamID, dest.ServerID)
	if err != nil {
		return err
	}
	enc, err := a.Store.GetDatabaseCredentials(ctx, teamID, db.ID)
	if err != nil {
		return err
	}
	password := ""
	if enc != "" {
		plain, err := a.Store.Box.Decrypt(enc)
		if err != nil {
			return fmt.Errorf("decrypt credentials")
		}
		var creds map[string]string
		if json.Unmarshal(plain, &creds) == nil {
			password = creds["password"]
		}
	}
	envMap, err := a.databaseEnv(ctx, teamID, db, dest.ServerID)
	if err != nil {
		return err
	}
	var tlsMat *database.TLSMaterial
	if db.EnableSSL {
		tlsMat, err = a.prepareDatabaseTLS(ctx, teamID, dest.ServerID, db)
		if err != nil {
			return err
		}
	}
	mounts, err := a.databaseVolumeMounts(ctx, client, teamID, db)
	if err != nil {
		return err
	}
	scripts, err := a.loadDatabaseInitScripts(ctx, teamID, db)
	if err != nil {
		return err
	}
	if err := database.Start(client, db, dest.Network, password, envMap, tlsMat, mounts, scripts); err != nil {
		return err
	}
	return a.Store.UpdateDatabaseStatus(ctx, db.ID, "running")
}
