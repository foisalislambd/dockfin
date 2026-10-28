package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// GithubRunnerConfig is one server's GitHub Actions runner setup.
type GithubRunnerConfig struct {
	ID                 uuid.UUID `json:"id"`
	TeamID             uuid.UUID `json:"team_id"`
	ServerID           uuid.UUID `json:"server_id"`
	GitSourceID        uuid.UUID `json:"git_source_id"`
	GitSourceName      string    `json:"git_source_name,omitempty"`
	Organization       string    `json:"organization,omitempty"`
	Labels             []string  `json:"labels"`
	Enabled            bool      `json:"enabled"`
	AllowPullRequests  bool      `json:"allow_pull_requests"`
	MaxRunners         int       `json:"max_runners"`
	DockerMode         string    `json:"docker_mode"`
	RunnerImage        string    `json:"runner_image"`
	CPULimit           string    `json:"cpu_limit"`
	MemoryLimit        string    `json:"memory_limit"`
	QueueWaitMinutes   int       `json:"queue_wait_minutes"`
	IdleTimeoutMinutes int       `json:"idle_timeout_minutes"`
	JobTimeoutMinutes  int       `json:"job_timeout_minutes"`
}

// GithubRunnerExecution is one ephemeral runner or a job waiting for one.
type GithubRunnerExecution struct {
	ID                uuid.UUID  `json:"id"`
	TeamID            uuid.UUID  `json:"team_id"`
	GitSourceID       uuid.UUID  `json:"git_source_id"`
	ConfigID          *uuid.UUID `json:"config_id,omitempty"`
	ServerID          *uuid.UUID `json:"server_id,omitempty"`
	Status            string     `json:"status"`
	TriggerJobID      int64      `json:"trigger_job_id"`
	WorkflowJobID     *int64     `json:"workflow_job_id,omitempty"`
	HTMLURL           string     `json:"html_url,omitempty"`
	WorkflowName      string     `json:"workflow_name,omitempty"`
	JobName           string     `json:"job_name,omitempty"`
	Repository        string     `json:"repository,omitempty"`
	Labels            []string   `json:"labels"`
	IsPullRequest     bool       `json:"is_pull_request"`
	RunnerName        string     `json:"runner_name,omitempty"`
	RunnerID          *int64     `json:"runner_id,omitempty"`
	Conclusion        string     `json:"conclusion,omitempty"`
	ErrorMessage      string     `json:"error_message,omitempty"`
	ProvisionAttempts int        `json:"provision_attempts"`
	QueueWaitMinutes  int        `json:"queue_wait_minutes"`
	QueuedAt          *time.Time `json:"queued_at,omitempty"`
	ReadyAt           *time.Time `json:"ready_at,omitempty"`
	StartedAt         *time.Time `json:"started_at,omitempty"`
	CompletedAt       *time.Time `json:"completed_at,omitempty"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// RunnerMaintenance is an execution the scheduler should provision, finish, or remove.
type RunnerMaintenance struct {
	Execution          GithubRunnerExecution
	IdleTimeoutMinutes int
	JobTimeoutMinutes  int
}

type RunnerExecutionInput struct {
	TeamID           uuid.UUID
	GitSourceID      uuid.UUID
	TriggerJobID     int64
	HTMLURL          string
	WorkflowName     string
	JobName          string
	Repository       string
	Labels           []string
	IsPullRequest    bool
	QueueWaitMinutes int
}

// ReservedRunner is a queued execution locked onto one server.
type ReservedRunner struct {
	ExecutionID uuid.UUID
	TeamID      uuid.UUID
	ServerID    uuid.UUID
	GitSourceID uuid.UUID
	Config      GithubRunnerConfig
	RunnerName  string
	Attempts    int
}

func (s *Store) GetGithubRunnerConfig(ctx context.Context, teamID, serverID uuid.UUID) (*GithubRunnerConfig, error) {
	if _, err := s.GetServer(ctx, teamID, serverID); err != nil {
		return nil, err
	}
	row := s.Pool.QueryRow(ctx, `
		SELECT c.id, c.team_id, c.server_id, c.git_source_id, COALESCE(g.name,''), COALESCE(g.organization,''),
		       c.labels, c.enabled, c.allow_pull_requests, c.max_runners, c.docker_mode, c.runner_image,
		       c.cpu_limit, c.memory_limit, c.queue_wait_minutes, c.idle_timeout_minutes, c.job_timeout_minutes
		FROM github_runner_configs c
		JOIN git_sources g ON g.id = c.git_source_id
		WHERE c.team_id=$1 AND c.server_id=$2
	`, teamID, serverID)
	cfg, err := scanRunnerConfig(row.Scan)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return cfg, err
}

func (s *Store) SaveGithubRunnerConfig(ctx context.Context, cfg GithubRunnerConfig) (*GithubRunnerConfig, error) {
	if _, err := s.GetServer(ctx, cfg.TeamID, cfg.ServerID); err != nil {
		return nil, err
	}
	var sourceTeam uuid.UUID
	err := s.Pool.QueryRow(ctx, `SELECT team_id FROM git_sources WHERE id=$1`, cfg.GitSourceID).Scan(&sourceTeam)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && sourceTeam != cfg.TeamID) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	labels, _ := json.Marshal(cfg.Labels)
	row := s.Pool.QueryRow(ctx, `
		INSERT INTO github_runner_configs (
			team_id, server_id, git_source_id, labels, enabled, allow_pull_requests, max_runners,
			docker_mode, runner_image, cpu_limit, memory_limit, queue_wait_minutes, idle_timeout_minutes, job_timeout_minutes
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		ON CONFLICT (server_id) DO UPDATE SET
			git_source_id=EXCLUDED.git_source_id,
			labels=EXCLUDED.labels,
			enabled=EXCLUDED.enabled,
			allow_pull_requests=EXCLUDED.allow_pull_requests,
			max_runners=EXCLUDED.max_runners,
			docker_mode=EXCLUDED.docker_mode,
			runner_image=EXCLUDED.runner_image,
			cpu_limit=EXCLUDED.cpu_limit,
			memory_limit=EXCLUDED.memory_limit,
			queue_wait_minutes=EXCLUDED.queue_wait_minutes,
			idle_timeout_minutes=EXCLUDED.idle_timeout_minutes,
			job_timeout_minutes=EXCLUDED.job_timeout_minutes,
			updated_at=NOW()
		WHERE github_runner_configs.team_id=EXCLUDED.team_id
		RETURNING id
	`, cfg.TeamID, cfg.ServerID, cfg.GitSourceID, labels, cfg.Enabled, cfg.AllowPullRequests, cfg.MaxRunners,
		cfg.DockerMode, cfg.RunnerImage, cfg.CPULimit, cfg.MemoryLimit, cfg.QueueWaitMinutes, cfg.IdleTimeoutMinutes, cfg.JobTimeoutMinutes)
	var id uuid.UUID
	if err := row.Scan(&id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return s.GetGithubRunnerConfig(ctx, cfg.TeamID, cfg.ServerID)
}

func (s *Store) ListGithubRunnerExecutions(ctx context.Context, teamID, serverID uuid.UUID, limit int) ([]GithubRunnerExecution, error) {
	cfg, err := s.GetGithubRunnerConfig(ctx, teamID, serverID)
	if errors.Is(err, ErrNotFound) {
		return []GithubRunnerExecution{}, nil
	}
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 50 {
		limit = 30
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT `+runnerExecCols+`
		FROM github_runner_executions
		WHERE team_id=$1 AND (server_id=$2 OR config_id=$3 OR (status='queued' AND git_source_id=$4))
		ORDER BY created_at DESC
		LIMIT $5
	`, teamID, serverID, cfg.ID, cfg.GitSourceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GithubRunnerExecution
	for rows.Next() {
		row, err := scanRunnerExec(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, *row)
	}
	if out == nil {
		out = []GithubRunnerExecution{}
	}
	return out, rows.Err()
}

// CreateRunnerExecution inserts a queued job once. created is false when the job already exists.
func (s *Store) CreateRunnerExecution(ctx context.Context, in RunnerExecutionInput) (uuid.UUID, bool, error) {
	labels, _ := json.Marshal(in.Labels)
	if in.QueueWaitMinutes <= 0 {
		in.QueueWaitMinutes = 60
	}
	var id uuid.UUID
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO github_runner_executions (
			team_id, git_source_id, trigger_job_id, html_url, workflow_name, job_name, repository,
			labels, is_pull_request, queue_wait_minutes, status, queued_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'queued',NOW())
		ON CONFLICT (git_source_id, trigger_job_id) DO NOTHING
		RETURNING id
	`, in.TeamID, in.GitSourceID, in.TriggerJobID, in.HTMLURL, in.WorkflowName, in.JobName, in.Repository,
		labels, in.IsPullRequest, in.QueueWaitMinutes).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, false, nil
	}
	return id, err == nil, err
}

func (s *Store) ReserveRunner(ctx context.Context, executionID uuid.UUID, runnerName string, match func(custom, requested []string) bool) (*ReservedRunner, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var exec GithubRunnerExecution
	row, err := scanRunnerExec(tx.QueryRow(ctx, `
		SELECT `+runnerExecCols+` FROM github_runner_executions WHERE id=$1 FOR UPDATE
	`, executionID).Scan)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && row.Status != "queued") {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	exec = *row
	configs, err := s.runnerConfigsTx(ctx, tx, exec.GitSourceID)
	if err != nil {
		return nil, err
	}
	var chosen *GithubRunnerConfig
	chosenOccupied := 0
	for i := range configs {
		cfg := &configs[i]
		if !cfg.Enabled || (exec.IsPullRequest && !cfg.AllowPullRequests) || match == nil || !match(cfg.Labels, exec.Labels) {
			continue
		}
		var occupied int
		if err := tx.QueryRow(ctx, `
			SELECT COUNT(*) FROM github_runner_executions
			WHERE config_id=$1 AND status IN ('provisioning','idle','running')
		`, cfg.ID).Scan(&occupied); err != nil {
			return nil, err
		}
		if occupied >= cfg.MaxRunners {
			continue
		}
		if chosen == nil || occupied < chosenOccupied {
			chosen = cfg
			chosenOccupied = occupied
		}
	}
	if chosen == nil {
		return nil, nil
	}
	attempts := exec.ProvisionAttempts + 1
	tag, err := tx.Exec(ctx, `
		UPDATE github_runner_executions SET
			config_id=$2, server_id=$3, status='provisioning', runner_name=$4,
			provision_attempts=$5, updated_at=NOW()
		WHERE id=$1 AND status='queued'
	`, executionID, chosen.ID, chosen.ServerID, runnerName, attempts)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, nil
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &ReservedRunner{
		ExecutionID: executionID,
		TeamID:      exec.TeamID,
		ServerID:    chosen.ServerID,
		GitSourceID: exec.GitSourceID,
		Config:      *chosen,
		RunnerName:  runnerName,
		Attempts:    attempts,
	}, nil
}

func (s *Store) runnerConfigsTx(ctx context.Context, tx pgx.Tx, sourceID uuid.UUID) ([]GithubRunnerConfig, error) {
	rows, err := tx.Query(ctx, `
		SELECT c.id, c.team_id, c.server_id, c.git_source_id, COALESCE(g.name,''), COALESCE(g.organization,''),
		       c.labels, c.enabled, c.allow_pull_requests, c.max_runners, c.docker_mode, c.runner_image,
		       c.cpu_limit, c.memory_limit, c.queue_wait_minutes, c.idle_timeout_minutes, c.job_timeout_minutes
		FROM github_runner_configs c
		JOIN git_sources g ON g.id = c.git_source_id
		WHERE c.git_source_id=$1
		ORDER BY c.id
		FOR UPDATE OF c
	`, sourceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GithubRunnerConfig
	for rows.Next() {
		cfg, err := scanRunnerConfig(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, *cfg)
	}
	return out, rows.Err()
}

func (s *Store) SetRunnerResult(ctx context.Context, id uuid.UUID, runnerID int64, status, errMsg string) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE github_runner_executions SET
			runner_id=CASE WHEN $2 > 0 THEN $2 ELSE runner_id END,
			status=$3,
			error_message=$4,
			ready_at=CASE WHEN $3='idle' THEN COALESCE(ready_at, NOW()) ELSE ready_at END,
			completed_at=CASE WHEN $3 IN ('failed','cancelled','timed_out','completed') THEN COALESCE(completed_at, NOW()) ELSE completed_at END,
			updated_at=NOW()
		WHERE id=$1 AND status IN ('queued','provisioning','idle','running')
	`, id, runnerID, status, trimErr(errMsg))
	return err
}

func (s *Store) RequeueRunner(ctx context.Context, id uuid.UUID, errMsg string) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE github_runner_executions SET
			status='queued', config_id=NULL, server_id=NULL, runner_name='', runner_id=NULL,
			ready_at=NULL, error_message=$2, updated_at=NOW()
		WHERE id=$1 AND status='provisioning'
	`, id, trimErr(errMsg))
	return err
}

// MarkRunnerRunning records that GitHub gave this container a job.
// When that job is not the one the runner was started for, the original job is queued again.
func (s *Store) MarkRunnerRunning(ctx context.Context, gitSourceID uuid.UUID, runnerName string, jobID int64, htmlURL, workflow, job, repo string) (uuid.UUID, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer tx.Rollback(ctx)
	row, err := scanRunnerExec(tx.QueryRow(ctx, `
		SELECT `+runnerExecCols+`
		FROM github_runner_executions
		WHERE git_source_id=$1 AND runner_name=$2
		FOR UPDATE
	`, gitSourceID, runnerName).Scan)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, nil
	}
	if err != nil {
		return uuid.Nil, err
	}
	if row.Status != "provisioning" && row.Status != "idle" && row.Status != "running" {
		return uuid.Nil, nil
	}
	requeue := uuid.Nil
	if row.TriggerJobID != jobID {
		var otherStatus string
		otherErr := tx.QueryRow(ctx, `
			SELECT status FROM github_runner_executions
			WHERE git_source_id=$1 AND trigger_job_id=$2 AND id<>$3
			FOR UPDATE
		`, gitSourceID, jobID, row.ID).Scan(&otherStatus)
		otherActive := otherErr == nil && (otherStatus == "queued" || otherStatus == "provisioning" || otherStatus == "idle" || otherStatus == "running")
		if !otherActive {
			if otherErr == nil {
				_, _ = tx.Exec(ctx, `DELETE FROM github_runner_executions WHERE git_source_id=$1 AND trigger_job_id=$2 AND id<>$3`, gitSourceID, jobID, row.ID)
			}
			if row.ProvisionAttempts < 3 {
				labels, _ := json.Marshal(row.Labels)
				var newID uuid.UUID
				err = tx.QueryRow(ctx, `
					INSERT INTO github_runner_executions (
						team_id, git_source_id, trigger_job_id, html_url, workflow_name, job_name, repository,
						labels, is_pull_request, queue_wait_minutes, provision_attempts, status, queued_at
					) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,'queued',NOW())
					RETURNING id
				`, row.TeamID, row.GitSourceID, row.TriggerJobID, row.HTMLURL, row.WorkflowName, row.JobName, row.Repository,
					labels, row.IsPullRequest, row.QueueWaitMinutes, row.ProvisionAttempts).Scan(&newID)
				if err != nil {
					return uuid.Nil, err
				}
				requeue = newID
			}
			_, err = tx.Exec(ctx, `UPDATE github_runner_executions SET trigger_job_id=$2 WHERE id=$1`, row.ID, jobID)
			if err != nil {
				return uuid.Nil, err
			}
		}
	}
	_, err = tx.Exec(ctx, `
		UPDATE github_runner_executions SET
			status='running', workflow_job_id=$2, html_url=COALESCE(NULLIF($3,''), html_url),
			workflow_name=COALESCE(NULLIF($4,''), workflow_name), job_name=COALESCE(NULLIF($5,''), job_name),
			repository=COALESCE(NULLIF($6,''), repository), started_at=COALESCE(started_at, NOW()), updated_at=NOW()
		WHERE id=$1
	`, row.ID, jobID, htmlURL, workflow, job, repo)
	if err != nil {
		return uuid.Nil, err
	}
	return requeue, tx.Commit(ctx)
}

func (s *Store) FinishRunnerByName(ctx context.Context, gitSourceID uuid.UUID, runnerName, conclusion string) (*GithubRunnerExecution, error) {
	row, err := scanRunnerExec(s.Pool.QueryRow(ctx, `
		UPDATE github_runner_executions SET
			status='completed', conclusion=$3, completed_at=COALESCE(completed_at, NOW()), updated_at=NOW()
		WHERE git_source_id=$1 AND runner_name=$2 AND status IN ('provisioning','idle','running','failed')
		RETURNING `+runnerExecCols+`
	`, gitSourceID, runnerName, conclusion).Scan)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return row, err
}

func (s *Store) CancelQueuedRunner(ctx context.Context, gitSourceID uuid.UUID, jobID int64, message string) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE github_runner_executions SET
			status='cancelled', error_message=$3, completed_at=COALESCE(completed_at, NOW()), updated_at=NOW()
		WHERE git_source_id=$1 AND trigger_job_id=$2 AND status='queued'
	`, gitSourceID, jobID, trimErr(message))
	return err
}

func (s *Store) ListRunnerMaintenance(ctx context.Context) ([]RunnerMaintenance, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT `+runnerExecColsE+`,
		       COALESCE(c.idle_timeout_minutes, 10), COALESCE(c.job_timeout_minutes, 360)
		FROM github_runner_executions e
		LEFT JOIN github_runner_configs c ON c.id = e.config_id
		WHERE e.status IN ('queued','provisioning','idle','running')
		ORDER BY e.updated_at
		LIMIT 50
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RunnerMaintenance
	for rows.Next() {
		var item RunnerMaintenance
		row, err := scanRunnerExec(func(dest ...any) error {
			return rows.Scan(append(dest, &item.IdleTimeoutMinutes, &item.JobTimeoutMinutes)...)
		})
		if err != nil {
			return nil, err
		}
		item.Execution = *row
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) GitSourceRunnerGroup(ctx context.Context, sourceID uuid.UUID) (int64, error) {
	var id *int64
	err := s.Pool.QueryRow(ctx, `SELECT runner_group_id FROM git_sources WHERE id=$1`, sourceID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil || id == nil {
		return 0, err
	}
	return *id, nil
}

func (s *Store) SetGitSourceRunnerGroup(ctx context.Context, sourceID uuid.UUID, groupID int64) error {
	_, err := s.Pool.Exec(ctx, `UPDATE git_sources SET runner_group_id=$2, updated_at=NOW() WHERE id=$1`, sourceID, groupID)
	return err
}

func (s *Store) EnabledRunnerConfigsForSource(ctx context.Context, sourceID uuid.UUID) ([]GithubRunnerConfig, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT c.id, c.team_id, c.server_id, c.git_source_id, COALESCE(g.name,''), COALESCE(g.organization,''),
		       c.labels, c.enabled, c.allow_pull_requests, c.max_runners, c.docker_mode, c.runner_image,
		       c.cpu_limit, c.memory_limit, c.queue_wait_minutes, c.idle_timeout_minutes, c.job_timeout_minutes
		FROM github_runner_configs c
		JOIN git_sources g ON g.id = c.git_source_id
		WHERE c.git_source_id=$1 AND c.enabled
		ORDER BY c.id
	`, sourceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GithubRunnerConfig
	for rows.Next() {
		cfg, err := scanRunnerConfig(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, *cfg)
	}
	return out, rows.Err()
}

const runnerExecCols = `id, team_id, git_source_id, config_id, server_id, status, trigger_job_id,
	workflow_job_id, html_url, workflow_name, job_name, repository, labels, is_pull_request,
	runner_name, runner_id, conclusion, error_message, provision_attempts, queue_wait_minutes,
	queued_at, ready_at, started_at, completed_at, updated_at`

const runnerExecColsE = `e.id, e.team_id, e.git_source_id, e.config_id, e.server_id, e.status, e.trigger_job_id,
	e.workflow_job_id, e.html_url, e.workflow_name, e.job_name, e.repository, e.labels, e.is_pull_request,
	e.runner_name, e.runner_id, e.conclusion, e.error_message, e.provision_attempts, e.queue_wait_minutes,
	e.queued_at, e.ready_at, e.started_at, e.completed_at, e.updated_at`

func scanRunnerConfig(scan func(dest ...any) error) (*GithubRunnerConfig, error) {
	var cfg GithubRunnerConfig
	var labels []byte
	err := scan(
		&cfg.ID, &cfg.TeamID, &cfg.ServerID, &cfg.GitSourceID, &cfg.GitSourceName, &cfg.Organization,
		&labels, &cfg.Enabled, &cfg.AllowPullRequests, &cfg.MaxRunners, &cfg.DockerMode, &cfg.RunnerImage,
		&cfg.CPULimit, &cfg.MemoryLimit, &cfg.QueueWaitMinutes, &cfg.IdleTimeoutMinutes, &cfg.JobTimeoutMinutes,
	)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(labels, &cfg.Labels)
	if cfg.Labels == nil {
		cfg.Labels = []string{}
	}
	return &cfg, nil
}

func scanRunnerExec(scan func(dest ...any) error) (*GithubRunnerExecution, error) {
	var row GithubRunnerExecution
	var labels []byte
	err := scan(
		&row.ID, &row.TeamID, &row.GitSourceID, &row.ConfigID, &row.ServerID, &row.Status, &row.TriggerJobID,
		&row.WorkflowJobID, &row.HTMLURL, &row.WorkflowName, &row.JobName, &row.Repository, &labels, &row.IsPullRequest,
		&row.RunnerName, &row.RunnerID, &row.Conclusion, &row.ErrorMessage, &row.ProvisionAttempts, &row.QueueWaitMinutes,
		&row.QueuedAt, &row.ReadyAt, &row.StartedAt, &row.CompletedAt, &row.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(labels, &row.Labels)
	if row.Labels == nil {
		row.Labels = []string{}
	}
	return &row, nil
}

func trimErr(msg string) string {
	if len(msg) > 500 {
		return msg[:500]
	}
	return msg
}
