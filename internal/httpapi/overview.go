package httpapi

import (
	"net/http"

	"github.com/dockfin/dockfin/internal/store"
	"github.com/google/uuid"
)

func (a *API) handleOverview(w http.ResponseWriter, r *http.Request) {
	ov, err := a.Store.TeamOverview(r.Context(), currentTeamID(r))
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	ov = a.redactOverview(r, ov)
	writeJSON(w, http.StatusOK, ov)
}

// redactOverview hides projects a limited member cannot view. Counts of those
// projects are reduced so the dashboard does not reveal that hidden resources exist.
func (a *API) redactOverview(r *http.Request, ov *store.TeamOverview) *store.TeamOverview {
	if ov == nil || isTeamAdmin(r) {
		return ov
	}
	scope, err := a.Store.MemberAccess(r.Context(), currentTeamID(r), currentUser(r).ID, "")
	if err != nil || scope.Unrestricted {
		return ov
	}
	allow := func(id *uuid.UUID) bool {
		if id == nil {
			return false
		}
		_, ok := scope.Grants[*id]
		return ok
	}
	projects := make([]store.Project, 0, len(ov.RecentProjects))
	for _, p := range ov.RecentProjects {
		if _, ok := scope.Grants[p.ID]; ok {
			projects = append(projects, p)
		}
	}
	ov.RecentProjects = projects
	deps := make([]store.OverviewDeployment, 0, len(ov.Deployments))
	for _, d := range ov.Deployments {
		if allow(d.ProjectID) {
			deps = append(deps, d)
		}
	}
	ov.Deployments = deps
	items := make([]store.AttentionItem, 0, len(ov.Attention))
	for _, item := range ov.Attention {
		if allow(item.ProjectID) {
			items = append(items, item)
		}
	}
	ov.Attention = items
	frozen := make([]store.FrozenEnvironment, 0, len(ov.FrozenEnvironments))
	for _, env := range ov.FrozenEnvironments {
		id := env.ProjectID
		if allow(&id) {
			frozen = append(frozen, env)
		}
	}
	ov.FrozenEnvironments = frozen
	ov.Counts.Projects = len(scope.Grants)
	ov.Counts.Applications, ov.Counts.Databases, ov.Counts.Services = a.countScopedResources(r, scope)
	ov.DeployWindow = store.DeployWindow{}
	ov.Queue = store.DeployQueue{}
	return ov
}

func (a *API) countScopedResources(r *http.Request, scope store.AccessScope) (apps, dbs, svcs int) {
	teamID := currentTeamID(r)
	granted := func(envID uuid.UUID) bool {
		projectID, ok := scope.EnvProject[envID]
		if !ok {
			return false
		}
		_, ok = scope.Grants[projectID]
		return ok
	}
	if list, err := a.Store.ListApplications(r.Context(), teamID, nil); err == nil {
		for _, app := range list {
			if granted(app.EnvironmentID) {
				apps++
			}
		}
	}
	if list, err := a.Store.ListDatabases(r.Context(), teamID, nil); err == nil {
		for _, db := range list {
			if granted(db.EnvironmentID) {
				dbs++
			}
		}
	}
	if list, err := a.Store.ListServices(r.Context(), teamID, nil); err == nil {
		for _, svc := range list {
			if granted(svc.EnvironmentID) {
				svcs++
			}
		}
	}
	return apps, dbs, svcs
}
