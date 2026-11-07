package store

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

func clampListLimit(limit, fallback, max int) int {
	if limit <= 0 {
		return fallback
	}
	if limit > max {
		return max
	}
	return limit
}

// OverviewCounts is the fleet size for one team.
type OverviewCounts struct {
	Servers      int `json:"servers"`
	Projects     int `json:"projects"`
	Applications int `json:"applications"`
	Databases    int `json:"databases"`
	Services     int `json:"services"`
}

// DeployWindow is deployment volume over the last 7 days.
type DeployWindow struct {
	Finished  int `json:"finished"`
	Failed    int `json:"failed"`
	Active    int `json:"active"`
	Cancelled int `json:"cancelled"`
}

// DeployQueue is work still waiting on this team's servers.
type DeployQueue struct {
	Queued     int `json:"queued"`
	InProgress int `json:"in_progress"`
}

// OverviewDeployment is a recent deploy without the log payload.
type OverviewDeployment struct {
	ID              uuid.UUID  `json:"id"`
	Status          string     `json:"status"`
	ResourceType    string     `json:"resource_type"`
	ResourceID      uuid.UUID  `json:"resource_id"`
	ResourceName    string     `json:"resource_name"`
	ProjectID       *uuid.UUID `json:"project_id,omitempty"`
	ProjectName     string     `json:"project_name,omitempty"`
	EnvironmentID   *uuid.UUID `json:"environment_id,omitempty"`
	EnvironmentName string     `json:"environment_name,omitempty"`
	CommitSHA       string     `json:"commit_sha"`
	CommitMessage   string     `json:"commit_message"`
	ErrorMessage    string     `json:"error_message,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	FinishedAt      *time.Time `json:"finished_at,omitempty"`
	DurationSeconds *int       `json:"duration_seconds,omitempty"`
}

// AttentionItem is something an operator should look at.
type AttentionItem struct {
	Kind            string     `json:"kind"`
	ResourceType    string     `json:"resource_type"`
	ResourceID      uuid.UUID  `json:"resource_id"`
	ResourceName    string     `json:"resource_name"`
	ProjectID       *uuid.UUID `json:"project_id,omitempty"`
	ProjectName     string     `json:"project_name,omitempty"`
	EnvironmentID   *uuid.UUID `json:"environment_id,omitempty"`
	EnvironmentName string     `json:"environment_name,omitempty"`
	Status          string     `json:"status,omitempty"`
	Message         string     `json:"message"`
	At              time.Time  `json:"at,omitempty"`
}

// FrozenEnvironment is an environment that refuses new deploys.
type FrozenEnvironment struct {
	ID          uuid.UUID  `json:"id"`
	Name        string     `json:"name"`
	ProjectID   uuid.UUID  `json:"project_id"`
	ProjectName string     `json:"project_name"`
	Reason      string     `json:"reason"`
	FrozenAt    *time.Time `json:"frozen_at,omitempty"`
}

// TeamOverview is the home-screen pulse for one team.
type TeamOverview struct {
	Counts             OverviewCounts       `json:"counts"`
	DeployWindow       DeployWindow         `json:"deploy_window"`
	Queue              DeployQueue          `json:"queue"`
	RecentProjects     []Project            `json:"recent_projects"`
	Deployments        []OverviewDeployment `json:"deployments"`
	Attention          []AttentionItem      `json:"attention"`
	FrozenEnvironments []FrozenEnvironment  `json:"frozen_environments"`
}

// TeamOverview loads counts, recent deploys, and items that need a human.
func (s *Store) TeamOverview(ctx context.Context, teamID uuid.UUID) (*TeamOverview, error) {
	out := &TeamOverview{
		RecentProjects:     []Project{},
		Deployments:        []OverviewDeployment{},
		Attention:          []AttentionItem{},
		FrozenEnvironments: []FrozenEnvironment{},
	}
	err := s.Pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM servers WHERE team_id=$1),
			(SELECT COUNT(*) FROM projects WHERE team_id=$1),
			(SELECT COUNT(*) FROM applications WHERE team_id=$1 AND deleted_at IS NULL),
			(SELECT COUNT(*) FROM databases WHERE team_id=$1 AND deleted_at IS NULL),
			(SELECT COUNT(*) FROM services WHERE team_id=$1 AND deleted_at IS NULL)
	`, teamID).Scan(
		&out.Counts.Servers, &out.Counts.Projects, &out.Counts.Applications,
		&out.Counts.Databases, &out.Counts.Services,
	)
	if err != nil {
		return nil, err
	}

	err = s.Pool.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE status='finished'),
			COUNT(*) FILTER (WHERE status='failed'),
			COUNT(*) FILTER (WHERE status IN ('queued','in_progress')),
			COUNT(*) FILTER (WHERE status='cancelled')
		FROM deployments
		WHERE team_id=$1 AND created_at > NOW() - INTERVAL '7 days'
	`, teamID).Scan(&out.DeployWindow.Finished, &out.DeployWindow.Failed, &out.DeployWindow.Active, &out.DeployWindow.Cancelled)
	if err != nil {
		return nil, err
	}

	err = s.Pool.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE status='queued'),
			COUNT(*) FILTER (WHERE status='in_progress')
		FROM deployments
		WHERE team_id=$1 AND status IN ('queued','in_progress')
	`, teamID).Scan(&out.Queue.Queued, &out.Queue.InProgress)
	if err != nil {
		return nil, err
	}

	if out.RecentProjects, err = s.listRecentProjects(ctx, teamID, 6); err != nil {
		return nil, err
	}
	if out.Deployments, err = s.listRecentDeployments(ctx, teamID, 12); err != nil {
		return nil, err
	}
	attention, err := s.listAttention(ctx, teamID)
	if err != nil {
		return nil, err
	}
	out.Attention = PrepareAttention(attention)
	if out.FrozenEnvironments, err = s.listFrozenEnvironments(ctx, teamID); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) listRecentProjects(ctx context.Context, teamID uuid.UUID, limit int) ([]Project, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, team_id, name, description, created_at
		FROM projects
		WHERE team_id=$1
		ORDER BY created_at DESC, name
		LIMIT $2
	`, teamID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Project{}
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.TeamID, &p.Name, &p.Description, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) listRecentDeployments(ctx context.Context, teamID uuid.UUID, limit int) ([]OverviewDeployment, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT d.id, d.status,
			CASE WHEN d.application_id IS NOT NULL THEN 'application' ELSE 'service' END,
			COALESCE(d.application_id, d.service_id),
			COALESCE(a.name, svc.name, ''),
			e.project_id, COALESCE(p.name, ''),
			COALESCE(a.environment_id, svc.environment_id), COALESCE(e.name, ''),
			d.commit_sha, LEFT(d.commit_message, 180), LEFT(d.error_message, 240),
			d.created_at, d.started_at, d.finished_at
		FROM deployments d
		LEFT JOIN applications a ON a.id = d.application_id AND a.team_id = d.team_id
		LEFT JOIN services svc ON svc.id = d.service_id AND svc.team_id = d.team_id
		LEFT JOIN environments e ON e.id = COALESCE(a.environment_id, svc.environment_id)
		LEFT JOIN projects p ON p.id = e.project_id
		WHERE d.team_id=$1 AND (d.application_id IS NOT NULL OR d.service_id IS NOT NULL)
		ORDER BY d.created_at DESC
		LIMIT $2
	`, teamID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []OverviewDeployment{}
	for rows.Next() {
		var d OverviewDeployment
		var resourceID *uuid.UUID
		if err := rows.Scan(
			&d.ID, &d.Status, &d.ResourceType, &resourceID, &d.ResourceName,
			&d.ProjectID, &d.ProjectName, &d.EnvironmentID, &d.EnvironmentName,
			&d.CommitSHA, &d.CommitMessage, &d.ErrorMessage,
			&d.CreatedAt, &d.StartedAt, &d.FinishedAt,
		); err != nil {
			return nil, err
		}
		if resourceID != nil {
			d.ResourceID = *resourceID
		}
		d.DurationSeconds = SecondsBetween(d.StartedAt, d.FinishedAt)
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) listAttention(ctx context.Context, teamID uuid.UUID) ([]AttentionItem, error) {
	var out []AttentionItem
	failed, err := s.listFailedLatestDeploys(ctx, teamID)
	if err != nil {
		return nil, err
	}
	out = append(out, failed...)
	unhealthy, err := s.listUnhealthyResources(ctx, teamID)
	if err != nil {
		return nil, err
	}
	out = append(out, unhealthy...)
	backups, err := s.listFailedBackups(ctx, teamID)
	if err != nil {
		return nil, err
	}
	out = append(out, backups...)
	return out, nil
}

func (s *Store) listFailedLatestDeploys(ctx context.Context, teamID uuid.UUID) ([]AttentionItem, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT
			CASE WHEN latest.application_id IS NOT NULL THEN 'application' ELSE 'service' END,
			COALESCE(latest.application_id, latest.service_id),
			COALESCE(a.name, svc.name, ''),
			e.project_id, COALESCE(p.name, ''),
			COALESCE(a.environment_id, svc.environment_id), COALESCE(e.name, ''),
			latest.error_message, latest.created_at
		FROM (
			SELECT DISTINCT ON (COALESCE(d.application_id, d.service_id))
				d.application_id, d.service_id, d.status, LEFT(d.error_message, 240) AS error_message, d.created_at
			FROM deployments d
			WHERE d.team_id=$1 AND (d.application_id IS NOT NULL OR d.service_id IS NOT NULL)
			ORDER BY COALESCE(d.application_id, d.service_id), d.created_at DESC
		) latest
		LEFT JOIN applications a ON a.id = latest.application_id AND a.team_id=$1
		LEFT JOIN services svc ON svc.id = latest.service_id AND svc.team_id=$1
		LEFT JOIN environments e ON e.id = COALESCE(a.environment_id, svc.environment_id)
		LEFT JOIN projects p ON p.id = e.project_id
		WHERE latest.status='failed'
		ORDER BY latest.created_at DESC
		LIMIT 12
	`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AttentionItem
	for rows.Next() {
		item := AttentionItem{Kind: "deploy_failed", Status: "failed", Message: "Latest deployment failed"}
		var id *uuid.UUID
		var errMsg string
		if err := rows.Scan(
			&item.ResourceType, &id, &item.ResourceName,
			&item.ProjectID, &item.ProjectName, &item.EnvironmentID, &item.EnvironmentName,
			&errMsg, &item.At,
		); err != nil {
			return nil, err
		}
		if id != nil {
			item.ResourceID = *id
		}
		if strings.TrimSpace(errMsg) != "" {
			item.Message = errMsg
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) listUnhealthyResources(ctx context.Context, teamID uuid.UUID) ([]AttentionItem, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT kind, id, name, status, environment_id, project_id, environment_name, project_name
		FROM (
			SELECT 'application' AS kind, a.id, a.name, a.status, a.environment_id, e.project_id,
				e.name AS environment_name, p.name AS project_name
			FROM applications a
			JOIN environments e ON e.id = a.environment_id
			JOIN projects p ON p.id = e.project_id
			WHERE a.team_id=$1 AND lower(a.status) IN ('failed', 'unhealthy', 'error', 'degraded')
			UNION ALL
			SELECT 'database', d.id, d.name, d.status, d.environment_id, e.project_id, e.name, p.name
			FROM databases d
			JOIN environments e ON e.id = d.environment_id
			JOIN projects p ON p.id = e.project_id
			WHERE d.team_id=$1 AND lower(d.status) IN ('failed', 'unhealthy', 'error', 'degraded')
			UNION ALL
			SELECT 'service', svc.id, svc.name, svc.status, svc.environment_id, e.project_id, e.name, p.name
			FROM services svc
			JOIN environments e ON e.id = svc.environment_id
			JOIN projects p ON p.id = e.project_id
			WHERE svc.team_id=$1 AND lower(svc.status) IN ('failed', 'unhealthy', 'error', 'degraded')
		) resources
		LIMIT 12
	`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AttentionItem
	for rows.Next() {
		item := AttentionItem{Kind: "unhealthy"}
		if err := rows.Scan(
			&item.ResourceType, &item.ResourceID, &item.ResourceName, &item.Status,
			&item.EnvironmentID, &item.ProjectID, &item.EnvironmentName, &item.ProjectName,
		); err != nil {
			return nil, err
		}
		item.Message = "Status is " + item.Status
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) listFailedBackups(ctx context.Context, teamID uuid.UUID) ([]AttentionItem, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT b.resource_type, b.resource_id, COALESCE(a.name, d.name, svc.name, ''),
			e.project_id, COALESCE(p.name, ''),
			COALESCE(a.environment_id, d.environment_id, svc.environment_id), COALESCE(e.name, ''),
			LEFT(b.error_message, 240), b.started_at
		FROM backup_executions b
		LEFT JOIN applications a ON b.resource_type='application' AND a.id=b.resource_id AND a.team_id=b.team_id
		LEFT JOIN databases d ON b.resource_type='database' AND d.id=b.resource_id AND d.team_id=b.team_id
		LEFT JOIN services svc ON b.resource_type='service' AND svc.id=b.resource_id AND svc.team_id=b.team_id
		LEFT JOIN environments e ON e.id = COALESCE(a.environment_id, d.environment_id, svc.environment_id)
		LEFT JOIN projects p ON p.id = e.project_id
		WHERE b.team_id=$1 AND b.status='failed' AND b.started_at > NOW() - INTERVAL '14 days'
		ORDER BY b.started_at DESC
		LIMIT 8
	`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AttentionItem
	for rows.Next() {
		item := AttentionItem{Kind: "backup_failed", Status: "failed", Message: "Backup failed"}
		var errMsg string
		if err := rows.Scan(
			&item.ResourceType, &item.ResourceID, &item.ResourceName,
			&item.ProjectID, &item.ProjectName, &item.EnvironmentID, &item.EnvironmentName,
			&errMsg, &item.At,
		); err != nil {
			return nil, err
		}
		if strings.TrimSpace(errMsg) != "" {
			item.Message = errMsg
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) listFrozenEnvironments(ctx context.Context, teamID uuid.UUID) ([]FrozenEnvironment, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT e.id, e.name, e.project_id, p.name, e.deploy_freeze_reason, e.deploy_frozen_at
		FROM environments e
		JOIN projects p ON p.id = e.project_id
		WHERE e.team_id=$1 AND e.deploy_frozen
		ORDER BY e.deploy_frozen_at DESC NULLS LAST, e.name
	`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []FrozenEnvironment{}
	for rows.Next() {
		var f FrozenEnvironment
		if err := rows.Scan(&f.ID, &f.Name, &f.ProjectID, &f.ProjectName, &f.Reason, &f.FrozenAt); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// SecondsBetween returns whole seconds from start to end.
// A missing timestamp or a negative span yields nil.
func SecondsBetween(start, end *time.Time) *int {
	if start == nil || end == nil {
		return nil
	}
	sec := int(end.Sub(*start).Seconds())
	if sec < 0 {
		return nil
	}
	return &sec
}

// PrepareAttention drops duplicate resources, prefers a failed deploy over a
// status warning, and keeps the newest 12 items.
func PrepareAttention(items []AttentionItem) []AttentionItem {
	if len(items) == 0 {
		return []AttentionItem{}
	}
	sort.SliceStable(items, func(i, j int) bool {
		return attentionNewer(items[i], items[j])
	})
	seen := map[string]int{}
	out := make([]AttentionItem, 0, len(items))
	for _, item := range items {
		if item.Kind == "backup_failed" {
			out = append(out, item)
			continue
		}
		key := item.ResourceType + ":" + item.ResourceID.String()
		if idx, ok := seen[key]; ok {
			if out[idx].Kind == "unhealthy" && item.Kind == "deploy_failed" {
				out[idx] = item
			}
			continue
		}
		seen[key] = len(out)
		out = append(out, item)
	}
	if len(out) > 12 {
		out = out[:12]
	}
	return out
}

func attentionNewer(a, b AttentionItem) bool {
	if a.At.IsZero() != b.At.IsZero() {
		return !a.At.IsZero()
	}
	return a.At.After(b.At)
}
