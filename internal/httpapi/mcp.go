package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/dockfin/dockfin/internal/mcp"
	"github.com/dockfin/dockfin/internal/sshdial"
	"github.com/dockfin/dockfin/internal/sshx"
	"github.com/dockfin/dockfin/internal/version"
	"github.com/dockfin/dockfin/internal/worker"
)

// mcpMaxItems bounds list results so an agent cannot pull an unbounded dump.
const mcpMaxItems = 100

// mcpBackend adapts the store to mcp.Backend for one authenticated team.
type mcpBackend struct {
	api    *API
	teamID uuid.UUID
}

func (b *mcpBackend) ListServers(ctx context.Context) (any, error) {
	list, err := b.api.Store.ListServers(ctx, b.teamID)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(list))
	for i, s := range list {
		if i >= mcpMaxItems {
			break
		}
		out = append(out, map[string]any{
			"id": s.ID, "name": s.Name, "ip": s.IP, "port": s.Port,
			"user": s.UserName, "is_reachable": s.IsReachable, "is_usable": s.IsUsable,
			"proxy_type": s.ProxyType, "proxy_status": s.ProxyStatus,
		})
	}
	return map[string]any{"servers": out, "count": len(out)}, nil
}

func (b *mcpBackend) ListProjects(ctx context.Context) (any, error) {
	list, err := b.api.Store.ListProjects(ctx, b.teamID)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(list))
	for i, p := range list {
		if i >= mcpMaxItems {
			break
		}
		out = append(out, map[string]any{"id": p.ID, "name": p.Name, "description": p.Description})
	}
	return map[string]any{"projects": out, "count": len(out)}, nil
}

func (b *mcpBackend) ListApplications(ctx context.Context) (any, error) {
	list, err := b.api.Store.ListApplications(ctx, b.teamID, nil)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(list))
	for i, a := range list {
		if i >= mcpMaxItems {
			break
		}
		out = append(out, map[string]any{"id": a.ID, "name": a.Name, "status": a.Status, "build_pack": a.BuildPack, "fqdn": a.FQDN})
	}
	return map[string]any{"applications": out, "count": len(out)}, nil
}

func (b *mcpBackend) GetApplication(ctx context.Context, id string) (any, error) {
	appID, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid application id")
	}
	app, err := b.api.Store.GetApplication(ctx, b.teamID, appID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"id": app.ID, "name": app.Name, "status": app.Status, "build_pack": app.BuildPack,
		"git_repository": app.GitRepository, "git_branch": app.GitBranch, "fqdn": app.FQDN,
		"environment_id": app.EnvironmentID, "destination_id": app.DestinationID,
	}, nil
}

func (b *mcpBackend) DeployApplication(ctx context.Context, id string, forceRebuild bool) (any, error) {
	// Cookie sessions skip ability checks; API tokens must have deploy/write.
	if abilities, ok := ctx.Value(ctxAPIAbilities).([]string); ok && !apiTokenCanDeploy(abilities) {
		return nil, fmt.Errorf("insufficient API token abilities for deploy")
	}
	appID, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid application id")
	}
	app, err := b.api.Store.GetApplication(ctx, b.teamID, appID)
	if err != nil {
		return nil, err
	}
	if app.DestinationID == nil {
		return nil, fmt.Errorf("application has no destination")
	}
	dest, err := b.api.Store.GetDestination(ctx, b.teamID, *app.DestinationID)
	if err != nil {
		return nil, err
	}
	if b.api.Queue == nil {
		return nil, fmt.Errorf("deploy queue unavailable")
	}
	limit, _ := b.api.Store.GetDeploymentQueueLimit(ctx, dest.ServerID)
	active, _ := b.api.Store.CountActiveDeployments(ctx, dest.ServerID)
	if active >= limit {
		return nil, fmt.Errorf("server deployment queue limit reached")
	}
	serverID := dest.ServerID
	dep, err := b.api.Store.CreateDeployment(ctx, b.teamID, appID, &serverID, "", "", forceRebuild, false, true, 0)
	if err != nil {
		return nil, err
	}
	if err := b.api.Queue.Enqueue(worker.DeployJob{DeploymentID: dep.ID, TeamID: b.teamID, ForceRebuild: forceRebuild}); err != nil {
		return nil, err
	}
	return map[string]any{"deployment_id": dep.ID, "application_id": appID, "status": "queued"}, nil
}

func (b *mcpBackend) StopApplication(ctx context.Context, id string) (any, error) {
	appID, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid application id")
	}
	app, err := b.api.Store.GetApplication(ctx, b.teamID, appID)
	if err != nil {
		return nil, err
	}
	if app.DestinationID == nil {
		return nil, fmt.Errorf("application has no destination")
	}
	dest, err := b.api.Store.GetDestination(ctx, b.teamID, *app.DestinationID)
	if err != nil {
		return nil, err
	}
	if b.api.Queue == nil || b.api.Queue.SSH == nil {
		return nil, fmt.Errorf("ssh pool unavailable")
	}
	client, err := sshdial.DialClient(ctx, b.api.Store, b.api.Queue.SSH, b.teamID, dest.ServerID)
	if err != nil {
		return nil, err
	}
	cname := "dockfin-" + app.ID.String()
	_, _, _ = sshx.RunArgs(client, "docker", "stop", cname)
	_ = b.api.Store.UpdateApplicationStatus(ctx, app.ID, "exited")
	return map[string]any{"id": app.ID, "status": "exited"}, nil
}

func (b *mcpBackend) ListServices(ctx context.Context) (any, error) {
	list, err := b.api.Store.ListServices(ctx, b.teamID, nil)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(list))
	for i, s := range list {
		if i >= mcpMaxItems {
			break
		}
		out = append(out, map[string]any{"id": s.ID, "name": s.Name, "status": s.Status, "service_type": s.ServiceType, "fqdn": s.FQDN})
	}
	return map[string]any{"services": out, "count": len(out)}, nil
}

func (b *mcpBackend) DeployService(ctx context.Context, id string) (any, error) {
	if abilities, ok := ctx.Value(ctxAPIAbilities).([]string); ok && !apiTokenCanDeploy(abilities) {
		return nil, fmt.Errorf("insufficient API token abilities for deploy")
	}
	svcID, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid service id")
	}
	svc, err := b.api.Store.GetService(ctx, b.teamID, svcID)
	if err != nil {
		return nil, err
	}
	serverID, _, err := b.api.resolveServiceTarget(ctx, b.teamID, svc)
	if err != nil {
		return nil, err
	}
	if b.api.Queue == nil {
		return nil, fmt.Errorf("deploy queue unavailable")
	}
	sid := serverID
	dep, err := b.api.Store.CreateServiceDeployment(ctx, b.teamID, svcID, &sid, false, false, true)
	if err != nil {
		return nil, err
	}
	if err := b.api.Queue.Enqueue(worker.DeployJob{DeploymentID: dep.ID, TeamID: b.teamID}); err != nil {
		return nil, err
	}
	return map[string]any{"deployment_id": dep.ID, "service_id": svcID, "status": "queued"}, nil
}

func (b *mcpBackend) ListDatabases(ctx context.Context, environmentID string) (any, error) {
	var envID *uuid.UUID
	if environmentID != "" {
		id, err := uuid.Parse(environmentID)
		if err != nil {
			return nil, fmt.Errorf("invalid environment_id")
		}
		envID = &id
	}
	list, err := b.api.Store.ListDatabases(ctx, b.teamID, envID)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(list))
	for i, d := range list {
		if i >= mcpMaxItems {
			break
		}
		out = append(out, map[string]any{
			"id": d.ID, "name": d.Name, "engine": d.Engine, "status": d.Status,
			"environment_id": d.EnvironmentID,
		})
	}
	return map[string]any{"databases": out, "count": len(out)}, nil
}

// mcpEnabled gates the endpoint on the instance setting.
func (a *API) mcpEnabled(ctx context.Context) bool {
	st, err := a.Store.GetInstanceSettings(ctx)
	return err == nil && st.IsMCPServerEnabled
}

// handleMCPProbe answers GET /api/v1/mcp with server metadata (useful for clients
// that check availability before opening a session).
func (a *API) handleMCPProbe(w http.ResponseWriter, r *http.Request) {
	if !a.mcpEnabled(r.Context()) {
		writeError(w, http.StatusNotFound, "MCP server is disabled")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"name":             "dockfin",
		"version":          version.Version,
		"protocol_version": mcp.ProtocolVersion,
		"transport":        "http",
		"tools":            mcp.Tools(),
	})
}

func (a *API) handleMCP(w http.ResponseWriter, r *http.Request) {
	if !a.mcpEnabled(r.Context()) {
		writeError(w, http.StatusNotFound, "MCP server is disabled")
		return
	}
	// MCP clients speak plain JSON-RPC; unknown fields must not be rejected.
	defer r.Body.Close()
	var raw json.RawMessage
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&raw); err != nil {
		writeJSON(w, http.StatusOK, mcp.Response{
			JSONRPC: "2.0",
			Error:   &mcp.RPCError{Code: mcp.CodeParse, Message: "invalid JSON"},
		})
		return
	}
	backend := &mcpBackend{api: a, teamID: currentTeamID(r)}

	// Batch request.
	if len(raw) > 0 && raw[0] == '[' {
		var reqs []mcp.Request
		if err := json.Unmarshal(raw, &reqs); err != nil {
			writeJSON(w, http.StatusOK, mcp.Response{
				JSONRPC: "2.0",
				Error:   &mcp.RPCError{Code: mcp.CodeInvalidRequest, Message: "invalid batch"},
			})
			return
		}
		var out []mcp.Response
		for _, req := range reqs {
			if resp := mcp.Handle(r.Context(), backend, req, "dockfin", version.Version); resp != nil {
				out = append(out, *resp)
			}
		}
		if len(out) == 0 {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		writeJSON(w, http.StatusOK, out)
		return
	}

	var req mcp.Request
	if err := json.Unmarshal(raw, &req); err != nil {
		writeJSON(w, http.StatusOK, mcp.Response{
			JSONRPC: "2.0",
			Error:   &mcp.RPCError{Code: mcp.CodeInvalidRequest, Message: "invalid request"},
		})
		return
	}
	resp := mcp.Handle(r.Context(), backend, req, "dockfin", version.Version)
	if resp == nil {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

var _ mcp.Backend = (*mcpBackend)(nil)
