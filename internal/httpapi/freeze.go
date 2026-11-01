package httpapi

import (
	"context"
	"net/http"

	"github.com/dockfin/dockfin/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (a *API) handleSetDeployFreeze(w http.ResponseWriter, r *http.Request) {
	if !isTeamAdmin(r) {
		writeError(w, http.StatusForbidden, "admin or owner role required")
		return
	}
	pid, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	eid, err := uuid.Parse(chi.URLParam(r, "envID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid environment id")
		return
	}
	var body struct {
		Frozen bool   `json:"frozen"`
		Reason string `json:"reason"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	reason, err := store.NormalizeFreezeReason(body.Reason)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var userID *uuid.UUID
	if u := currentUser(r); u != nil {
		id := u.ID
		userID = &id
	}
	env, err := a.Store.SetEnvironmentDeployFreeze(r.Context(), currentTeamID(r), pid, eid, userID, body.Frozen, reason)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, env)
}

// stopIfFrozen writes a response and reports true when a new deploy must not start.
// allowOverride is honored only for team owners and admins.
func (a *API) stopIfFrozen(w http.ResponseWriter, r *http.Request, envID uuid.UUID, allowOverride bool) bool {
	msg, err := a.frozenDeployMessage(r.Context(), currentTeamID(r), envID)
	if err != nil {
		mapStoreErr(w, err)
		return true
	}
	if msg == "" {
		return false
	}
	if allowOverride {
		if !isTeamAdmin(r) {
			writeError(w, http.StatusForbidden, "only an owner or admin can override a deploy freeze")
			return true
		}
		return false
	}
	writeError(w, http.StatusLocked, msg)
	return true
}

// frozenDeployMessage returns a user-facing message when the environment is frozen.
// An empty message means deploys are allowed. A store error is returned unchanged.
func (a *API) frozenDeployMessage(ctx context.Context, teamID, envID uuid.UUID) (string, error) {
	if a == nil || a.Store == nil || envID == uuid.Nil {
		return "", nil
	}
	env, err := a.Store.GetEnvironment(ctx, teamID, envID)
	if err != nil {
		return "", err
	}
	if !env.DeployFrozen {
		return "", nil
	}
	return store.FrozenDeployError(env.DeployFreezeReason).Error(), nil
}
