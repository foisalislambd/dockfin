package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// PendingDeletion is a resource hidden from lists while cleanup runs or after it failed.
type PendingDeletion struct {
	ResourceType  string     `json:"resource_type"`
	ResourceID    uuid.UUID  `json:"resource_id"`
	Name          string     `json:"name"`
	Status        string     `json:"status"`
	Error         string     `json:"error"`
	EnvironmentID *uuid.UUID `json:"environment_id,omitempty"`
	ProjectID     *uuid.UUID `json:"project_id,omitempty"`
	DeletedAt     time.Time  `json:"deleted_at"`
}

// MarkDeleting hides the resource and inserts a resource_delete job in one transaction.
// A second call is allowed only after a previous cleanup stored delete_error.
func (s *Store) MarkDeleting(ctx context.Context, teamID uuid.UUID, kind string, id uuid.UUID, payload []byte) (*BackgroundJob, error) {
	if len(payload) == 0 {
		payload = []byte("{}")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	exists, deleting, err := deletionState(ctx, tx, teamID, kind, id)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrNotFound
	}
	if deleting {
		var active uuid.UUID
		err = tx.QueryRow(ctx, `
			SELECT id FROM background_jobs
			WHERE team_id=$1 AND kind='resource_delete' AND status IN ('queued','running')
			  AND payload->>'resource_id'=$2
			LIMIT 1
		`, teamID, id.String()).Scan(&active)
		if err == nil {
			return nil, fmt.Errorf("%w: deletion already scheduled", ErrConflict)
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
	}

	switch kind {
	case "application":
		if _, err := tx.Exec(ctx, `
			UPDATE applications
			SET deleted_at=COALESCE(deleted_at, NOW()), status='deleting', delete_error='', updated_at=NOW()
			WHERE id=$1 AND team_id=$2
		`, id, teamID); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE application_previews
			SET deleted_at=COALESCE(deleted_at, NOW()), status='deleting', delete_error='', updated_at=NOW()
			WHERE application_id=$1 AND team_id=$2
		`, id, teamID); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE deployments
			SET status='cancelled', finished_at=NOW(), updated_at=NOW(), error_message='resource deleted'
			WHERE team_id=$1 AND application_id=$2 AND status='queued'
		`, teamID, id); err != nil {
			return nil, err
		}
	case "service":
		if _, err := tx.Exec(ctx, `
			UPDATE services
			SET deleted_at=COALESCE(deleted_at, NOW()), status='deleting', delete_error='', updated_at=NOW()
			WHERE id=$1 AND team_id=$2
		`, id, teamID); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE deployments
			SET status='cancelled', finished_at=NOW(), updated_at=NOW(), error_message='resource deleted'
			WHERE team_id=$1 AND service_id=$2 AND status='queued'
		`, teamID, id); err != nil {
			return nil, err
		}
	case "database":
		if _, err := tx.Exec(ctx, `
			UPDATE databases
			SET deleted_at=COALESCE(deleted_at, NOW()), status='deleting', delete_error='', updated_at=NOW()
			WHERE id=$1 AND team_id=$2
		`, id, teamID); err != nil {
			return nil, err
		}
	case "preview":
		if _, err := tx.Exec(ctx, `
			UPDATE application_previews
			SET deleted_at=COALESCE(deleted_at, NOW()), status='deleting', delete_error='', updated_at=NOW()
			WHERE id=$1 AND team_id=$2
		`, id, teamID); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unknown resource type")
	}

	var job BackgroundJob
	err = tx.QueryRow(ctx, `
		INSERT INTO background_jobs (team_id, kind, payload)
		VALUES ($1, 'resource_delete', $2)
		RETURNING id, team_id, kind, payload, status, attempts, max_attempts, run_after, error, result, created_at, updated_at
	`, teamID, payload).Scan(
		&job.ID, &job.TeamID, &job.Kind, &job.Payload, &job.Status, &job.Attempts, &job.MaxAttempts,
		&job.RunAfter, &job.Error, &job.Result, &job.CreatedAt, &job.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &job, nil
}

func deletionState(ctx context.Context, tx pgx.Tx, teamID uuid.UUID, kind string, id uuid.UUID) (exists, deleting bool, err error) {
	var deletedAt *time.Time
	var deleteError string
	switch kind {
	case "application":
		err = tx.QueryRow(ctx, `SELECT deleted_at, delete_error FROM applications WHERE id=$1 AND team_id=$2`, id, teamID).Scan(&deletedAt, &deleteError)
	case "service":
		err = tx.QueryRow(ctx, `SELECT deleted_at, delete_error FROM services WHERE id=$1 AND team_id=$2`, id, teamID).Scan(&deletedAt, &deleteError)
	case "database":
		err = tx.QueryRow(ctx, `SELECT deleted_at, delete_error FROM databases WHERE id=$1 AND team_id=$2`, id, teamID).Scan(&deletedAt, &deleteError)
	case "preview":
		err = tx.QueryRow(ctx, `SELECT deleted_at, delete_error FROM application_previews WHERE id=$1 AND team_id=$2`, id, teamID).Scan(&deletedAt, &deleteError)
	default:
		return false, false, fmt.Errorf("unknown resource type")
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return false, false, nil
	}
	if err != nil {
		return false, false, err
	}
	// A tombstone with an empty error and no active job is still "deleting".
	return true, deletedAt != nil && deleteError == "", nil
}

func (s *Store) SetDeleteError(ctx context.Context, teamID uuid.UUID, kind string, id uuid.UUID, message string) error {
	var q string
	switch kind {
	case "application":
		q = `UPDATE applications SET delete_error=$3, updated_at=NOW() WHERE id=$1 AND team_id=$2 AND deleted_at IS NOT NULL`
	case "service":
		q = `UPDATE services SET delete_error=$3, updated_at=NOW() WHERE id=$1 AND team_id=$2 AND deleted_at IS NOT NULL`
	case "database":
		q = `UPDATE databases SET delete_error=$3, updated_at=NOW() WHERE id=$1 AND team_id=$2 AND deleted_at IS NOT NULL`
	case "preview":
		q = `UPDATE application_previews SET delete_error=$3, updated_at=NOW() WHERE id=$1 AND team_id=$2 AND deleted_at IS NOT NULL`
	default:
		return fmt.Errorf("unknown resource type")
	}
	_, err := s.Pool.Exec(ctx, q, id, teamID, message)
	return err
}

func (s *Store) ListPendingDeletions(ctx context.Context, teamID uuid.UUID) ([]PendingDeletion, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT 'application', a.id, a.name, a.status, a.delete_error, a.deleted_at, a.environment_id, e.project_id
		FROM applications a
		JOIN environments e ON e.id = a.environment_id
		WHERE a.team_id=$1 AND a.deleted_at IS NOT NULL
		UNION ALL
		SELECT 'service', sv.id, sv.name, sv.status, sv.delete_error, sv.deleted_at, sv.environment_id, e.project_id
		FROM services sv
		JOIN environments e ON e.id = sv.environment_id
		WHERE sv.team_id=$1 AND sv.deleted_at IS NOT NULL
		UNION ALL
		SELECT 'database', d.id, d.name, d.status, d.delete_error, d.deleted_at, d.environment_id, e.project_id
		FROM databases d
		JOIN environments e ON e.id = d.environment_id
		WHERE d.team_id=$1 AND d.deleted_at IS NOT NULL
		UNION ALL
		SELECT 'preview', p.id, a.name || ' PR #' || p.pull_request_id::text, p.status, p.delete_error, p.deleted_at, a.environment_id, e.project_id
		FROM application_previews p
		JOIN applications a ON a.id = p.application_id
		JOIN environments e ON e.id = a.environment_id
		WHERE p.team_id=$1 AND p.deleted_at IS NOT NULL AND a.deleted_at IS NULL
		ORDER BY 6 DESC
	`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PendingDeletion{}
	for rows.Next() {
		var row PendingDeletion
		if err := rows.Scan(&row.ResourceType, &row.ResourceID, &row.Name, &row.Status, &row.Error, &row.DeletedAt, &row.EnvironmentID, &row.ProjectID); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// DeleteJobActive reports a queued or running resource_delete for this resource.
func (s *Store) DeleteJobActive(ctx context.Context, teamID, resourceID uuid.UUID) (bool, error) {
	var n int
	err := s.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM background_jobs
		WHERE team_id=$1 AND kind='resource_delete' AND status IN ('queued','running')
		  AND payload->>'resource_id'=$2
	`, teamID, resourceID.String()).Scan(&n)
	return n > 0, err
}

func (s *Store) CancelQueuedDeleteJobs(ctx context.Context, teamID, resourceID uuid.UUID) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE background_jobs SET status='cancelled', updated_at=NOW()
		WHERE team_id=$1 AND kind='resource_delete' AND status='queued'
		  AND payload->>'resource_id'=$2
	`, teamID, resourceID.String())
	return err
}

// deletionPayload is documented here so job writers and the store stay aligned.
type DeletionPayload struct {
	ResourceType         string `json:"resource_type"`
	ResourceID           string `json:"resource_id"`
	ServerID             string `json:"server_id,omitempty"`
	ApplicationID        string `json:"application_id,omitempty"`
	PullRequestID        int    `json:"pull_request_id,omitempty"`
	DeleteVolumes        bool   `json:"delete_volumes"`
	DeleteConfigurations bool   `json:"delete_configurations"`
	DeleteNetworks       bool   `json:"delete_networks"`
	DockerCleanup        bool   `json:"docker_cleanup"`
}

func (s *Store) GetDatabaseAny(ctx context.Context, teamID, id uuid.UUID) (*Database, error) {
	var d Database
	err := s.Pool.QueryRow(ctx, `
		SELECT id, team_id, environment_id, destination_id, name, description, engine, image, status, is_public, public_port, COALESCE(enable_ssl,false), engine_config, created_at
		FROM databases WHERE id=$1 AND team_id=$2
	`, id, teamID).Scan(
		&d.ID, &d.TeamID, &d.EnvironmentID, &d.DestinationID, &d.Name, &d.Description, &d.Engine, &d.Image,
		&d.Status, &d.IsPublic, &d.PublicPort, &d.EnableSSL, &d.EngineConfig, &d.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &d, err
}

func (s *Store) GetServiceAny(ctx context.Context, teamID, id uuid.UUID) (*Service, error) {
	var svc Service
	err := s.Pool.QueryRow(ctx, `
		SELECT id, team_id, environment_id, server_id, destination_id, name, description, service_type, docker_compose_raw, COALESCE(docker_compose, ''), COALESCE(fqdn,''), status, created_at, COALESCE(is_force_https, TRUE)
		FROM services WHERE id=$1 AND team_id=$2
	`, id, teamID).Scan(
		&svc.ID, &svc.TeamID, &svc.EnvironmentID, &svc.ServerID, &svc.DestinationID, &svc.Name, &svc.Description,
		&svc.ServiceType, &svc.DockerComposeRaw, &svc.DockerCompose, &svc.FQDN, &svc.Status, &svc.CreatedAt, &svc.IsForceHTTPS,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &svc, err
}

func (s *Store) ListPreviewsIncludingDeleted(ctx context.Context, teamID, appID uuid.UUID) ([]ApplicationPreview, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, team_id, application_id, pull_request_id, pull_request_title, git_branch, fqdn, status, created_at
		FROM application_previews WHERE team_id=$1 AND application_id=$2 ORDER BY pull_request_id DESC
	`, teamID, appID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ApplicationPreview
	for rows.Next() {
		var p ApplicationPreview
		if err := rows.Scan(&p.ID, &p.TeamID, &p.ApplicationID, &p.PullRequestID, &p.PullRequestTitle, &p.GitBranch, &p.FQDN, &p.Status, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) GetPreviewByID(ctx context.Context, teamID, id uuid.UUID) (*ApplicationPreview, error) {
	var p ApplicationPreview
	err := s.Pool.QueryRow(ctx, `
		SELECT id, team_id, application_id, pull_request_id, pull_request_title, git_branch, fqdn, status, created_at
		FROM application_previews WHERE id=$1 AND team_id=$2
	`, id, teamID).Scan(&p.ID, &p.TeamID, &p.ApplicationID, &p.PullRequestID, &p.PullRequestTitle, &p.GitBranch, &p.FQDN, &p.Status, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &p, err
}

func (s *Store) LatestDeletePayload(ctx context.Context, teamID, resourceID uuid.UUID) ([]byte, error) {
	var payload []byte
	err := s.Pool.QueryRow(ctx, `
		SELECT payload FROM background_jobs
		WHERE team_id=$1 AND kind='resource_delete' AND payload->>'resource_id'=$2
		ORDER BY created_at DESC LIMIT 1
	`, teamID, resourceID.String()).Scan(&payload)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return payload, err
}

func MarshalDeletionPayload(p DeletionPayload) []byte {
	b, err := json.Marshal(p)
	if err != nil {
		return []byte("{}")
	}
	return b
}
