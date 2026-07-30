package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Project struct {
	ID          uuid.UUID `json:"id"`
	TeamID      uuid.UUID `json:"team_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

type Environment struct {
	ID          uuid.UUID `json:"id"`
	TeamID      uuid.UUID `json:"team_id"`
	ProjectID   uuid.UUID `json:"project_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

type Application struct {
	ID                      uuid.UUID  `json:"id"`
	TeamID                  uuid.UUID  `json:"team_id"`
	EnvironmentID           uuid.UUID  `json:"environment_id"`
	DestinationID           *uuid.UUID `json:"destination_id"`
	Name                    string     `json:"name"`
	Description             string     `json:"description"`
	FQDN                    string     `json:"fqdn"`
	Status                  string     `json:"status"`
	BuildPack               string     `json:"build_pack"`
	GitRepository           string     `json:"git_repository"`
	GitBranch               string     `json:"git_branch"`
	GitCommitSHA            string     `json:"git_commit_sha"`
	DockerfileLocation      string     `json:"dockerfile_location"`
	DockerComposeLocation   string     `json:"docker_compose_location"`
	DockerRegistryImageName string     `json:"docker_registry_image_name"`
	DockerRegistryImageTag  string     `json:"docker_registry_image_tag"`
	PortsExposes            string     `json:"ports_exposes"`
	HealthCheckEnabled      bool       `json:"health_check_enabled"`
	HealthCheckPath         string     `json:"health_check_path"`
	HealthCheckPort         *int       `json:"health_check_port,omitempty"`
	HealthCheckMethod       string     `json:"health_check_method"`
	HealthCheckReturnCode   int        `json:"health_check_return_code"`
	HealthCheckInterval     int        `json:"health_check_interval"`
	HealthCheckTimeout      int        `json:"health_check_timeout"`
	HealthCheckRetries      int        `json:"health_check_retries"`
	LimitsMemory            string     `json:"limits_memory"`
	LimitsCpus              string     `json:"limits_cpus"`
	IsForceHTTPS            bool       `json:"is_force_https"`
	IsPreviewEnabled        bool       `json:"is_preview_enabled"`
	GitSourceID             *uuid.UUID `json:"git_source_id,omitempty"`
	IsBuildServerEnabled    bool       `json:"is_build_server_enabled"`
	CreatedAt               time.Time  `json:"created_at"`
}

type ApplicationPreview struct {
	ID               uuid.UUID `json:"id"`
	TeamID           uuid.UUID `json:"team_id"`
	ApplicationID    uuid.UUID `json:"application_id"`
	PullRequestID    int       `json:"pull_request_id"`
	PullRequestTitle string    `json:"pull_request_title"`
	GitBranch        string    `json:"git_branch"`
	FQDN             string    `json:"fqdn"`
	Status           string    `json:"status"`
	CreatedAt        time.Time `json:"created_at"`
}

const applicationSelectCols = `
	a.id, a.team_id, a.environment_id, a.destination_id, a.name, a.description, a.fqdn, a.status, a.build_pack,
	a.git_repository, a.git_branch, a.git_commit_sha, a.dockerfile_location, a.docker_compose_location,
	a.docker_registry_image_name, a.docker_registry_image_tag, a.ports_exposes,
	a.health_check_enabled, a.health_check_path, a.health_check_port, a.health_check_method,
	a.health_check_return_code, a.health_check_interval, a.health_check_timeout, a.health_check_retries,
	a.limits_memory, a.limits_cpus,
	a.git_source_id,
	COALESCE(s.is_build_server_enabled, FALSE),
	COALESCE(a.is_force_https, s.is_force_https_enabled, TRUE),
	COALESCE(s.is_preview_enabled, FALSE),
	a.created_at`

func scanApplication(scan func(dest ...any) error) (*Application, error) {
	var a Application
	err := scan(
		&a.ID, &a.TeamID, &a.EnvironmentID, &a.DestinationID, &a.Name, &a.Description, &a.FQDN, &a.Status, &a.BuildPack,
		&a.GitRepository, &a.GitBranch, &a.GitCommitSHA, &a.DockerfileLocation, &a.DockerComposeLocation,
		&a.DockerRegistryImageName, &a.DockerRegistryImageTag, &a.PortsExposes,
		&a.HealthCheckEnabled, &a.HealthCheckPath, &a.HealthCheckPort, &a.HealthCheckMethod,
		&a.HealthCheckReturnCode, &a.HealthCheckInterval, &a.HealthCheckTimeout, &a.HealthCheckRetries,
		&a.LimitsMemory, &a.LimitsCpus, &a.GitSourceID, &a.IsBuildServerEnabled, &a.IsForceHTTPS, &a.IsPreviewEnabled, &a.CreatedAt,
	)
	return &a, err
}

type Deployment struct {
	ID            uuid.UUID       `json:"id"`
	TeamID        uuid.UUID       `json:"team_id"`
	ApplicationID uuid.UUID       `json:"application_id"`
	ServerID      *uuid.UUID      `json:"server_id"`
	Status        string          `json:"status"`
	CommitSHA     string          `json:"commit_sha"`
	CommitMessage string          `json:"commit_message"`
	CurrentStage  string          `json:"current_stage"`
	Logs          json.RawMessage `json:"logs"`
	ErrorMessage  string          `json:"error_message"`
	StartedAt     *time.Time      `json:"started_at"`
	FinishedAt    *time.Time      `json:"finished_at"`
	CreatedAt     time.Time       `json:"created_at"`
}

type Database struct {
	ID            uuid.UUID       `json:"id"`
	TeamID        uuid.UUID       `json:"team_id"`
	EnvironmentID uuid.UUID       `json:"environment_id"`
	DestinationID *uuid.UUID      `json:"destination_id"`
	Name          string          `json:"name"`
	Description   string          `json:"description"`
	Engine        string          `json:"engine"`
	Image         string          `json:"image"`
	Status        string          `json:"status"`
	IsPublic      bool            `json:"is_public"`
	PublicPort    *int            `json:"public_port"`
	EngineConfig  json.RawMessage `json:"engine_config"`
	CreatedAt     time.Time       `json:"created_at"`
}

type Service struct {
	ID               uuid.UUID  `json:"id"`
	TeamID           uuid.UUID  `json:"team_id"`
	EnvironmentID    uuid.UUID  `json:"environment_id"`
	ServerID         *uuid.UUID `json:"server_id"`
	DestinationID    *uuid.UUID `json:"destination_id"`
	Name             string     `json:"name"`
	Description      string     `json:"description"`
	ServiceType      string     `json:"service_type"`
	DockerComposeRaw string     `json:"docker_compose_raw"`
	DockerCompose    string     `json:"docker_compose,omitempty"`
	FQDN             string     `json:"fqdn"`
	Status           string     `json:"status"`
	CreatedAt        time.Time  `json:"created_at"`
}

func (s *Store) CreateProject(ctx context.Context, teamID uuid.UUID, name, desc string) (*Project, *Environment, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback(ctx)
	var p Project
	err = tx.QueryRow(ctx, `
		INSERT INTO projects (team_id, name, description) VALUES ($1,$2,$3)
		RETURNING id, team_id, name, description, created_at
	`, teamID, name, desc).Scan(&p.ID, &p.TeamID, &p.Name, &p.Description, &p.CreatedAt)
	if err != nil {
		return nil, nil, err
	}
	var e Environment
	err = tx.QueryRow(ctx, `
		INSERT INTO environments (team_id, project_id, name) VALUES ($1,$2,'production')
		RETURNING id, team_id, project_id, name, description, created_at
	`, teamID, p.ID).Scan(&e.ID, &e.TeamID, &e.ProjectID, &e.Name, &e.Description, &e.CreatedAt)
	if err != nil {
		return nil, nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, nil, err
	}
	return &p, &e, nil
}

func (s *Store) ListProjects(ctx context.Context, teamID uuid.UUID) ([]Project, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, team_id, name, description, created_at FROM projects WHERE team_id=$1 ORDER BY name
	`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.TeamID, &p.Name, &p.Description, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) GetProject(ctx context.Context, teamID, id uuid.UUID) (*Project, error) {
	var p Project
	err := s.Pool.QueryRow(ctx, `
		SELECT id, team_id, name, description, created_at FROM projects WHERE id=$1 AND team_id=$2
	`, id, teamID).Scan(&p.ID, &p.TeamID, &p.Name, &p.Description, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &p, err
}

func (s *Store) ListEnvironments(ctx context.Context, teamID, projectID uuid.UUID) ([]Environment, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, team_id, project_id, name, description, created_at
		FROM environments WHERE team_id=$1 AND project_id=$2 ORDER BY name
	`, teamID, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Environment
	for rows.Next() {
		var e Environment
		if err := rows.Scan(&e.ID, &e.TeamID, &e.ProjectID, &e.Name, &e.Description, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) CreateEnvironment(ctx context.Context, teamID, projectID uuid.UUID, name, desc string) (*Environment, error) {
	var e Environment
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO environments (team_id, project_id, name, description) VALUES ($1,$2,$3,$4)
		RETURNING id, team_id, project_id, name, description, created_at
	`, teamID, projectID, name, desc).Scan(&e.ID, &e.TeamID, &e.ProjectID, &e.Name, &e.Description, &e.CreatedAt)
	return &e, err
}

func (s *Store) CreateApplication(ctx context.Context, app *Application) (*Application, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	err = tx.QueryRow(ctx, `
		INSERT INTO applications (
			team_id, environment_id, destination_id, name, description, fqdn, build_pack,
			git_repository, git_branch, dockerfile_location, docker_compose_location,
			docker_registry_image_name, docker_registry_image_tag, ports_exposes
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		RETURNING id
	`, app.TeamID, app.EnvironmentID, app.DestinationID, app.Name, app.Description, app.FQDN, app.BuildPack,
		app.GitRepository, app.GitBranch, app.DockerfileLocation, app.DockerComposeLocation,
		app.DockerRegistryImageName, app.DockerRegistryImageTag, app.PortsExposes,
	).Scan(&app.ID)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO application_settings (application_id) VALUES ($1)`, app.ID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetApplication(ctx, app.TeamID, app.ID)
}

func (s *Store) ListApplications(ctx context.Context, teamID uuid.UUID, environmentID *uuid.UUID) ([]Application, error) {
	q := `SELECT ` + applicationSelectCols + `
		FROM applications a
		LEFT JOIN application_settings s ON s.application_id = a.id
		WHERE a.team_id=$1`
	args := []any{teamID}
	if environmentID != nil {
		q += ` AND a.environment_id=$2`
		args = append(args, *environmentID)
	}
	q += ` ORDER BY a.name`
	rows, err := s.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Application
	for rows.Next() {
		a, err := scanApplication(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
	}
	return out, rows.Err()
}

func (s *Store) GetApplication(ctx context.Context, teamID, id uuid.UUID) (*Application, error) {
	row := s.Pool.QueryRow(ctx, `SELECT `+applicationSelectCols+`
		FROM applications a
		LEFT JOIN application_settings s ON s.application_id = a.id
		WHERE a.id=$1 AND a.team_id=$2`, id, teamID)
	a, err := scanApplication(row.Scan)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return a, err
}

func (s *Store) DeleteApplication(ctx context.Context, teamID, id uuid.UUID) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var exists uuid.UUID
	err = tx.QueryRow(ctx, `SELECT id FROM applications WHERE id=$1 AND team_id=$2`, id, teamID).Scan(&exists)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM environment_variables WHERE team_id=$1 AND resource_type='application' AND resource_id=$2`, teamID, id); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM scheduled_tasks WHERE team_id=$1 AND resource_type='application' AND resource_id=$2`, teamID, id); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM scheduled_backups WHERE team_id=$1 AND resource_type='application' AND resource_id=$2`, teamID, id); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM applications WHERE id=$1 AND team_id=$2`, id, teamID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) UpdateApplicationStatus(ctx context.Context, id uuid.UUID, status string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE applications SET status=$2, updated_at=NOW() WHERE id=$1`, id, status)
	return err
}

func (s *Store) UpdateApplication(ctx context.Context, app *Application) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE applications SET
			name=$2, description=$3, fqdn=$4, git_repository=$5, git_branch=$6,
			ports_exposes=$7, docker_registry_image_name=$8, docker_registry_image_tag=$9,
			dockerfile_location=$10, destination_id=$11, git_source_id=$12, updated_at=NOW()
		WHERE id=$1 AND team_id=$13
	`, app.ID, app.Name, app.Description, app.FQDN, app.GitRepository, app.GitBranch,
		app.PortsExposes, app.DockerRegistryImageName, app.DockerRegistryImageTag,
		app.DockerfileLocation, app.DestinationID, app.GitSourceID, app.TeamID)
	return err
}

func (s *Store) SetApplicationBuildServerEnabled(ctx context.Context, teamID, appID uuid.UUID, enabled bool) error {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE application_settings SET is_build_server_enabled=$3, updated_at=NOW()
		WHERE application_id=$1 AND EXISTS (SELECT 1 FROM applications WHERE id=$1 AND team_id=$2)
	`, appID, teamID, enabled)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) SetDeploymentBuildServer(ctx context.Context, deploymentID uuid.UUID, buildServerID *uuid.UUID) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE deployments SET build_server_id=$2, updated_at=NOW() WHERE id=$1
	`, deploymentID, buildServerID)
	return err
}

func (s *Store) CreateDeployment(ctx context.Context, teamID, appID uuid.UUID, serverID *uuid.UUID, commitSHA, commitMsg string, forceRebuild, isWebhook, isAPI bool) (*Deployment, error) {
	var d Deployment
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO deployments (team_id, application_id, server_id, status, commit_sha, commit_message, force_rebuild, is_webhook, is_api)
		VALUES ($1,$2,$3,'queued',$4,$5,$6,$7,$8)
		RETURNING id, team_id, application_id, server_id, status, commit_sha, commit_message, current_stage, logs, error_message, started_at, finished_at, created_at
	`, teamID, appID, serverID, commitSHA, commitMsg, forceRebuild, isWebhook, isAPI).Scan(
		&d.ID, &d.TeamID, &d.ApplicationID, &d.ServerID, &d.Status, &d.CommitSHA, &d.CommitMessage,
		&d.CurrentStage, &d.Logs, &d.ErrorMessage, &d.StartedAt, &d.FinishedAt, &d.CreatedAt,
	)
	return &d, err
}

func (s *Store) GetDeployment(ctx context.Context, teamID, id uuid.UUID) (*Deployment, error) {
	var d Deployment
	err := s.Pool.QueryRow(ctx, `
		SELECT id, team_id, application_id, server_id, status, commit_sha, commit_message, current_stage, logs, error_message, started_at, finished_at, created_at
		FROM deployments WHERE id=$1 AND team_id=$2
	`, id, teamID).Scan(
		&d.ID, &d.TeamID, &d.ApplicationID, &d.ServerID, &d.Status, &d.CommitSHA, &d.CommitMessage,
		&d.CurrentStage, &d.Logs, &d.ErrorMessage, &d.StartedAt, &d.FinishedAt, &d.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &d, err
}

func (s *Store) ListDeployments(ctx context.Context, teamID, appID uuid.UUID, limit int) ([]Deployment, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT id, team_id, application_id, server_id, status, commit_sha, commit_message, current_stage, logs, error_message, started_at, finished_at, created_at
		FROM deployments WHERE team_id=$1 AND application_id=$2
		ORDER BY created_at DESC LIMIT $3
	`, teamID, appID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Deployment
	for rows.Next() {
		var d Deployment
		if err := rows.Scan(
			&d.ID, &d.TeamID, &d.ApplicationID, &d.ServerID, &d.Status, &d.CommitSHA, &d.CommitMessage,
			&d.CurrentStage, &d.Logs, &d.ErrorMessage, &d.StartedAt, &d.FinishedAt, &d.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) AppendDeploymentLog(ctx context.Context, id uuid.UUID, stage, line string) error {
	entry, _ := json.Marshal(map[string]any{
		"ts":    time.Now().UTC().Format(time.RFC3339Nano),
		"stage": stage,
		"line":  line,
	})
	_, err := s.Pool.Exec(ctx, `
		UPDATE deployments
		SET logs = logs || $2::jsonb,
		    current_stage = $3,
		    updated_at = NOW()
		WHERE id = $1
	`, id, entry, stage)
	return err
}

func (s *Store) SetDeploymentStatus(ctx context.Context, id uuid.UUID, status, errMsg string) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE deployments SET
			status=$2,
			error_message=$3,
			started_at = CASE WHEN $2='in_progress' AND started_at IS NULL THEN NOW() ELSE started_at END,
			finished_at = CASE WHEN $2 IN ('finished','failed','cancelled') THEN NOW() ELSE finished_at END,
			updated_at=NOW()
		WHERE id=$1
	`, id, status, errMsg)
	return err
}

func DefaultDBImage(engine string) string {
	switch engine {
	case "postgresql":
		return "postgres:16-alpine"
	case "mysql":
		return "mysql:8"
	case "mariadb":
		return "mariadb:11"
	case "mongodb":
		return "mongo:7"
	case "redis":
		return "redis:7-alpine"
	case "keydb":
		return "eqalpha/keydb:latest"
	case "dragonfly":
		return "docker.dragonflydb.io/dragonflydb/dragonfly:latest"
	case "clickhouse":
		return "clickhouse/clickhouse-server:24"
	default:
		return ""
	}
}

func (s *Store) CreateDatabase(ctx context.Context, db *Database, credentialsEnc string) (*Database, error) {
	if db.Image == "" {
		db.Image = DefaultDBImage(db.Engine)
	}
	if len(db.EngineConfig) == 0 {
		db.EngineConfig = json.RawMessage(`{}`)
	}
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO databases (team_id, environment_id, destination_id, name, description, engine, image, is_public, public_port, engine_config, credentials_enc)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING id, team_id, environment_id, destination_id, name, description, engine, image, status, is_public, public_port, engine_config, created_at
	`, db.TeamID, db.EnvironmentID, db.DestinationID, db.Name, db.Description, db.Engine, db.Image, db.IsPublic, db.PublicPort, db.EngineConfig, credentialsEnc).Scan(
		&db.ID, &db.TeamID, &db.EnvironmentID, &db.DestinationID, &db.Name, &db.Description, &db.Engine, &db.Image,
		&db.Status, &db.IsPublic, &db.PublicPort, &db.EngineConfig, &db.CreatedAt,
	)
	return db, err
}

func (s *Store) ListDatabases(ctx context.Context, teamID uuid.UUID, environmentID *uuid.UUID) ([]Database, error) {
	q := `
		SELECT id, team_id, environment_id, destination_id, name, description, engine, image, status, is_public, public_port, engine_config, created_at
		FROM databases WHERE team_id=$1`
	args := []any{teamID}
	if environmentID != nil {
		q += ` AND environment_id=$2`
		args = append(args, *environmentID)
	}
	q += ` ORDER BY name`
	rows, err := s.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Database
	for rows.Next() {
		var d Database
		if err := rows.Scan(
			&d.ID, &d.TeamID, &d.EnvironmentID, &d.DestinationID, &d.Name, &d.Description, &d.Engine, &d.Image,
			&d.Status, &d.IsPublic, &d.PublicPort, &d.EngineConfig, &d.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) GetDatabase(ctx context.Context, teamID, id uuid.UUID) (*Database, error) {
	var d Database
	err := s.Pool.QueryRow(ctx, `
		SELECT id, team_id, environment_id, destination_id, name, description, engine, image, status, is_public, public_port, engine_config, created_at
		FROM databases WHERE id=$1 AND team_id=$2
	`, id, teamID).Scan(
		&d.ID, &d.TeamID, &d.EnvironmentID, &d.DestinationID, &d.Name, &d.Description, &d.Engine, &d.Image,
		&d.Status, &d.IsPublic, &d.PublicPort, &d.EngineConfig, &d.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &d, err
}

func (s *Store) DeleteDatabase(ctx context.Context, teamID, id uuid.UUID) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var exists uuid.UUID
	err = tx.QueryRow(ctx, `SELECT id FROM databases WHERE id=$1 AND team_id=$2`, id, teamID).Scan(&exists)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM environment_variables WHERE team_id=$1 AND resource_type='database' AND resource_id=$2`, teamID, id); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM scheduled_tasks WHERE team_id=$1 AND resource_type='database' AND resource_id=$2`, teamID, id); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM scheduled_backups WHERE team_id=$1 AND resource_type='database' AND resource_id=$2`, teamID, id); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM backup_executions WHERE team_id=$1 AND resource_type='database' AND resource_id=$2`, teamID, id); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM databases WHERE id=$1 AND team_id=$2`, id, teamID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) UpdateDatabaseStatus(ctx context.Context, id uuid.UUID, status string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE databases SET status=$2, updated_at=NOW() WHERE id=$1`, id, status)
	return err
}

func (s *Store) CreateService(ctx context.Context, svc *Service) (*Service, error) {
	prepared := svc.DockerCompose
	if prepared == "" {
		prepared = svc.DockerComposeRaw
	}
	if svc.ID == uuid.Nil {
		svc.ID = uuid.New()
	}
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO services (id, team_id, environment_id, server_id, destination_id, name, description, service_type, docker_compose_raw, docker_compose, fqdn)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING id, team_id, environment_id, server_id, destination_id, name, description, service_type, docker_compose_raw, docker_compose, COALESCE(fqdn,''), status, created_at
	`, svc.ID, svc.TeamID, svc.EnvironmentID, svc.ServerID, svc.DestinationID, svc.Name, svc.Description, svc.ServiceType, svc.DockerComposeRaw, prepared, svc.FQDN).Scan(
		&svc.ID, &svc.TeamID, &svc.EnvironmentID, &svc.ServerID, &svc.DestinationID, &svc.Name, &svc.Description,
		&svc.ServiceType, &svc.DockerComposeRaw, &svc.DockerCompose, &svc.FQDN, &svc.Status, &svc.CreatedAt,
	)
	return svc, err
}

func (s *Store) ListServices(ctx context.Context, teamID uuid.UUID, environmentID *uuid.UUID) ([]Service, error) {
	q := `
		SELECT id, team_id, environment_id, server_id, destination_id, name, description, service_type, docker_compose_raw, COALESCE(docker_compose, ''), COALESCE(fqdn,''), status, created_at
		FROM services WHERE team_id=$1 AND deleted_at IS NULL`
	args := []any{teamID}
	if environmentID != nil {
		q += ` AND environment_id=$2`
		args = append(args, *environmentID)
	}
	q += ` ORDER BY name`
	rows, err := s.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Service
	for rows.Next() {
		var svc Service
		if err := rows.Scan(
			&svc.ID, &svc.TeamID, &svc.EnvironmentID, &svc.ServerID, &svc.DestinationID, &svc.Name, &svc.Description,
			&svc.ServiceType, &svc.DockerComposeRaw, &svc.DockerCompose, &svc.FQDN, &svc.Status, &svc.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, svc)
	}
	return out, rows.Err()
}

func (s *Store) GetService(ctx context.Context, teamID, id uuid.UUID) (*Service, error) {
	var svc Service
	err := s.Pool.QueryRow(ctx, `
		SELECT id, team_id, environment_id, server_id, destination_id, name, description, service_type, docker_compose_raw, COALESCE(docker_compose, ''), COALESCE(fqdn,''), status, created_at
		FROM services WHERE id=$1 AND team_id=$2 AND deleted_at IS NULL
	`, id, teamID).Scan(
		&svc.ID, &svc.TeamID, &svc.EnvironmentID, &svc.ServerID, &svc.DestinationID, &svc.Name, &svc.Description,
		&svc.ServiceType, &svc.DockerComposeRaw, &svc.DockerCompose, &svc.FQDN, &svc.Status, &svc.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &svc, err
}

func (s *Store) UpdateServiceCompose(ctx context.Context, id uuid.UUID, prepared string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE services SET docker_compose=$2, updated_at=NOW() WHERE id=$1`, id, prepared)
	return err
}

func (s *Store) UpdateServiceMeta(ctx context.Context, teamID, id uuid.UUID, name, description string) (*Service, error) {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE services SET name=$3, description=$4, updated_at=NOW()
		WHERE id=$1 AND team_id=$2 AND deleted_at IS NULL
	`, id, teamID, name, description)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrNotFound
	}
	return s.GetService(ctx, teamID, id)
}

func (s *Store) UpdateServiceFQDN(ctx context.Context, id uuid.UUID, fqdn string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE services SET fqdn=$2, updated_at=NOW() WHERE id=$1`, id, fqdn)
	return err
}

func (s *Store) ListQueuedDeployments(ctx context.Context, limit int) ([]Deployment, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT id, team_id, application_id, server_id, status, commit_sha, commit_message, current_stage, logs, error_message, started_at, finished_at, created_at
		FROM deployments
		WHERE status IN ('queued', 'in_progress')
		ORDER BY created_at ASC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Deployment
	for rows.Next() {
		var d Deployment
		if err := rows.Scan(
			&d.ID, &d.TeamID, &d.ApplicationID, &d.ServerID, &d.Status, &d.CommitSHA, &d.CommitMessage,
			&d.CurrentStage, &d.Logs, &d.ErrorMessage, &d.StartedAt, &d.FinishedAt, &d.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) UpdateServiceStatus(ctx context.Context, id uuid.UUID, status string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE services SET status=$2, updated_at=NOW() WHERE id=$1`, id, status)
	return err
}

func (s *Store) GetApplicationByID(ctx context.Context, id uuid.UUID) (*Application, error) {
	row := s.Pool.QueryRow(ctx, `SELECT `+applicationSelectCols+`
		FROM applications a
		LEFT JOIN application_settings s ON s.application_id = a.id
		WHERE a.id=$1`, id)
	a, err := scanApplication(row.Scan)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return a, err
}

func (s *Store) GetDatabaseCredentials(ctx context.Context, teamID, id uuid.UUID) (string, error) {
	var enc string
	err := s.Pool.QueryRow(ctx, `SELECT credentials_enc FROM databases WHERE id=$1 AND team_id=$2`, id, teamID).Scan(&enc)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return enc, nil
}

func (s *Store) CreatePreview(ctx context.Context, teamID, appID uuid.UUID, prID int, title, branch, fqdn string) (*ApplicationPreview, error) {
	var p ApplicationPreview
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO application_previews (team_id, application_id, pull_request_id, pull_request_title, git_branch, fqdn, status)
		VALUES ($1,$2,$3,$4,$5,$6,'queued')
		ON CONFLICT (application_id, pull_request_id) DO UPDATE
		SET pull_request_title=EXCLUDED.pull_request_title, git_branch=EXCLUDED.git_branch,
		    fqdn=EXCLUDED.fqdn, status='queued', updated_at=NOW()
		RETURNING id, team_id, application_id, pull_request_id, pull_request_title, git_branch, fqdn, status, created_at
	`, teamID, appID, prID, title, branch, fqdn).Scan(
		&p.ID, &p.TeamID, &p.ApplicationID, &p.PullRequestID, &p.PullRequestTitle, &p.GitBranch, &p.FQDN, &p.Status, &p.CreatedAt,
	)
	return &p, err
}

func (s *Store) ListPreviews(ctx context.Context, teamID, appID uuid.UUID) ([]ApplicationPreview, error) {
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

func (s *Store) DeletePreview(ctx context.Context, teamID, appID uuid.UUID, prID int) error {
	tag, err := s.Pool.Exec(ctx, `
		DELETE FROM application_previews WHERE team_id=$1 AND application_id=$2 AND pull_request_id=$3
	`, teamID, appID, prID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

type NotificationSetting struct {
	ID        uuid.UUID `json:"id"`
	TeamID    uuid.UUID `json:"team_id"`
	Channel   string    `json:"channel"`
	Enabled   bool      `json:"enabled"`
	ConfigEnc string    `json:"-"`
	Events    []string  `json:"events"`
}

func (s *Store) ListEnabledNotifications(ctx context.Context, teamID uuid.UUID) ([]NotificationSetting, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, team_id, channel, enabled, config_enc, events
		FROM notification_settings WHERE team_id=$1 AND enabled=TRUE
	`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []NotificationSetting
	for rows.Next() {
		var n NotificationSetting
		if err := rows.Scan(&n.ID, &n.TeamID, &n.Channel, &n.Enabled, &n.ConfigEnc, &n.Events); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

type S3Storage struct {
	ID        uuid.UUID `json:"id"`
	TeamID    uuid.UUID `json:"team_id"`
	Name      string    `json:"name"`
	Endpoint  string    `json:"endpoint"`
	Bucket    string    `json:"bucket"`
	Region    string    `json:"region"`
	PathStyle bool      `json:"path_style"`
	CreatedAt time.Time `json:"created_at"`
}

type ScheduledBackup struct {
	ID          uuid.UUID  `json:"id"`
	TeamID      uuid.UUID  `json:"team_id"`
	ResourceType string    `json:"resource_type"`
	ResourceID  uuid.UUID  `json:"resource_id"`
	S3StorageID *uuid.UUID `json:"s3_storage_id,omitempty"`
	Frequency   string     `json:"frequency"`
	Enabled     bool       `json:"enabled"`
	Retention   int        `json:"retention"`
	CreatedAt   time.Time  `json:"created_at"`
}

func (s *Store) CreateS3Storage(ctx context.Context, teamID uuid.UUID, name, endpoint, bucket, region, accessKeyEnc, secretKeyEnc string, pathStyle bool) (*S3Storage, error) {
	if region == "" {
		region = "us-east-1"
	}
	var st S3Storage
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO s3_storages (team_id, name, endpoint, bucket, region, access_key_enc, secret_key_enc, path_style)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING id, team_id, name, endpoint, bucket, region, path_style, created_at
	`, teamID, name, endpoint, bucket, region, accessKeyEnc, secretKeyEnc, pathStyle).Scan(
		&st.ID, &st.TeamID, &st.Name, &st.Endpoint, &st.Bucket, &st.Region, &st.PathStyle, &st.CreatedAt,
	)
	return &st, err
}

func (s *Store) ListS3Storages(ctx context.Context, teamID uuid.UUID) ([]S3Storage, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, team_id, name, endpoint, bucket, region, path_style, created_at
		FROM s3_storages WHERE team_id=$1 ORDER BY name
	`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []S3Storage
	for rows.Next() {
		var st S3Storage
		if err := rows.Scan(&st.ID, &st.TeamID, &st.Name, &st.Endpoint, &st.Bucket, &st.Region, &st.PathStyle, &st.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

func (s *Store) GetS3Storage(ctx context.Context, teamID, id uuid.UUID) (*S3Storage, error) {
	var st S3Storage
	err := s.Pool.QueryRow(ctx, `
		SELECT id, team_id, name, endpoint, bucket, region, path_style, created_at
		FROM s3_storages WHERE id=$1 AND team_id=$2
	`, id, teamID).Scan(&st.ID, &st.TeamID, &st.Name, &st.Endpoint, &st.Bucket, &st.Region, &st.PathStyle, &st.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &st, err
}

func (s *Store) GetS3StorageSecrets(ctx context.Context, teamID, id uuid.UUID) (accessKeyEnc, secretKeyEnc string, err error) {
	err = s.Pool.QueryRow(ctx, `
		SELECT access_key_enc, secret_key_enc FROM s3_storages WHERE id=$1 AND team_id=$2
	`, id, teamID).Scan(&accessKeyEnc, &secretKeyEnc)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", ErrNotFound
	}
	return accessKeyEnc, secretKeyEnc, err
}

func (s *Store) DeleteS3Storage(ctx context.Context, teamID, id uuid.UUID) error {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM s3_storages WHERE id=$1 AND team_id=$2`, id, teamID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) CreateScheduledBackup(ctx context.Context, teamID uuid.UUID, resourceType string, resourceID uuid.UUID, s3ID *uuid.UUID, frequency string, retention int) (*ScheduledBackup, error) {
	if frequency == "" {
		frequency = "0 0 * * *"
	}
	if retention <= 0 {
		retention = 7
	}
	var b ScheduledBackup
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO scheduled_backups (team_id, resource_type, resource_id, s3_storage_id, frequency, retention)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING id, team_id, resource_type, resource_id, s3_storage_id, frequency, enabled, retention, created_at
	`, teamID, resourceType, resourceID, s3ID, frequency, retention).Scan(
		&b.ID, &b.TeamID, &b.ResourceType, &b.ResourceID, &b.S3StorageID, &b.Frequency, &b.Enabled, &b.Retention, &b.CreatedAt,
	)
	return &b, err
}

func (s *Store) GetScheduledBackup(ctx context.Context, teamID, id uuid.UUID) (*ScheduledBackup, error) {
	var b ScheduledBackup
	err := s.Pool.QueryRow(ctx, `
		SELECT id, team_id, resource_type, resource_id, s3_storage_id, frequency, enabled, retention, created_at
		FROM scheduled_backups WHERE id=$1 AND team_id=$2
	`, id, teamID).Scan(&b.ID, &b.TeamID, &b.ResourceType, &b.ResourceID, &b.S3StorageID, &b.Frequency, &b.Enabled, &b.Retention, &b.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &b, err
}

func (s *Store) ListScheduledBackups(ctx context.Context, teamID uuid.UUID) ([]ScheduledBackup, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, team_id, resource_type, resource_id, s3_storage_id, frequency, enabled, retention, created_at
		FROM scheduled_backups WHERE team_id=$1 ORDER BY created_at DESC
	`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ScheduledBackup
	for rows.Next() {
		var b ScheduledBackup
		if err := rows.Scan(&b.ID, &b.TeamID, &b.ResourceType, &b.ResourceID, &b.S3StorageID, &b.Frequency, &b.Enabled, &b.Retention, &b.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

type BackupExecution struct {
	ID                uuid.UUID  `json:"id"`
	TeamID            uuid.UUID  `json:"team_id"`
	ScheduledBackupID *uuid.UUID `json:"scheduled_backup_id,omitempty"`
	ResourceType      string     `json:"resource_type"`
	ResourceID        uuid.UUID  `json:"resource_id"`
	Status            string     `json:"status"`
	SizeBytes         int64      `json:"size_bytes"`
	Filename          string     `json:"filename"`
	S3Uploaded        bool       `json:"s3_uploaded"`
	S3Key             string     `json:"s3_key,omitempty"`
	ErrorMessage      string     `json:"error_message"`
	StartedAt         time.Time  `json:"started_at"`
	FinishedAt        *time.Time `json:"finished_at,omitempty"`
}

func (s *Store) CreateBackupExecution(ctx context.Context, teamID uuid.UUID, resourceType string, resourceID uuid.UUID, filename string) (*BackupExecution, error) {
	var b BackupExecution
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO backup_executions (team_id, resource_type, resource_id, status, filename)
		VALUES ($1,$2,$3,'running',$4)
		RETURNING id, team_id, scheduled_backup_id, resource_type, resource_id, status, size_bytes, filename, s3_uploaded, s3_key, error_message, started_at, finished_at
	`, teamID, resourceType, resourceID, filename).Scan(
		&b.ID, &b.TeamID, &b.ScheduledBackupID, &b.ResourceType, &b.ResourceID, &b.Status, &b.SizeBytes, &b.Filename, &b.S3Uploaded, &b.S3Key, &b.ErrorMessage, &b.StartedAt, &b.FinishedAt,
	)
	return &b, err
}

func (s *Store) MarkBackupS3Uploaded(ctx context.Context, execID uuid.UUID, key string) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE backup_executions SET s3_uploaded=TRUE, s3_key=$2 WHERE id=$1
	`, execID, key)
	return err
}

func (s *Store) FinishBackupExecution(ctx context.Context, id uuid.UUID, status string, sizeBytes int64, errMsg string) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE backup_executions
		SET status=$2, size_bytes=$3, error_message=$4, finished_at=NOW()
		WHERE id=$1
	`, id, status, sizeBytes, errMsg)
	return err
}

func (s *Store) ListBackupExecutions(ctx context.Context, teamID uuid.UUID, resourceType string, resourceID uuid.UUID) ([]BackupExecution, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, team_id, scheduled_backup_id, resource_type, resource_id, status, size_bytes, filename, s3_uploaded, s3_key, error_message, started_at, finished_at
		FROM backup_executions
		WHERE team_id=$1 AND resource_type=$2 AND resource_id=$3
		ORDER BY started_at DESC
		LIMIT 50
	`, teamID, resourceType, resourceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BackupExecution
	for rows.Next() {
		var b BackupExecution
		if err := rows.Scan(&b.ID, &b.TeamID, &b.ScheduledBackupID, &b.ResourceType, &b.ResourceID, &b.Status, &b.SizeBytes, &b.Filename, &b.S3Uploaded, &b.S3Key, &b.ErrorMessage, &b.StartedAt, &b.FinishedAt); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (s *Store) GetBackupExecution(ctx context.Context, teamID, id uuid.UUID) (*BackupExecution, error) {
	var b BackupExecution
	err := s.Pool.QueryRow(ctx, `
		SELECT id, team_id, scheduled_backup_id, resource_type, resource_id, status, size_bytes, filename, s3_uploaded, s3_key, error_message, started_at, finished_at
		FROM backup_executions WHERE id=$1 AND team_id=$2
	`, id, teamID).Scan(
		&b.ID, &b.TeamID, &b.ScheduledBackupID, &b.ResourceType, &b.ResourceID, &b.Status, &b.SizeBytes, &b.Filename, &b.S3Uploaded, &b.S3Key, &b.ErrorMessage, &b.StartedAt, &b.FinishedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &b, err
}

type ServerMetric struct {
	CPUPercent       float64   `json:"cpu_percent"`
	MemoryUsedBytes  int64     `json:"memory_used_bytes"`
	MemoryTotalBytes int64     `json:"memory_total_bytes"`
	DiskUsedBytes    int64     `json:"disk_used_bytes"`
	DiskTotalBytes   int64     `json:"disk_total_bytes"`
	RecordedAt       time.Time `json:"recorded_at"`
}

func (s *Store) ListServerMetrics(ctx context.Context, teamID, serverID uuid.UUID, limit int) ([]ServerMetric, error) {
	if limit <= 0 || limit > 500 {
		limit = 60
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT cpu_percent, memory_used_bytes, memory_total_bytes, disk_used_bytes, disk_total_bytes, recorded_at
		FROM server_metrics
		WHERE team_id=$1 AND server_id=$2
		ORDER BY recorded_at DESC
		LIMIT $3
	`, teamID, serverID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ServerMetric
	for rows.Next() {
		var m ServerMetric
		if err := rows.Scan(&m.CPUPercent, &m.MemoryUsedBytes, &m.MemoryTotalBytes, &m.DiskUsedBytes, &m.DiskTotalBytes, &m.RecordedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, rows.Err()
}

type ScheduledTaskRow struct {
	ID           uuid.UUID
	TeamID       uuid.UUID
	ResourceType string
	ResourceID   uuid.UUID
	Name         string
	Command      string
	Frequency    string
	Container    string
	Enabled      bool
}

func (s *Store) ListEnabledScheduledTasks(ctx context.Context) ([]ScheduledTaskRow, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, team_id, resource_type, resource_id, name, command, frequency, container_name, enabled
		FROM scheduled_tasks WHERE enabled=TRUE
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ScheduledTaskRow
	for rows.Next() {
		var t ScheduledTaskRow
		if err := rows.Scan(&t.ID, &t.TeamID, &t.ResourceType, &t.ResourceID, &t.Name, &t.Command, &t.Frequency, &t.Container, &t.Enabled); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) TaskRanThisMinute(ctx context.Context, taskID uuid.UUID, minute time.Time) (bool, error) {
	var n int
	err := s.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM scheduled_task_executions
		WHERE scheduled_task_id=$1 AND started_at >= $2 AND started_at < $3
	`, taskID, minute, minute.Add(time.Minute)).Scan(&n)
	return n > 0, err
}

func (s *Store) CreateTaskExecution(ctx context.Context, teamID, taskID uuid.UUID) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO scheduled_task_executions (team_id, scheduled_task_id, status)
		VALUES ($1,$2,'running') RETURNING id
	`, teamID, taskID).Scan(&id)
	return id, err
}

func (s *Store) FinishTaskExecution(ctx context.Context, id uuid.UUID, status, output string) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE scheduled_task_executions
		SET status=$2, output=$3, finished_at=NOW()
		WHERE id=$1
	`, id, status, output)
	return err
}

type ScheduledBackupRow struct {
	ID           uuid.UUID
	TeamID       uuid.UUID
	ResourceType string
	ResourceID   uuid.UUID
	S3StorageID  *uuid.UUID
	Frequency    string
	Enabled      bool
	Retention    int
}

func (s *Store) ListEnabledScheduledBackups(ctx context.Context) ([]ScheduledBackupRow, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, team_id, resource_type, resource_id, s3_storage_id, frequency, enabled, retention
		FROM scheduled_backups WHERE enabled=TRUE
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ScheduledBackupRow
	for rows.Next() {
		var b ScheduledBackupRow
		if err := rows.Scan(&b.ID, &b.TeamID, &b.ResourceType, &b.ResourceID, &b.S3StorageID, &b.Frequency, &b.Enabled, &b.Retention); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (s *Store) BackupRanThisMinute(ctx context.Context, backupID uuid.UUID, minute time.Time) (bool, error) {
	var n int
	err := s.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM backup_executions
		WHERE scheduled_backup_id=$1 AND started_at >= $2 AND started_at < $3
	`, backupID, minute, minute.Add(time.Minute)).Scan(&n)
	return n > 0, err
}

func (s *Store) CreateBackupExecutionScheduled(ctx context.Context, teamID uuid.UUID, scheduledID *uuid.UUID, resourceType string, resourceID uuid.UUID, filename string) (*BackupExecution, error) {
	var b BackupExecution
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO backup_executions (team_id, scheduled_backup_id, resource_type, resource_id, status, filename)
		VALUES ($1,$2,$3,$4,'running',$5)
		RETURNING id, team_id, scheduled_backup_id, resource_type, resource_id, status, size_bytes, filename, s3_uploaded, s3_key, error_message, started_at, finished_at
	`, teamID, scheduledID, resourceType, resourceID, filename).Scan(
		&b.ID, &b.TeamID, &b.ScheduledBackupID, &b.ResourceType, &b.ResourceID, &b.Status, &b.SizeBytes, &b.Filename, &b.S3Uploaded, &b.S3Key, &b.ErrorMessage, &b.StartedAt, &b.FinishedAt,
	)
	return &b, err
}
