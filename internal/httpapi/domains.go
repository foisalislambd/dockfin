package httpapi

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/dockfin/dockfin/internal/proxy"
	"github.com/dockfin/dockfin/internal/store"
)

// resolveServerForDomain returns the server used for magic/wildcard FQDN generation.
func (a *API) resolveServerForDomain(ctx context.Context, teamID uuid.UUID, serverID, destinationID *uuid.UUID) (*store.Server, error) {
	switch {
	case serverID != nil:
		return a.Store.GetServer(ctx, teamID, *serverID)
	case destinationID != nil:
		dest, err := a.Store.GetDestination(ctx, teamID, *destinationID)
		if err != nil {
			return nil, err
		}
		return a.Store.GetServer(ctx, teamID, dest.ServerID)
	default:
		return nil, store.ErrNotFound
	}
}

// generateResourceFQDN builds a free sslip.io/nip.io (or wildcard) hostname.
func generateResourceFQDN(name string, id uuid.UUID, srv *store.Server) string {
	if srv == nil {
		return ""
	}
	magicIP := proxy.PreferMagicIP(srv.IP, srv.PublicIP)
	return proxy.GenerateFQDN(name, id, magicIP, srv.WildcardDomain, srv.MagicDomain)
}

func (a *API) handleGenerateDomain(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name          string `json:"name"`
		ServerID      string `json:"server_id"`
		DestinationID string `json:"destination_id"`
		ResourceID    string `json:"resource_id"` // optional; random UUID used if empty
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if body.Name == "" {
		writeError(w, http.StatusBadRequest, "name required")
		return
	}
	teamID := currentTeamID(r)
	var serverID, destID *uuid.UUID
	if body.ServerID != "" {
		id, err := uuid.Parse(body.ServerID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid server_id")
			return
		}
		serverID = &id
	}
	if body.DestinationID != "" {
		id, err := uuid.Parse(body.DestinationID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid destination_id")
			return
		}
		destID = &id
	}
	srv, err := a.resolveServerForDomain(r.Context(), teamID, serverID, destID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	rid := uuid.New()
	if body.ResourceID != "" {
		if id, err := uuid.Parse(body.ResourceID); err == nil {
			rid = id
		}
	}
	fqdn := generateResourceFQDN(body.Name, rid, srv)
	if fqdn == "" {
		writeError(w, http.StatusBadRequest, "cannot generate domain: server has no usable IP")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"fqdn": fqdn,
		"url":  proxy.PublicURL(fqdn),
	})
}
