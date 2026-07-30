package httpapi

import (
	"net/http"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/goolify/goolify/internal/terminal"
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
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"session_id": sessionID.String()})
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
