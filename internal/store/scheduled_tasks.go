package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type ScheduledTask struct {
	ID           uuid.UUID `json:"id"`
	TeamID       uuid.UUID `json:"team_id"`
	ResourceType string    `json:"resource_type"`
	ResourceID   uuid.UUID `json:"resource_id"`
	Name         string    `json:"name"`
	Command      string    `json:"command"`
	Frequency    string    `json:"frequency"`
	Container    string    `json:"container_name"`
	Enabled      bool      `json:"enabled"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type ScheduledTaskExecution struct {
	ID         uuid.UUID  `json:"id"`
	Status     string     `json:"status"`
	Output     string     `json:"output"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at"`
}

func (s *Store) ListScheduledTasks(ctx context.Context, teamID uuid.UUID, resourceType string, resourceID *uuid.UUID) ([]ScheduledTask, error) {
	q := `
		SELECT id, team_id, resource_type, resource_id, name, command, frequency, COALESCE(container_name,''), enabled, created_at, updated_at
		FROM scheduled_tasks WHERE team_id=$1`
	args := []any{teamID}
	if resourceType != "" {
		args = append(args, resourceType)
		q += fmt.Sprintf(` AND resource_type=$%d`, len(args))
	}
	if resourceID != nil {
		args = append(args, *resourceID)
		q += fmt.Sprintf(` AND resource_id=$%d`, len(args))
	}
	q += ` ORDER BY name`
	rows, err := s.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ScheduledTask
	for rows.Next() {
		var t ScheduledTask
		if err := rows.Scan(&t.ID, &t.TeamID, &t.ResourceType, &t.ResourceID, &t.Name, &t.Command, &t.Frequency, &t.Container, &t.Enabled, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	if out == nil {
		out = []ScheduledTask{}
	}
	return out, rows.Err()
}

func (s *Store) GetScheduledTask(ctx context.Context, teamID, id uuid.UUID) (*ScheduledTask, error) {
	var t ScheduledTask
	err := s.Pool.QueryRow(ctx, `
		SELECT id, team_id, resource_type, resource_id, name, command, frequency, COALESCE(container_name,''), enabled, created_at, updated_at
		FROM scheduled_tasks WHERE id=$1 AND team_id=$2
	`, id, teamID).Scan(&t.ID, &t.TeamID, &t.ResourceType, &t.ResourceID, &t.Name, &t.Command, &t.Frequency, &t.Container, &t.Enabled, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *Store) CreateScheduledTask(ctx context.Context, teamID uuid.UUID, resourceType string, resourceID uuid.UUID, name, command, frequency, container string) (*ScheduledTask, error) {
	var t ScheduledTask
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO scheduled_tasks (team_id, resource_type, resource_id, name, command, frequency, container_name)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING id, team_id, resource_type, resource_id, name, command, frequency, COALESCE(container_name,''), enabled, created_at, updated_at
	`, teamID, resourceType, resourceID, name, command, frequency, container).Scan(
		&t.ID, &t.TeamID, &t.ResourceType, &t.ResourceID, &t.Name, &t.Command, &t.Frequency, &t.Container, &t.Enabled, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

type UpdateScheduledTaskInput struct {
	Name      *string
	Command   *string
	Frequency *string
	Container *string
	Enabled   *bool
}

func (s *Store) UpdateScheduledTask(ctx context.Context, teamID, id uuid.UUID, in UpdateScheduledTaskInput) (*ScheduledTask, error) {
	cur, err := s.GetScheduledTask(ctx, teamID, id)
	if err != nil {
		return nil, err
	}
	if in.Name != nil {
		cur.Name = *in.Name
	}
	if in.Command != nil {
		cur.Command = *in.Command
	}
	if in.Frequency != nil {
		cur.Frequency = *in.Frequency
	}
	if in.Container != nil {
		cur.Container = *in.Container
	}
	if in.Enabled != nil {
		cur.Enabled = *in.Enabled
	}
	err = s.Pool.QueryRow(ctx, `
		UPDATE scheduled_tasks
		SET name=$3, command=$4, frequency=$5, container_name=$6, enabled=$7, updated_at=NOW()
		WHERE id=$1 AND team_id=$2
		RETURNING id, team_id, resource_type, resource_id, name, command, frequency, COALESCE(container_name,''), enabled, created_at, updated_at
	`, id, teamID, cur.Name, cur.Command, cur.Frequency, cur.Container, cur.Enabled).Scan(
		&cur.ID, &cur.TeamID, &cur.ResourceType, &cur.ResourceID, &cur.Name, &cur.Command, &cur.Frequency, &cur.Container, &cur.Enabled, &cur.CreatedAt, &cur.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return cur, nil
}

func (s *Store) DeleteScheduledTask(ctx context.Context, teamID, id uuid.UUID) error {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM scheduled_tasks WHERE id=$1 AND team_id=$2`, id, teamID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ListScheduledTaskExecutions(ctx context.Context, teamID, taskID uuid.UUID, limit int) ([]ScheduledTaskExecution, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT id, status, COALESCE(output,''), started_at, finished_at
		FROM scheduled_task_executions
		WHERE team_id=$1 AND scheduled_task_id=$2
		ORDER BY started_at DESC
		LIMIT $3
	`, teamID, taskID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ScheduledTaskExecution
	for rows.Next() {
		var e ScheduledTaskExecution
		if err := rows.Scan(&e.ID, &e.Status, &e.Output, &e.StartedAt, &e.FinishedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	if out == nil {
		out = []ScheduledTaskExecution{}
	}
	return out, rows.Err()
}

// ScheduledTaskExecutionWithTask carries the parent task's identity alongside an execution row,
// used for the team-wide "recent failures" view (Settings → Scheduled Jobs).
type ScheduledTaskExecutionWithTask struct {
	ID           uuid.UUID  `json:"id"`
	Status       string     `json:"status"`
	Output       string     `json:"output"`
	StartedAt    time.Time  `json:"started_at"`
	FinishedAt   *time.Time `json:"finished_at"`
	TaskID       uuid.UUID  `json:"task_id"`
	TaskName     string     `json:"task_name"`
	ResourceType string     `json:"resource_type"`
	ResourceID   uuid.UUID  `json:"resource_id"`
}

// ListScheduledTaskExecutionsForTeam returns recent executions across all scheduled tasks for a
// team, optionally filtered by status (e.g. "failed"). Used by the Settings > Scheduled Jobs
// "recent issues" view.
func (s *Store) ListScheduledTaskExecutionsForTeam(ctx context.Context, teamID uuid.UUID, status string, limit int) ([]ScheduledTaskExecutionWithTask, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	q := `
		SELECT e.id, e.status, COALESCE(e.output,''), e.started_at, e.finished_at,
		       t.id, t.name, t.resource_type, t.resource_id
		FROM scheduled_task_executions e
		JOIN scheduled_tasks t ON t.id = e.scheduled_task_id
		WHERE e.team_id = $1`
	args := []any{teamID}
	if status != "" {
		args = append(args, status)
		q += fmt.Sprintf(" AND e.status = $%d", len(args))
	}
	args = append(args, limit)
	q += fmt.Sprintf(" ORDER BY e.started_at DESC LIMIT $%d", len(args))
	rows, err := s.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ScheduledTaskExecutionWithTask
	for rows.Next() {
		var e ScheduledTaskExecutionWithTask
		if err := rows.Scan(
			&e.ID, &e.Status, &e.Output, &e.StartedAt, &e.FinishedAt,
			&e.TaskID, &e.TaskName, &e.ResourceType, &e.ResourceID,
		); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	if out == nil {
		out = []ScheduledTaskExecutionWithTask{}
	}
	return out, rows.Err()
}

func (s *Store) GetServiceWebhookSecret(ctx context.Context, serviceID uuid.UUID) (teamID uuid.UUID, secret string, err error) {
	var enc string
	err = s.Pool.QueryRow(ctx, `
		SELECT team_id, COALESCE(webhook_secret_enc,'') FROM services WHERE id=$1 AND deleted_at IS NULL
	`, serviceID).Scan(&teamID, &enc)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, "", ErrNotFound
	}
	if err != nil {
		return uuid.Nil, "", err
	}
	if enc == "" {
		return teamID, "", nil
	}
	secret, err = s.Box.DecryptString(enc)
	return teamID, secret, err
}

func (s *Store) SetServiceWebhookSecret(ctx context.Context, teamID, serviceID uuid.UUID, secret string) error {
	enc, err := s.Box.EncryptString(secret)
	if err != nil {
		return err
	}
	tag, err := s.Pool.Exec(ctx, `
		UPDATE services SET webhook_secret_enc=$3, updated_at=NOW() WHERE id=$1 AND team_id=$2 AND deleted_at IS NULL
	`, serviceID, teamID, enc)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ServiceHasWebhookSecret(ctx context.Context, teamID, serviceID uuid.UUID) (bool, error) {
	var enc string
	err := s.Pool.QueryRow(ctx, `
		SELECT COALESCE(webhook_secret_enc,'') FROM services WHERE id=$1 AND team_id=$2 AND deleted_at IS NULL
	`, serviceID, teamID).Scan(&enc)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ErrNotFound
	}
	return enc != "", err
}

func (s *Store) GetServiceByID(ctx context.Context, id uuid.UUID) (*Service, error) {
	var svc Service
	err := s.Pool.QueryRow(ctx, `
		SELECT id, team_id, environment_id, server_id, destination_id, name, description, service_type, docker_compose_raw, COALESCE(docker_compose, ''), COALESCE(fqdn,''), status, created_at, COALESCE(is_force_https, TRUE)
		FROM services WHERE id=$1 AND deleted_at IS NULL
	`, id).Scan(
		&svc.ID, &svc.TeamID, &svc.EnvironmentID, &svc.ServerID, &svc.DestinationID, &svc.Name, &svc.Description,
		&svc.ServiceType, &svc.DockerComposeRaw, &svc.DockerCompose, &svc.FQDN, &svc.Status, &svc.CreatedAt, &svc.IsForceHTTPS,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &svc, nil
}
