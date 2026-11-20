package httpapi

import (
	"net/http"
	"strings"

	"github.com/dockfin/dockfin/internal/store"
	"github.com/dockfin/dockfin/internal/terminal"
	"github.com/google/uuid"
)

// containerResource is one application, service, or database a caller may touch
// on a specific server.
type containerResource struct {
	Kind       string
	ID         uuid.UUID
	ServerID   uuid.UUID
	CustomName string
}

// reservedContainer names are the control plane and edge, not a user's workload.
func reservedContainer(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "dockfin", "dockfin-proxy", "dockfin-sentinel", "dockfin-maintenance", "dockfin-cloudflared", "dockfin-db", "dockfin-buildkit":
		return true
	default:
		return false
	}
}

// containerBelongsToServer reports whether name is a workload container for one
// of the caller's resources on serverID. Admins do not use this check.
func containerBelongsToServer(name string, serverID uuid.UUID, resources []containerResource) bool {
	name = strings.TrimPrefix(strings.TrimSpace(name), "/")
	if reservedContainer(name) || !terminal.ValidContainerName(name) {
		return false
	}
	for _, res := range resources {
		if res.ServerID != serverID || res.ID == uuid.Nil {
			continue
		}
		id := res.ID.String()
		switch res.Kind {
		case "database":
			if name == "dockfin-db-"+id {
				return true
			}
		case "service":
			if strings.HasPrefix(name, "dockfin-svc-"+id[:8]+"-") {
				return true
			}
		case "application":
			if name == "dockfin-"+id {
				return true
			}
			if res.CustomName != "" && name == res.CustomName && !reservedContainer(res.CustomName) {
				return true
			}
			if strings.HasPrefix(name, "dockfin-"+id[:8]+"-") {
				return true
			}
		}
	}
	return false
}

// applicationLogContainerAllowed limits `docker logs` to this application's
// containers. live is the compose project container list from the server.
func applicationLogContainerAllowed(app *store.Application, name string, live []string) bool {
	if app == nil {
		return false
	}
	name = strings.TrimPrefix(strings.TrimSpace(name), "/")
	if reservedContainer(name) || !terminal.ValidContainerName(name) {
		return false
	}
	for _, n := range live {
		if strings.TrimPrefix(strings.TrimSpace(n), "/") == name {
			return true
		}
	}
	id := app.ID.String()
	if name == "dockfin-"+id {
		return true
	}
	if custom := applicationContainerAlias(app); custom != "" && name == custom {
		return true
	}
	if app.BuildPack == "dockercompose" {
		return false
	}
	return strings.HasPrefix(name, "dockfin-"+id+"-")
}

func applicationContainerAlias(app *store.Application) string {
	if app == nil || !app.IsConsistentContainerNameEnabled {
		return ""
	}
	s := strings.ToLower(strings.TrimSpace(app.CustomInternalName))
	var b strings.Builder
	for _, r := range s {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_'
		if ok {
			b.WriteRune(r)
		} else if r != 0 {
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return ""
	}
	if len(out) > 63 {
		out = out[:63]
	}
	if !strings.HasPrefix(out, "dockfin-") {
		out = "dockfin-" + out
	}
	if reservedContainer(out) || !terminal.ValidContainerName(out) {
		return ""
	}
	return out
}

func (a *API) workloadContainers(r *http.Request, serverID uuid.UUID) ([]containerResource, error) {
	ctx := r.Context()
	teamID := currentTeamID(r)
	user := currentUser(r)
	dests, err := a.Store.ListDestinations(ctx, teamID, &serverID)
	if err != nil {
		return nil, err
	}
	onDest := map[uuid.UUID]bool{}
	for _, d := range dests {
		if d.ServerID == serverID {
			onDest[d.ID] = true
		}
	}
	var out []containerResource
	apps, err := a.Store.ListApplications(ctx, teamID, nil)
	if err != nil {
		return nil, err
	}
	for i := range apps {
		app := &apps[i]
		if app.DestinationID == nil || !onDest[*app.DestinationID] {
			continue
		}
		if err := a.allowResource(ctx, teamID, user.ID, "application", app.ID, true, false); err != nil {
			continue
		}
		out = append(out, containerResource{
			Kind: "application", ID: app.ID, ServerID: serverID, CustomName: applicationContainerAlias(app),
		})
	}
	svcs, err := a.Store.ListServices(ctx, teamID, nil)
	if err != nil {
		return nil, err
	}
	for i := range svcs {
		svc := &svcs[i]
		on := (svc.ServerID != nil && *svc.ServerID == serverID) || (svc.DestinationID != nil && onDest[*svc.DestinationID])
		if !on {
			continue
		}
		if err := a.allowResource(ctx, teamID, user.ID, "service", svc.ID, true, false); err != nil {
			continue
		}
		out = append(out, containerResource{Kind: "service", ID: svc.ID, ServerID: serverID})
	}
	dbs, err := a.Store.ListDatabases(ctx, teamID, nil)
	if err != nil {
		return nil, err
	}
	for i := range dbs {
		db := &dbs[i]
		if db.DestinationID == nil || !onDest[*db.DestinationID] {
			continue
		}
		if err := a.allowResource(ctx, teamID, user.ID, "database", db.ID, true, false); err != nil {
			continue
		}
		out = append(out, containerResource{Kind: "database", ID: db.ID, ServerID: serverID})
	}
	return out, nil
}

func clampLogTail(raw string, def int) int {
	if raw == "" {
		return def
	}
	n := 0
	for _, r := range raw {
		if r < '0' || r > '9' {
			return def
		}
		n = n*10 + int(r-'0')
		if n > 5000 {
			return 5000
		}
	}
	if n <= 0 {
		return def
	}
	return n
}

func composeContainerNames(out string) []string {
	var names []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimPrefix(strings.TrimSpace(line), "/")
		if line != "" && !reservedContainer(line) {
			names = append(names, line)
		}
	}
	return names
}
