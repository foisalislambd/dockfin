package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type BackgroundJob struct {
	ID          uuid.UUID       `json:"id"`
	TeamID      uuid.UUID       `json:"team_id"`
	Kind        string          `json:"kind"`
	Payload     json.RawMessage `json:"payload"`
	Status      string          `json:"status"`
	Attempts    int             `json:"attempts"`
	MaxAttempts int             `json:"max_attempts"`
	RunAfter    time.Time       `json:"run_after"`
	Error       string          `json:"error,omitempty"`
	Result      json.RawMessage `json:"result"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

func (s *Store) EnqueueJob(ctx context.Context, teamID uuid.UUID, kind string, payload []byte) (*BackgroundJob, error) {
	if len(payload) == 0 {
		payload = []byte("{}")
	}
	var job BackgroundJob
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO background_jobs (team_id, kind, payload)
		VALUES ($1,$2,$3)
		RETURNING id, team_id, kind, payload, status, attempts, max_attempts, run_after, error, result, created_at, updated_at
	`, teamID, kind, payload).Scan(
		&job.ID, &job.TeamID, &job.Kind, &job.Payload, &job.Status, &job.Attempts, &job.MaxAttempts,
		&job.RunAfter, &job.Error, &job.Result, &job.CreatedAt, &job.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func (s *Store) GetJob(ctx context.Context, teamID, id uuid.UUID) (*BackgroundJob, error) {
	var job BackgroundJob
	err := s.Pool.QueryRow(ctx, `
		SELECT id, team_id, kind, payload, status, attempts, max_attempts, run_after, error, result, created_at, updated_at
		FROM background_jobs WHERE team_id=$1 AND id=$2
	`, teamID, id).Scan(
		&job.ID, &job.TeamID, &job.Kind, &job.Payload, &job.Status, &job.Attempts, &job.MaxAttempts,
		&job.RunAfter, &job.Error, &job.Result, &job.CreatedAt, &job.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if len(job.Result) == 0 {
		job.Result = []byte("{}")
	}
	return &job, nil
}

func (s *Store) ListJobs(ctx context.Context, teamID uuid.UUID, limit int) ([]BackgroundJob, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT id, team_id, kind, payload, status, attempts, max_attempts, run_after, error, result, created_at, updated_at
		FROM background_jobs WHERE team_id=$1 ORDER BY created_at DESC LIMIT $2
	`, teamID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BackgroundJob
	for rows.Next() {
		var job BackgroundJob
		if err := rows.Scan(
			&job.ID, &job.TeamID, &job.Kind, &job.Payload, &job.Status, &job.Attempts, &job.MaxAttempts,
			&job.RunAfter, &job.Error, &job.Result, &job.CreatedAt, &job.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, job)
	}
	if out == nil {
		out = []BackgroundJob{}
	}
	return out, rows.Err()
}

// ClaimJob locks one queued job. locked_until is a lease so a crashed worker
// can be reclaimed after it expires.
func (s *Store) ClaimJob(ctx context.Context) (*BackgroundJob, error) {
	var job BackgroundJob
	err := s.Pool.QueryRow(ctx, `
		UPDATE background_jobs SET
			status='running',
			attempts=attempts+1,
			locked_until=NOW() + INTERVAL '12 minutes',
			updated_at=NOW()
		WHERE id = (
			SELECT id FROM background_jobs
			WHERE status='queued' AND run_after <= NOW()
			ORDER BY created_at
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		RETURNING id, team_id, kind, payload, status, attempts, max_attempts, run_after, error, result, created_at, updated_at
	`).Scan(
		&job.ID, &job.TeamID, &job.Kind, &job.Payload, &job.Status, &job.Attempts, &job.MaxAttempts,
		&job.RunAfter, &job.Error, &job.Result, &job.CreatedAt, &job.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func (s *Store) RequeueExpiredJobs(ctx context.Context) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE background_jobs
		SET status='queued', locked_until=NULL, updated_at=NOW(),
		    run_after=NOW() + INTERVAL '15 seconds',
		    error='worker stopped before the job finished'
		WHERE status='running' AND locked_until IS NOT NULL AND locked_until < NOW() AND attempts < max_attempts
	`)
	if err != nil {
		return err
	}
	_, err = s.Pool.Exec(ctx, `
		UPDATE background_jobs
		SET status='failed', updated_at=NOW(), error='worker stopped before the job finished'
		WHERE status='running' AND locked_until IS NOT NULL AND locked_until < NOW() AND attempts >= max_attempts
	`)
	return err
}

func (s *Store) FinishJob(ctx context.Context, id uuid.UUID, result []byte) error {
	if len(result) == 0 {
		result = []byte("{}")
	}
	_, err := s.Pool.Exec(ctx, `
		UPDATE background_jobs
		SET status='succeeded', result=$2, error='', locked_until=NULL, updated_at=NOW()
		WHERE id=$1 AND status='running'
	`, id, result)
	return err
}

func (s *Store) FailJob(ctx context.Context, id uuid.UUID, attempts, maxAttempts int, message string) error {
	if attempts < maxAttempts {
		backoff := time.Duration(attempts*attempts) * 10 * time.Second
		_, err := s.Pool.Exec(ctx, `
			UPDATE background_jobs
			SET status='queued', error=$2, run_after=$3, locked_until=NULL, updated_at=NOW()
			WHERE id=$1 AND status='running'
		`, id, message, time.Now().UTC().Add(backoff))
		return err
	}
	_, err := s.Pool.Exec(ctx, `
		UPDATE background_jobs
		SET status='failed', error=$2, locked_until=NULL, updated_at=NOW()
		WHERE id=$1 AND status='running'
	`, id, message)
	return err
}

func (s *Store) CancelJob(ctx context.Context, teamID, id uuid.UUID) error {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE background_jobs SET status='cancelled', updated_at=NOW()
		WHERE team_id=$1 AND id=$2 AND status='queued'
	`, teamID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
