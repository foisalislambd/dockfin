package httpapi

import (
	"net/http"
	"strings"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/dockfin/dockfin/internal/terminal"
)

func (a *API) handleCreateTerminal(w http.ResponseWriter, r *http.Request) {
	serverID, err := uuid.Parse(chi.URLParam(r, "serverID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if a.Terminals == nil {
		writeError(w, http.StatusServiceUnavailable, "terminal not configured")
		return
	}
	var body struct {
		Container string `json:"container"`
	}
	_ = decodeJSON(r, &body)
	body.Container = strings.TrimSpace(body.Container)
	role, _ := r.Context().Value(ctxRole).(string)
	isAdmin := role == "owner" || role == "admin"
	// Host shell (no container) is equivalent to server exec — admin/owner only.
	if body.Container == "" {
		if !isAdmin {
			writeError(w, http.StatusForbidden, "host shell requires admin or owner role")
			return
		}
	} else if !terminal.ValidContainerName(body.Container) {
		writeError(w, http.StatusBadRequest, "invalid container name")
		return
	}
	if !isAdmin && !a.terminalACLAllows(r, serverID) {
		writeError(w, http.StatusForbidden, "terminal access is restricted on this server")
		return
	}

	client, err := a.dialServer(r, serverID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	sessionID, err := a.Terminals.Create(terminal.CreateOpts{
		TeamID:    currentTeamID(r),
		UserID:    currentUser(r).ID,
		ServerID:  serverID,
		Client:    client,
		Container: body.Container,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"session_id": sessionID.String()})
}

// terminalACLAllows reports whether the caller may open a terminal on the
// server. An empty terminal_acl_user_ids list keeps the previous behaviour of
// allowing every team member; owners and admins always bypass the list.
func (a *API) terminalACLAllows(r *http.Request, serverID uuid.UUID) bool {
	ops, err := a.Store.GetServerOpsSettings(r.Context(), currentTeamID(r), serverID)
	if err != nil || len(ops.TerminalACLUserIDs) == 0 {
		return true
	}
	me := currentUser(r).ID.String()
	for _, id := range ops.TerminalACLUserIDs {
		if strings.EqualFold(strings.TrimSpace(id), me) {
			return true
		}
	}
	return false
}

func (a *API) handleTerminalWS(w http.ResponseWriter, r *http.Request) {
	if a.Terminals == nil {
		writeError(w, http.StatusServiceUnavailable, "terminal not configured")
		return
	}
	sessionID, err := uuid.Parse(chi.URLParam(r, "sessionID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid session id")
		return
	}
	ls, err := a.Terminals.Get(sessionID, currentTeamID(r))
	if err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: a.Cfg.CORSOrigins,
	})
	if err != nil {
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "")
	a.Terminals.ServeWS(r.Context(), conn, ls)
}
