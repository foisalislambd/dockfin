package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/dockfin/dockfin/internal/git"
	"github.com/dockfin/dockfin/internal/redact"
)

type Project struct {
	ID          uuid.UUID `json:"id"`
	TeamID      uuid.UUID `json:"team_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	IsEmpty     bool      `json:"is_empty"`
}

type Environment struct {
	ID          uuid.UUID `json:"id"`
	TeamID      uuid.UUID `json:"team_id"`
	ProjectID   uuid.UUID `json:"project_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	IsEmpty     bool      `json:"is_empty"`
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
	// Dockerfile is inline content for Coolify-style "Dockerfile without Git".
	Dockerfile              string     `json:"dockerfile,omitempty"`
	DockerComposeLocation   string     `json:"docker_compose_location"`
	DockerRegistryImageName string     `json:"docker_registry_image_name"`
	DockerRegistryImageTag  string     `json:"docker_registry_image_tag"`
	PortsExposes            string     `json:"ports_exposes"`
	PortsMappings           string     `json:"ports_mappings,omitempty"`
	CustomNetworkAliases    string     `json:"custom_network_aliases,omitempty"`
	InstallCommand          string     `json:"install_command,omitempty"`
	BuildCommand            string     `json:"build_command,omitempty"`
	StartCommand            string     `json:"start_command,omitempty"`
	PublishDirectory        string     `json:"publish_directory,omitempty"`
	CustomNginxConfiguration string    `json:"custom_nginx_configuration,omitempty"`
	PreviewURLTemplate      string     `json:"preview_url_template,omitempty"`
	// ComposePrepare adapts Git compose for Dockfin (Traefik, network, strip host ports).
	// False = deploy the repository compose file unchanged.
	ComposePrepare bool `json:"compose_prepare"`
	// Coolify-parity compose preview + build options.
	DockerComposeRaw                  string          `json:"docker_compose_raw,omitempty"`
	DockerCompose                     string          `json:"docker_compose,omitempty"`
	DockerComposeDomains              json.RawMessage `json:"docker_compose_domains,omitempty"`
	BaseDirectory                     string          `json:"base_directory"`
	DockerComposeCustomBuildCommand   string          `json:"docker_compose_custom_build_command,omitempty"`
	DockerComposeCustomStartCommand   string          `json:"docker_compose_custom_start_command,omitempty"`
	CustomDockerRunOptions            string          `json:"custom_docker_run_options,omitempty"`
	DockerfileTargetBuild             string          `json:"dockerfile_target_build,omitempty"`
	HealthCheckEnabled                bool            `json:"health_check_enabled"`
	HealthCheckPath                   string          `json:"health_check_path"`
	HealthCheckPort                   *int            `json:"health_check_port,omitempty"`
	HealthCheckMethod                 string          `json:"health_check_method"`
	HealthCheckReturnCode             int             `json:"health_check_return_code"`
	HealthCheckInterval               int             `json:"health_check_interval"`
	HealthCheckTimeout                int             `json:"health_check_timeout"`
	HealthCheckRetries                int             `json:"health_check_retries"`
	HealthCheckHost                   string          `json:"health_check_host"`
	HealthCheckScheme                 string          `json:"health_check_scheme"`
	HealthCheckResponseText           string          `json:"health_check_response_text,omitempty"`
	HealthCheckStartPeriod            int             `json:"health_check_start_period"`
	HealthCheckType                   string          `json:"health_check_type"`
	HealthCheckCommand                string          `json:"health_check_command,omitempty"`
	LimitsMemory                      string          `json:"limits_memory"`
	LimitsCpus                        string          `json:"limits_cpus"`
	PreDeploymentCommand              string          `json:"pre_deployment_command,omitempty"`
	PostDeploymentCommand             string          `json:"post_deployment_command,omitempty"`
	CustomLabels                      string          `json:"custom_labels,omitempty"`
	HTTPBasicAuthUsername             string          `json:"http_basic_auth_username,omitempty"`
	HasHTTPBasicAuth                  bool            `json:"has_http_basic_auth"`
	HTTPBasicAuthPasswordEnc          string          `json:"-"`
	IsForceHTTPS                      bool            `json:"is_force_https"`
	IsPreviewEnabled                  bool            `json:"is_preview_enabled"`
	GitSourceID                       *uuid.UUID      `json:"git_source_id,omitempty"`
	PrivateKeyID                      *uuid.UUID      `json:"private_key_id,omitempty"`
	IsBuildServerEnabled              bool            `json:"is_build_server_enabled"`
	IsAutoDeployEnabled               bool            `json:"is_auto_deploy_enabled"`
	IsGitSubmodulesEnabled            bool            `json:"is_git_submodules_enabled"`
	IsPreserveRepositoryEnabled       bool            `json:"is_preserve_repository_enabled"`
	WatchPaths                        string          `json:"watch_paths,omitempty"`
	Redirect                          string          `json:"redirect"`
	DockerRegistryID                  *uuid.UUID      `json:"docker_registry_id,omitempty"`
	IsDisableBuildCache               bool            `json:"is_disable_build_cache"`
	IsGitShallowCloneEnabled          bool            `json:"is_git_shallow_clone_enabled"`
	IsGitLFSEnabled                   bool            `json:"is_git_lfs_enabled"`
	IsGPUEnabled                      bool            `json:"is_gpu_enabled"`
	GPUCount                          int             `json:"gpu_count"`
	CustomDockerStopTimeout           int             `json:"custom_docker_stop_timeout"`
	CustomDockerRestartPolicy         string          `json:"custom_docker_restart_policy"`
	IsSPA                             bool            `json:"is_spa"`
	InjectBuildArgsToDockerfile       bool            `json:"inject_build_args_to_dockerfile"`
	UseBuildSecrets                   bool            `json:"use_build_secrets"`
	IncludeSourceCommitInBuild        bool            `json:"include_source_commit_in_build"`
	DockerImagesToKeep                int             `json:"docker_images_to_keep"`
	IsConsistentContainerNameEnabled  bool            `json:"is_consistent_container_name_enabled"`
	CustomInternalName                string          `json:"custom_internal_name,omitempty"`
	IsGzipEnabled                     bool            `json:"is_gzip_enabled"`
	IsStripPrefixEnabled              bool            `json:"is_stripprefix_enabled"`
	IsLogDrainEnabled                 bool            `json:"is_log_drain_enabled"`
	IsDebugEnabled                    bool            `json:"is_debug_enabled"`
	IsEnvSortingEnabled               bool            `json:"is_env_sorting_enabled"`
	IsPRDeploymentsPublicEnabled      bool            `json:"is_pr_deployments_public_enabled"`
	SkipRebuildIfUnchanged            bool            `json:"skip_rebuild_if_unchanged"`
	GPUDriver                         string          `json:"gpu_driver,omitempty"`
	GPUDeviceIDs                      string          `json:"gpu_device_ids,omitempty"`
	GPUOptions                        string          `json:"gpu_options,omitempty"`
	CustomDockerMaxRestartCount       int             `json:"custom_docker_max_restart_count"`
	PreDeploymentCommandContainer     string          `json:"pre_deployment_command_container,omitempty"`
	PostDeploymentCommandContainer    string          `json:"post_deployment_command_container,omitempty"`
	IsSwarmOnlyWorkerNodes            bool            `json:"is_swarm_only_worker_nodes"`
	IsIncludeTimestamps               bool            `json:"is_include_timestamps"`
	LogsLineLimit                     int             `json:"logs_line_limit"`
	SwarmReplicas                     int             `json:"swarm_replicas"`
	SwarmPlacementConstraints         string          `json:"swarm_placement_constraints,omitempty"`
	CreatedAt                         time.Time       `json:"created_at"`
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
	a.git_repository, a.git_branch, a.git_commit_sha, a.dockerfile_location, COALESCE(a.dockerfile, ''),
	a.docker_compose_location,
	a.docker_registry_image_name, a.docker_registry_image_tag, a.ports_exposes,
	COALESCE(a.ports_mappings, ''), COALESCE(a.custom_network_aliases, ''),
	COALESCE(a.install_command, ''), COALESCE(a.build_command, ''), COALESCE(a.start_command, ''),
	COALESCE(a.publish_directory, ''), COALESCE(a.custom_nginx_configuration, ''),
	COALESCE(NULLIF(a.preview_url_template, ''), '{{pr_id}}.{{domain}}'),
	COALESCE(a.compose_prepare, TRUE),
	COALESCE(a.docker_compose_raw, ''), COALESCE(a.docker_compose, ''),
	COALESCE(a.docker_compose_domains, '{}'::jsonb),
	COALESCE(NULLIF(a.base_directory, ''), '/'),
	COALESCE(a.docker_compose_custom_build_command, ''),
	COALESCE(a.docker_compose_custom_start_command, ''),
	COALESCE(a.custom_docker_run_options, ''),
	COALESCE(a.dockerfile_target_build, ''),
	a.health_check_enabled, a.health_check_path, a.health_check_port, a.health_check_method,
	a.health_check_return_code, a.health_check_interval, a.health_check_timeout, a.health_check_retries,
	COALESCE(a.health_check_host, 'localhost'), COALESCE(a.health_check_scheme, 'http'),
	COALESCE(a.health_check_response_text, ''), COALESCE(a.health_check_start_period, 5),
	COALESCE(NULLIF(a.health_check_type, ''), 'http'), COALESCE(a.health_check_command, ''),
	a.limits_memory, a.limits_cpus,
	COALESCE(a.pre_deployment_command, ''), COALESCE(a.post_deployment_command, ''),
	COALESCE(a.custom_labels, ''), COALESCE(a.http_basic_auth_username, ''), COALESCE(a.http_basic_auth_password_enc, ''),
	a.git_source_id, a.private_key_id,
	COALESCE(s.is_build_server_enabled, FALSE),
	COALESCE(a.is_force_https, s.is_force_https_enabled, TRUE),
	COALESCE(s.is_preview_enabled, FALSE),
	COALESCE(s.is_auto_deploy_enabled, TRUE),
	COALESCE(s.is_git_submodules_enabled, FALSE),
	COALESCE(s.is_preserve_repository_enabled, FALSE),
	COALESCE(s.watch_paths, ''),
	COALESCE(NULLIF(a.redirect, ''), 'both'),
	a.docker_registry_id,
	COALESCE(s.is_disable_build_cache, FALSE),
	COALESCE(s.is_git_shallow_clone_enabled, TRUE),
	COALESCE(s.is_git_lfs_enabled, FALSE),
	COALESCE(s.is_gpu_enabled, FALSE),
	COALESCE(s.gpu_count, 0),
	COALESCE(s.custom_docker_stop_timeout, 0),
	COALESCE(NULLIF(s.custom_docker_restart_policy, ''), 'unless-stopped'),
	COALESCE(s.is_spa, FALSE),
	COALESCE(s.inject_build_args_to_dockerfile, TRUE),
	COALESCE(s.use_build_secrets, FALSE),
	COALESCE(s.include_source_commit_in_build, FALSE),
	COALESCE(s.docker_images_to_keep, 5),
	COALESCE(s.is_consistent_container_name_enabled, FALSE),
	COALESCE(s.custom_internal_name, ''),
	COALESCE(s.is_gzip_enabled, TRUE),
	COALESCE(s.is_stripprefix_enabled, TRUE),
	COALESCE(s.is_log_drain_enabled, FALSE),
	COALESCE(s.is_debug_enabled, FALSE),
	COALESCE(s.is_env_sorting_enabled, TRUE),
	COALESCE(s.is_pr_deployments_public_enabled, FALSE),
	COALESCE(s.skip_rebuild_if_unchanged, TRUE),
	COALESCE(NULLIF(s.gpu_driver, ''), 'nvidia'),
	COALESCE(s.gpu_device_ids, ''),
	COALESCE(s.gpu_options, ''),
	COALESCE(s.custom_docker_max_restart_count, 0),
	COALESCE(s.pre_deployment_command_container, ''),
	COALESCE(s.post_deployment_command_container, ''),
	COALESCE(s.is_swarm_only_worker_nodes, FALSE),
	COALESCE(s.is_include_timestamps, FALSE),
	COALESCE(s.logs_line_limit, 1000),
	COALESCE(a.swarm_replicas, 1),
	COALESCE(a.swarm_placement_constraints, ''),
	a.created_at`

func scanApplication(scan func(dest ...any) error) (*Application, error) {
	var a Application
	err := scan(
		&a.ID, &a.TeamID, &a.EnvironmentID, &a.DestinationID, &a.Name, &a.Description, &a.FQDN, &a.Status, &a.BuildPack,
		&a.GitRepository, &a.GitBranch, &a.GitCommitSHA, &a.DockerfileLocation, &a.Dockerfile, &a.DockerComposeLocation,
		&a.DockerRegistryImageName, &a.DockerRegistryImageTag, &a.PortsExposes,
		&a.PortsMappings, &a.CustomNetworkAliases,
		&a.InstallCommand, &a.BuildCommand, &a.StartCommand,
		&a.PublishDirectory, &a.CustomNginxConfiguration, &a.PreviewURLTemplate,
		&a.ComposePrepare,
		&a.DockerComposeRaw, &a.DockerCompose, &a.DockerComposeDomains,
		&a.BaseDirectory,
		&a.DockerComposeCustomBuildCommand, &a.DockerComposeCustomStartCommand,
		&a.CustomDockerRunOptions, &a.DockerfileTargetBuild,
		&a.HealthCheckEnabled, &a.HealthCheckPath, &a.HealthCheckPort, &a.HealthCheckMethod,
		&a.HealthCheckReturnCode, &a.HealthCheckInterval, &a.HealthCheckTimeout, &a.HealthCheckRetries,
		&a.HealthCheckHost, &a.HealthCheckScheme, &a.HealthCheckResponseText, &a.HealthCheckStartPeriod,
		&a.HealthCheckType, &a.HealthCheckCommand,
		&a.LimitsMemory, &a.LimitsCpus,
		&a.PreDeploymentCommand, &a.PostDeploymentCommand, &a.CustomLabels, &a.HTTPBasicAuthUsername, &a.HTTPBasicAuthPasswordEnc,
		&a.GitSourceID, &a.PrivateKeyID, &a.IsBuildServerEnabled, &a.IsForceHTTPS, &a.IsPreviewEnabled,
		&a.IsAutoDeployEnabled, &a.IsGitSubmodulesEnabled, &a.IsPreserveRepositoryEnabled, &a.WatchPaths,
		&a.Redirect, &a.DockerRegistryID,
		&a.IsDisableBuildCache, &a.IsGitShallowCloneEnabled, &a.IsGitLFSEnabled,
		&a.IsGPUEnabled, &a.GPUCount, &a.CustomDockerStopTimeout, &a.CustomDockerRestartPolicy,
		&a.IsSPA, &a.InjectBuildArgsToDockerfile, &a.UseBuildSecrets, &a.IncludeSourceCommitInBuild,
		&a.DockerImagesToKeep, &a.IsConsistentContainerNameEnabled, &a.CustomInternalName,
		&a.IsGzipEnabled, &a.IsStripPrefixEnabled, &a.IsLogDrainEnabled, &a.IsDebugEnabled,
		&a.IsEnvSortingEnabled, &a.IsPRDeploymentsPublicEnabled, &a.SkipRebuildIfUnchanged,
		&a.GPUDriver, &a.GPUDeviceIDs, &a.GPUOptions, &a.CustomDockerMaxRestartCount,
		&a.PreDeploymentCommandContainer, &a.PostDeploymentCommandContainer,
		&a.IsSwarmOnlyWorkerNodes, &a.IsIncludeTimestamps, &a.LogsLineLimit,
		&a.SwarmReplicas, &a.SwarmPlacementConstraints,
		&a.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if len(a.DockerComposeDomains) == 0 {
		a.DockerComposeDomains = json.RawMessage(`{}`)
	}
	if a.Redirect == "" {
		a.Redirect = "both"
	}
	if a.CustomDockerRestartPolicy == "" {
		a.CustomDockerRestartPolicy = "unless-stopped"
	}
	if a.PreviewURLTemplate == "" {
		a.PreviewURLTemplate = "{{pr_id}}.{{domain}}"
	}
	if a.HealthCheckType == "" {
		a.HealthCheckType = "http"
	}
	if a.HealthCheckHost == "" {
		a.HealthCheckHost = "localhost"
	}
	if a.HealthCheckScheme == "" {
		a.HealthCheckScheme = "http"
	}
	if a.GPUDriver == "" {
		a.GPUDriver = "nvidia"
	}
	if a.DockerImagesToKeep <= 0 {
		a.DockerImagesToKeep = 5
	}
	if a.LogsLineLimit <= 0 {
		a.LogsLineLimit = 1000
	}
	if a.SwarmReplicas <= 0 {
		a.SwarmReplicas = 1
	}
	a.HasHTTPBasicAuth = strings.TrimSpace(a.HTTPBasicAuthPasswordEnc) != "" && strings.TrimSpace(a.HTTPBasicAuthUsername) != ""
	return &a, nil
}

type Deployment struct {
	ID            uuid.UUID       `json:"id"`
	TeamID        uuid.UUID       `json:"team_id"`
	ApplicationID *uuid.UUID      `json:"application_id,omitempty"`
	ServiceID     *uuid.UUID      `json:"service_id,omitempty"`
	ServerID      *uuid.UUID      `json:"server_id"`
	Status        string          `json:"status"`
	CommitSHA     string          `json:"commit_sha"`
	CommitMessage string          `json:"commit_message"`
	PullRequestID int             `json:"pull_request_id"`
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
		if strings.Contains(err.Error(), "projects_team_id_name") || strings.Contains(err.Error(), "duplicate") {
			return nil, nil, ErrConflict
		}
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
	p.IsEmpty = true
	e.IsEmpty = true
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
		empty, err := s.ProjectIsEmpty(ctx, teamID, p.ID)
		if err != nil {
			return nil, err
		}
		p.IsEmpty = empty
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
	if err != nil {
		return nil, err
	}
	empty, err := s.ProjectIsEmpty(ctx, teamID, id)
	if err != nil {
		return nil, err
	}
	p.IsEmpty = empty
	return &p, nil
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
		empty, err := s.EnvironmentIsEmpty(ctx, teamID, e.ID)
		if err != nil {
			return nil, err
		}
		e.IsEmpty = empty
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) CreateEnvironment(ctx context.Context, teamID, projectID uuid.UUID, name, desc string) (*Environment, error) {
	if _, err := s.GetProject(ctx, teamID, projectID); err != nil {
		return nil, err
	}
	var e Environment
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO environments (team_id, project_id, name, description) VALUES ($1,$2,$3,$4)
		RETURNING id, team_id, project_id, name, description, created_at
	`, teamID, projectID, name, desc).Scan(&e.ID, &e.TeamID, &e.ProjectID, &e.Name, &e.Description, &e.CreatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "environments_project_id_name") || strings.Contains(err.Error(), "duplicate") {
			return nil, ErrConflict
		}
		return nil, err
	}
	e.IsEmpty = true
	return &e, nil
}

// ProjectIsEmpty mirrors Coolify Project::isEmpty — no apps/dbs/services in any env.
func (s *Store) ProjectIsEmpty(ctx context.Context, teamID, projectID uuid.UUID) (bool, error) {
	return s.projectIsEmptyTx(ctx, s.Pool, teamID, projectID)
}

type queryRower interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func (s *Store) projectIsEmptyTx(ctx context.Context, q queryRower, teamID, projectID uuid.UUID) (bool, error) {
	var n int
	err := q.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM applications a
			 JOIN environments e ON e.id = a.environment_id
			 WHERE e.project_id=$1 AND a.team_id=$2)
			+
			(SELECT COUNT(*) FROM databases d
			 JOIN environments e ON e.id = d.environment_id
			 WHERE e.project_id=$1 AND d.team_id=$2)
			+
			(SELECT COUNT(*) FROM services svc
			 JOIN environments e ON e.id = svc.environment_id
			 WHERE e.project_id=$1 AND svc.team_id=$2)
	`, projectID, teamID).Scan(&n)
	return n == 0, err
}

// EnvironmentIsEmpty is true when the environment has no apps/dbs/services.
func (s *Store) EnvironmentIsEmpty(ctx context.Context, teamID, envID uuid.UUID) (bool, error) {
	return s.environmentIsEmptyTx(ctx, s.Pool, teamID, envID)
}

func (s *Store) environmentIsEmptyTx(ctx context.Context, q queryRower, teamID, envID uuid.UUID) (bool, error) {
	var n int
	err := q.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM applications WHERE environment_id=$1 AND team_id=$2)
			+
			(SELECT COUNT(*) FROM databases WHERE environment_id=$1 AND team_id=$2)
			+
			(SELECT COUNT(*) FROM services WHERE environment_id=$1 AND team_id=$2)
	`, envID, teamID).Scan(&n)
	return n == 0, err
}

func (s *Store) UpdateProject(ctx context.Context, teamID, id uuid.UUID, name, description string) (*Project, error) {
	var p Project
	err := s.Pool.QueryRow(ctx, `
		UPDATE projects SET name=$3, description=$4, updated_at=NOW()
		WHERE id=$1 AND team_id=$2
		RETURNING id, team_id, name, description, created_at
	`, id, teamID, name, description).Scan(&p.ID, &p.TeamID, &p.Name, &p.Description, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		if strings.Contains(err.Error(), "projects_team_id_name") || strings.Contains(err.Error(), "duplicate") {
			return nil, ErrConflict
		}
		return nil, err
	}
	empty, err := s.ProjectIsEmpty(ctx, teamID, id)
	if err != nil {
		return nil, err
	}
	p.IsEmpty = empty
	return &p, nil
}

func (s *Store) DeleteProject(ctx context.Context, teamID, id uuid.UUID) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var locked uuid.UUID
	err = tx.QueryRow(ctx, `
		SELECT id FROM projects WHERE id=$1 AND team_id=$2 FOR UPDATE
	`, id, teamID).Scan(&locked)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	empty, err := s.projectIsEmptyTx(ctx, tx, teamID, id)
	if err != nil {
		return err
	}
	if !empty {
		return ErrNotEmpty
	}
	tag, err := tx.Exec(ctx, `DELETE FROM projects WHERE id=$1 AND team_id=$2`, id, teamID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return tx.Commit(ctx)
}

func (s *Store) UpdateEnvironment(ctx context.Context, teamID, projectID, envID uuid.UUID, name, description string) (*Environment, error) {
	var e Environment
	err := s.Pool.QueryRow(ctx, `
		UPDATE environments SET name=$4, description=$5, updated_at=NOW()
		WHERE id=$1 AND project_id=$2 AND team_id=$3
		RETURNING id, team_id, project_id, name, description, created_at
	`, envID, projectID, teamID, name, description).Scan(
		&e.ID, &e.TeamID, &e.ProjectID, &e.Name, &e.Description, &e.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		if strings.Contains(err.Error(), "environments_project_id_name") || strings.Contains(err.Error(), "duplicate") {
			return nil, ErrConflict
		}
		return nil, err
	}
	empty, err := s.EnvironmentIsEmpty(ctx, teamID, envID)
	if err != nil {
		return nil, err
	}
	e.IsEmpty = empty
	return &e, nil
}

func (s *Store) DeleteEnvironment(ctx context.Context, teamID, projectID, envID uuid.UUID) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var locked uuid.UUID
	err = tx.QueryRow(ctx, `
		SELECT id FROM environments WHERE id=$1 AND project_id=$2 AND team_id=$3 FOR UPDATE
	`, envID, projectID, teamID).Scan(&locked)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	empty, err := s.environmentIsEmptyTx(ctx, tx, teamID, envID)
	if err != nil {
		return err
	}
	if !empty {
		return ErrNotEmpty
	}
	tag, err := tx.Exec(ctx, `
		DELETE FROM environments WHERE id=$1 AND project_id=$2 AND team_id=$3
	`, envID, projectID, teamID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return tx.Commit(ctx)
}

func (s *Store) CreateApplication(ctx context.Context, app *Application) (*Application, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	base := strings.TrimSpace(app.BaseDirectory)
	if base == "" {
		base = "/"
	}
	err = tx.QueryRow(ctx, `
		INSERT INTO applications (
			team_id, environment_id, destination_id, name, description, fqdn, build_pack,
			git_repository, git_branch, dockerfile_location, dockerfile, docker_compose_location,
			docker_registry_image_name, docker_registry_image_tag, ports_exposes,
			compose_prepare, git_source_id, private_key_id, base_directory
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)
		RETURNING id
	`, app.TeamID, app.EnvironmentID, app.DestinationID, app.Name, app.Description, app.FQDN, app.BuildPack,
		app.GitRepository, app.GitBranch, app.DockerfileLocation, app.Dockerfile, app.DockerComposeLocation,
		app.DockerRegistryImageName, app.DockerRegistryImageTag, app.PortsExposes,
		app.ComposePrepare, app.GitSourceID, app.PrivateKeyID, base,
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
	if _, err := tx.Exec(ctx, `DELETE FROM volumes WHERE team_id=$1 AND resource_type='application' AND resource_id=$2`, teamID, id); err != nil {
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

func (s *Store) UpdateApplicationComposePreview(ctx context.Context, teamID, id uuid.UUID, raw, prepared string) error {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE applications SET docker_compose_raw=$3, docker_compose=$4, updated_at=NOW()
		WHERE id=$1 AND team_id=$2
	`, id, teamID, raw, prepared)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) UpdateApplicationStatus(ctx context.Context, id uuid.UUID, status string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE applications SET status=$2, updated_at=NOW() WHERE id=$1`, id, status)
	return err
}

// UpdateApplicationGitCommitSHA persists the last successfully deployed commit (Coolify parity).
func (s *Store) UpdateApplicationGitCommitSHA(ctx context.Context, id uuid.UUID, sha string) error {
	sha = strings.TrimSpace(sha)
	if sha == "" || strings.EqualFold(sha, "HEAD") {
		return nil
	}
	_, err := s.Pool.Exec(ctx, `UPDATE applications SET git_commit_sha=$2, updated_at=NOW() WHERE id=$1`, id, sha)
	return err
}

func (s *Store) UpdateApplication(ctx context.Context, app *Application) error {
	domains := app.DockerComposeDomains
	if len(domains) == 0 {
		domains = json.RawMessage(`{}`)
	}
	base := strings.TrimSpace(app.BaseDirectory)
	if base == "" {
		base = "/"
	}
	_, err := s.Pool.Exec(ctx, `
		UPDATE applications SET
			name=$2, description=$3, fqdn=$4, git_repository=$5, git_branch=$6,
			ports_exposes=$7, docker_registry_image_name=$8, docker_registry_image_tag=$9,
			dockerfile_location=$10, dockerfile=$11, docker_compose_location=$12, destination_id=$13, git_source_id=$14, private_key_id=$15,
			compose_prepare=$16,
			docker_compose_raw=$17, docker_compose=$18, docker_compose_domains=$19,
			base_directory=$20,
			docker_compose_custom_build_command=$21, docker_compose_custom_start_command=$22,
			custom_docker_run_options=$23, dockerfile_target_build=$24,
			health_check_enabled=$25, health_check_path=$26, health_check_port=$27, health_check_method=$28,
			health_check_return_code=$29, health_check_interval=$30, health_check_timeout=$31, health_check_retries=$32,
			limits_memory=$33, limits_cpus=$34, is_force_https=$35,
			pre_deployment_command=$36, post_deployment_command=$37, custom_labels=$38,
			http_basic_auth_username=$39, http_basic_auth_password_enc=$40,
			redirect=$41, docker_registry_id=$42,
			ports_mappings=$43, custom_network_aliases=$44,
			install_command=$45, build_command=$46, start_command=$47, publish_directory=$48,
			custom_nginx_configuration=$49, preview_url_template=$50,
			health_check_host=$51, health_check_scheme=$52, health_check_response_text=$53,
			health_check_start_period=$54, health_check_type=$55, health_check_command=$56,
			swarm_replicas=$57, swarm_placement_constraints=$58,
			build_pack=$59,
			updated_at=NOW()
		WHERE id=$1 AND team_id=$60
	`, app.ID, app.Name, app.Description, app.FQDN, app.GitRepository, app.GitBranch,
		app.PortsExposes, app.DockerRegistryImageName, app.DockerRegistryImageTag,
		app.DockerfileLocation, app.Dockerfile, app.DockerComposeLocation, app.DestinationID, app.GitSourceID, app.PrivateKeyID,
		app.ComposePrepare,
		app.DockerComposeRaw, app.DockerCompose, domains,
		base,
		app.DockerComposeCustomBuildCommand, app.DockerComposeCustomStartCommand,
		app.CustomDockerRunOptions, app.DockerfileTargetBuild,
		app.HealthCheckEnabled, app.HealthCheckPath, app.HealthCheckPort, app.HealthCheckMethod,
		app.HealthCheckReturnCode, app.HealthCheckInterval, app.HealthCheckTimeout, app.HealthCheckRetries,
		app.LimitsMemory, app.LimitsCpus, app.IsForceHTTPS,
		app.PreDeploymentCommand, app.PostDeploymentCommand, app.CustomLabels,
		app.HTTPBasicAuthUsername, app.HTTPBasicAuthPasswordEnc,
		app.Redirect, app.DockerRegistryID,
		app.PortsMappings, app.CustomNetworkAliases,
		app.InstallCommand, app.BuildCommand, app.StartCommand, app.PublishDirectory,
		app.CustomNginxConfiguration, app.PreviewURLTemplate,
		app.HealthCheckHost, app.HealthCheckScheme, app.HealthCheckResponseText,
		app.HealthCheckStartPeriod, app.HealthCheckType, app.HealthCheckCommand,
		app.SwarmReplicas, app.SwarmPlacementConstraints,
		app.BuildPack,
		app.TeamID)
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

func (s *Store) SetApplicationPreviewEnabled(ctx context.Context, teamID, appID uuid.UUID, enabled bool) error {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE application_settings SET is_preview_enabled=$3, updated_at=NOW()
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

func (s *Store) SetApplicationAutoDeployEnabled(ctx context.Context, teamID, appID uuid.UUID, enabled bool) error {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE application_settings SET is_auto_deploy_enabled=$3, updated_at=NOW()
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

func (s *Store) SetApplicationGitSubmodulesEnabled(ctx context.Context, teamID, appID uuid.UUID, enabled bool) error {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE application_settings SET is_git_submodules_enabled=$3, updated_at=NOW()
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

func (s *Store) SetApplicationPreserveRepositoryEnabled(ctx context.Context, teamID, appID uuid.UUID, enabled bool) error {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE application_settings SET is_preserve_repository_enabled=$3, updated_at=NOW()
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

func (s *Store) SetApplicationWatchPaths(ctx context.Context, teamID, appID uuid.UUID, paths string) error {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE application_settings SET watch_paths=$3, updated_at=NOW()
		WHERE application_id=$1 AND EXISTS (SELECT 1 FROM applications WHERE id=$1 AND team_id=$2)
	`, appID, teamID, paths)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) SetApplicationForceHTTPS(ctx context.Context, teamID, appID uuid.UUID, enabled bool) error {
	// Keep applications + settings columns in sync (GET uses COALESCE of both).
	tag, err := s.Pool.Exec(ctx, `
		UPDATE applications SET is_force_https=$3, updated_at=NOW()
		WHERE id=$1 AND team_id=$2
	`, appID, teamID, enabled)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	_, err = s.Pool.Exec(ctx, `
		UPDATE application_settings SET is_force_https_enabled=$2, updated_at=NOW()
		WHERE application_id=$1
	`, appID, enabled)
	return err
}

// SetApplicationAdvancedSettings updates Coolify-style advanced deploy flags on application_settings.
func (s *Store) SetApplicationAdvancedSettings(ctx context.Context, teamID, appID uuid.UUID, in ApplicationAdvancedSettings) error {
	policy := strings.TrimSpace(in.CustomDockerRestartPolicy)
	if policy == "" {
		policy = "unless-stopped"
	}
	keep := in.DockerImagesToKeep
	if keep <= 0 {
		keep = 5
	}
	logsLimit := in.LogsLineLimit
	if logsLimit <= 0 {
		logsLimit = 1000
	}
	driver := strings.TrimSpace(in.GPUDriver)
	if driver == "" {
		driver = "nvidia"
	}
	tag, err := s.Pool.Exec(ctx, `
		UPDATE application_settings SET
			is_disable_build_cache=$3,
			is_git_shallow_clone_enabled=$4,
			is_git_lfs_enabled=$5,
			is_gpu_enabled=$6,
			gpu_count=$7,
			custom_docker_stop_timeout=$8,
			custom_docker_restart_policy=$9,
			is_spa=$10,
			inject_build_args_to_dockerfile=$11,
			use_build_secrets=$12,
			include_source_commit_in_build=$13,
			docker_images_to_keep=$14,
			is_consistent_container_name_enabled=$15,
			custom_internal_name=$16,
			is_gzip_enabled=$17,
			is_stripprefix_enabled=$18,
			is_log_drain_enabled=$19,
			is_debug_enabled=$20,
			is_env_sorting_enabled=$21,
			is_pr_deployments_public_enabled=$22,
			skip_rebuild_if_unchanged=$23,
			gpu_driver=$24,
			gpu_device_ids=$25,
			gpu_options=$26,
			custom_docker_max_restart_count=$27,
			pre_deployment_command_container=$28,
			post_deployment_command_container=$29,
			is_swarm_only_worker_nodes=$30,
			is_include_timestamps=$31,
			logs_line_limit=$32,
			updated_at=NOW()
		WHERE application_id=$1 AND EXISTS (SELECT 1 FROM applications WHERE id=$1 AND team_id=$2)
	`, appID, teamID,
		in.IsDisableBuildCache, in.IsGitShallowCloneEnabled, in.IsGitLFSEnabled,
		in.IsGPUEnabled, in.GPUCount, in.CustomDockerStopTimeout, policy,
		in.IsSPA, in.InjectBuildArgsToDockerfile, in.UseBuildSecrets, in.IncludeSourceCommitInBuild,
		keep, in.IsConsistentContainerNameEnabled, in.CustomInternalName,
		in.IsGzipEnabled, in.IsStripPrefixEnabled, in.IsLogDrainEnabled, in.IsDebugEnabled,
		in.IsEnvSortingEnabled, in.IsPRDeploymentsPublicEnabled, in.SkipRebuildIfUnchanged,
		driver, in.GPUDeviceIDs, in.GPUOptions, in.CustomDockerMaxRestartCount,
		in.PreDeploymentCommandContainer, in.PostDeploymentCommandContainer,
		in.IsSwarmOnlyWorkerNodes, in.IsIncludeTimestamps, logsLimit)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

type ApplicationAdvancedSettings struct {
	IsDisableBuildCache              bool
	IsGitShallowCloneEnabled         bool
	IsGitLFSEnabled                  bool
	IsGPUEnabled                     bool
	GPUCount                         int
	CustomDockerStopTimeout          int
	CustomDockerRestartPolicy        string
	IsSPA                            bool
	InjectBuildArgsToDockerfile      bool
	UseBuildSecrets                  bool
	IncludeSourceCommitInBuild       bool
	DockerImagesToKeep               int
	IsConsistentContainerNameEnabled bool
	CustomInternalName               string
	IsGzipEnabled                    bool
	IsStripPrefixEnabled             bool
	IsLogDrainEnabled                bool
	IsDebugEnabled                   bool
	IsEnvSortingEnabled              bool
	IsPRDeploymentsPublicEnabled     bool
	SkipRebuildIfUnchanged           bool
	GPUDriver                        string
	GPUDeviceIDs                     string
	GPUOptions                       string
	CustomDockerMaxRestartCount      int
	PreDeploymentCommandContainer    string
	PostDeploymentCommandContainer   string
	IsSwarmOnlyWorkerNodes           bool
	IsIncludeTimestamps              bool
	LogsLineLimit                    int
}

// CloneApplication duplicates a single application into the same or another environment.
// Copies config + env vars + volumes metadata + scheduled tasks/backups. Clears FQDN.
func (s *Store) CloneApplication(ctx context.Context, teamID, appID uuid.UUID, name string, environmentID *uuid.UUID) (*Application, error) {
	src, err := s.GetApplication(ctx, teamID, appID)
	if err != nil {
		return nil, err
	}
	envID := src.EnvironmentID
	if environmentID != nil {
		env, err := s.GetEnvironment(ctx, teamID, *environmentID)
		if err != nil {
			return nil, err
		}
		srcEnv, err := s.GetEnvironment(ctx, teamID, src.EnvironmentID)
		if err != nil {
			return nil, err
		}
		if env.ProjectID != srcEnv.ProjectID {
			return nil, fmt.Errorf("%w: target environment must be in the same project", ErrConflict)
		}
		envID = env.ID
	}
	clone := *src
	clone.ID = uuid.Nil
	clone.EnvironmentID = envID
	clone.FQDN = ""
	clone.Status = "exited"
	clone.DockerCompose = ""
	name = strings.TrimSpace(name)
	if name == "" {
		name = uniqueCloneName(src.Name, "clone")
	}
	clone.Name = name
	created, err := s.CreateApplication(ctx, &clone)
	if err != nil {
		return nil, err
	}
	// Copy application-level columns not set on INSERT.
	clone.ID = created.ID
	clone.TeamID = teamID
	if err := s.UpdateApplication(ctx, &clone); err != nil {
		_ = s.DeleteApplication(ctx, teamID, created.ID)
		return nil, err
	}
	adv := ApplicationAdvancedSettings{
		IsDisableBuildCache:              src.IsDisableBuildCache,
		IsGitShallowCloneEnabled:         src.IsGitShallowCloneEnabled,
		IsGitLFSEnabled:                  src.IsGitLFSEnabled,
		IsGPUEnabled:                     src.IsGPUEnabled,
		GPUCount:                         src.GPUCount,
		CustomDockerStopTimeout:          src.CustomDockerStopTimeout,
		CustomDockerRestartPolicy:        src.CustomDockerRestartPolicy,
		IsSPA:                            src.IsSPA,
		InjectBuildArgsToDockerfile:      src.InjectBuildArgsToDockerfile,
		UseBuildSecrets:                  src.UseBuildSecrets,
		IncludeSourceCommitInBuild:       src.IncludeSourceCommitInBuild,
		DockerImagesToKeep:               src.DockerImagesToKeep,
		IsConsistentContainerNameEnabled: src.IsConsistentContainerNameEnabled,
		CustomInternalName:               "",
		IsGzipEnabled:                    src.IsGzipEnabled,
		IsStripPrefixEnabled:             src.IsStripPrefixEnabled,
		IsLogDrainEnabled:                src.IsLogDrainEnabled,
		IsDebugEnabled:                   src.IsDebugEnabled,
		IsEnvSortingEnabled:              src.IsEnvSortingEnabled,
		IsPRDeploymentsPublicEnabled:     src.IsPRDeploymentsPublicEnabled,
		SkipRebuildIfUnchanged:           src.SkipRebuildIfUnchanged,
		GPUDriver:                        src.GPUDriver,
		GPUDeviceIDs:                     src.GPUDeviceIDs,
		GPUOptions:                       src.GPUOptions,
		CustomDockerMaxRestartCount:      src.CustomDockerMaxRestartCount,
		PreDeploymentCommandContainer:    src.PreDeploymentCommandContainer,
		PostDeploymentCommandContainer:   src.PostDeploymentCommandContainer,
		IsSwarmOnlyWorkerNodes:           src.IsSwarmOnlyWorkerNodes,
		IsIncludeTimestamps:              src.IsIncludeTimestamps,
		LogsLineLimit:                    src.LogsLineLimit,
	}
	_ = s.SetApplicationAdvancedSettings(ctx, teamID, created.ID, adv)
	_ = s.SetApplicationPreviewEnabled(ctx, teamID, created.ID, src.IsPreviewEnabled)
	_ = s.SetApplicationAutoDeployEnabled(ctx, teamID, created.ID, src.IsAutoDeployEnabled)
	_ = s.SetApplicationGitSubmodulesEnabled(ctx, teamID, created.ID, src.IsGitSubmodulesEnabled)
	_ = s.SetApplicationPreserveRepositoryEnabled(ctx, teamID, created.ID, src.IsPreserveRepositoryEnabled)
	_ = s.SetApplicationWatchPaths(ctx, teamID, created.ID, src.WatchPaths)
	_ = s.SetApplicationBuildServerEnabled(ctx, teamID, created.ID, src.IsBuildServerEnabled)
	_ = s.SetApplicationForceHTTPS(ctx, teamID, created.ID, src.IsForceHTTPS)
	if err := s.copyResourceExtras(ctx, teamID, "application", src.ID, created.ID); err != nil {
		_ = s.DeleteApplication(ctx, teamID, created.ID)
		return nil, err
	}
	return s.GetApplication(ctx, teamID, created.ID)
}

// MoveResource relocates an application, database, or service to another environment.
// Target must belong to the same team and the same project as the resource's current environment.
// Runtime containers and destinations are unchanged — only the environment association moves.
func (s *Store) MoveResource(ctx context.Context, teamID uuid.UUID, resourceType string, resourceID, envID uuid.UUID) error {
	target, err := s.GetEnvironment(ctx, teamID, envID)
	if err != nil {
		return err
	}
	var currentEnvID uuid.UUID
	switch resourceType {
	case "application":
		app, err := s.GetApplication(ctx, teamID, resourceID)
		if err != nil {
			return err
		}
		currentEnvID = app.EnvironmentID
	case "database":
		db, err := s.GetDatabase(ctx, teamID, resourceID)
		if err != nil {
			return err
		}
		currentEnvID = db.EnvironmentID
	case "service":
		svc, err := s.GetService(ctx, teamID, resourceID)
		if err != nil {
			return err
		}
		currentEnvID = svc.EnvironmentID
	default:
		return ErrNotFound
	}
	if currentEnvID == envID {
		return nil // already there
	}
	current, err := s.GetEnvironment(ctx, teamID, currentEnvID)
	if err != nil {
		return err
	}
	if current.ProjectID != target.ProjectID {
		return fmt.Errorf("%w: target environment must be in the same project", ErrConflict)
	}
	var query string
	switch resourceType {
	case "application":
		query = `UPDATE applications SET environment_id=$3, updated_at=NOW() WHERE id=$1 AND team_id=$2`
	case "database":
		query = `UPDATE databases SET environment_id=$3, updated_at=NOW() WHERE id=$1 AND team_id=$2`
	case "service":
		query = `UPDATE services SET environment_id=$3, updated_at=NOW() WHERE id=$1 AND team_id=$2`
	}
	tag, err := s.Pool.Exec(ctx, query, resourceID, teamID, envID)
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

const deploymentColumns = `id, team_id, application_id, service_id, server_id, status, commit_sha, commit_message, pull_request_id, current_stage, logs, error_message, started_at, finished_at, created_at`

func scanDeployment(scan func(dest ...any) error) (*Deployment, error) {
	var d Deployment
	err := scan(
		&d.ID, &d.TeamID, &d.ApplicationID, &d.ServiceID, &d.ServerID, &d.Status, &d.CommitSHA, &d.CommitMessage, &d.PullRequestID,
		&d.CurrentStage, &d.Logs, &d.ErrorMessage, &d.StartedAt, &d.FinishedAt, &d.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (s *Store) CreateDeployment(ctx context.Context, teamID, appID uuid.UUID, serverID *uuid.UUID, commitSHA, commitMsg string, forceRebuild, isWebhook, isAPI bool, pullRequestID int) (*Deployment, error) {
	row := s.Pool.QueryRow(ctx, `
		INSERT INTO deployments (team_id, application_id, server_id, status, commit_sha, commit_message, force_rebuild, is_webhook, is_api, pull_request_id)
		VALUES ($1,$2,$3,'queued',$4,$5,$6,$7,$8,$9)
		RETURNING `+deploymentColumns+`
	`, teamID, appID, serverID, commitSHA, commitMsg, forceRebuild, isWebhook, isAPI, pullRequestID)
	return scanDeployment(row.Scan)
}

func (s *Store) CreateServiceDeployment(ctx context.Context, teamID, serviceID uuid.UUID, serverID *uuid.UUID, forceRebuild, isWebhook, isAPI bool) (*Deployment, error) {
	row := s.Pool.QueryRow(ctx, `
		INSERT INTO deployments (team_id, service_id, server_id, status, force_rebuild, is_webhook, is_api)
		VALUES ($1,$2,$3,'queued',$4,$5,$6)
		RETURNING `+deploymentColumns+`
	`, teamID, serviceID, serverID, forceRebuild, isWebhook, isAPI)
	return scanDeployment(row.Scan)
}

func (s *Store) GetDeployment(ctx context.Context, teamID, id uuid.UUID) (*Deployment, error) {
	row := s.Pool.QueryRow(ctx, `
		SELECT `+deploymentColumns+`
		FROM deployments WHERE id=$1 AND team_id=$2
	`, id, teamID)
	d, err := scanDeployment(row.Scan)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return d, err
}

func (s *Store) ListDeployments(ctx context.Context, teamID, appID uuid.UUID, limit int) ([]Deployment, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT `+deploymentColumns+`
		FROM deployments WHERE team_id=$1 AND application_id=$2
		ORDER BY created_at DESC LIMIT $3
	`, teamID, appID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Deployment
	for rows.Next() {
		d, err := scanDeployment(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, *d)
	}
	return out, rows.Err()
}

func (s *Store) ListServiceDeployments(ctx context.Context, teamID, serviceID uuid.UUID, limit int) ([]Deployment, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT `+deploymentColumns+`
		FROM deployments WHERE team_id=$1 AND service_id=$2
		ORDER BY created_at DESC LIMIT $3
	`, teamID, serviceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Deployment
	for rows.Next() {
		d, err := scanDeployment(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, *d)
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
	errMsg = redact.Secrets(errMsg)
	// Never overwrite a cancelled deployment with finished/failed/in_progress.
	tag, err := s.Pool.Exec(ctx, `
		UPDATE deployments SET
			status=$2,
			error_message=$3,
			started_at = CASE WHEN $2='in_progress' AND started_at IS NULL THEN NOW() ELSE started_at END,
			finished_at = CASE WHEN $2 IN ('finished','failed','cancelled') THEN NOW() ELSE finished_at END,
			updated_at=NOW()
		WHERE id=$1
		  AND NOT (status='cancelled' AND $2 IN ('queued','in_progress','finished','failed'))
	`, id, status, errMsg)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		var cur string
		_ = s.Pool.QueryRow(ctx, `SELECT status FROM deployments WHERE id=$1`, id).Scan(&cur)
		if cur == "cancelled" {
			return nil
		}
	}
	return nil
}

func (s *Store) IsDeploymentCancelled(ctx context.Context, id uuid.UUID) (bool, error) {
	var status string
	err := s.Pool.QueryRow(ctx, `SELECT status FROM deployments WHERE id=$1`, id).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ErrNotFound
	}
	return status == "cancelled", err
}

func (s *Store) GetPreviewByPR(ctx context.Context, teamID, appID uuid.UUID, prID int) (*ApplicationPreview, error) {
	var p ApplicationPreview
	err := s.Pool.QueryRow(ctx, `
		SELECT id, team_id, application_id, pull_request_id, pull_request_title, git_branch, fqdn, status, created_at
		FROM application_previews WHERE team_id=$1 AND application_id=$2 AND pull_request_id=$3
	`, teamID, appID, prID).Scan(
		&p.ID, &p.TeamID, &p.ApplicationID, &p.PullRequestID, &p.PullRequestTitle, &p.GitBranch, &p.FQDN, &p.Status, &p.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
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
		SELECT `+deploymentColumns+`
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
		d, err := scanDeployment(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, *d)
	}
	return out, rows.Err()
}

func (s *Store) UpdateServiceStatus(ctx context.Context, id uuid.UUID, status string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE services SET status=$2, updated_at=NOW() WHERE id=$1`, id, status)
	return err
}

func (s *Store) DeleteService(ctx context.Context, teamID, id uuid.UUID) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var exists uuid.UUID
	err = tx.QueryRow(ctx, `SELECT id FROM services WHERE id=$1 AND team_id=$2`, id, teamID).Scan(&exists)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM environment_variables WHERE team_id=$1 AND resource_type='service' AND resource_id=$2`, teamID, id); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM scheduled_tasks WHERE team_id=$1 AND resource_type='service' AND resource_id=$2`, teamID, id); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM scheduled_backups WHERE team_id=$1 AND resource_type='service' AND resource_id=$2`, teamID, id); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM volumes WHERE team_id=$1 AND resource_type='service' AND resource_id=$2`, teamID, id); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM service_components WHERE service_id=$1`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM services WHERE id=$1 AND team_id=$2`, id, teamID); err != nil {
		return err
	}
	return tx.Commit(ctx)
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

// ListApplicationsForGitWebhook returns apps on a git source whose repository matches
// repoFullName and whose git_branch matches branch (case-sensitive branch, Coolify-style).
func (s *Store) ListApplicationsForGitWebhook(ctx context.Context, gitSourceID uuid.UUID, repoFullName, branch string) ([]Application, error) {
	branch = strings.TrimSpace(branch)
	if gitSourceID == uuid.Nil || branch == "" {
		return nil, nil
	}
	rows, err := s.Pool.Query(ctx, `SELECT `+applicationSelectCols+`
		FROM applications a
		LEFT JOIN application_settings s ON s.application_id = a.id
		WHERE a.git_source_id=$1 AND a.git_branch=$2
		ORDER BY a.name`, gitSourceID, branch)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	want := git.NormalizeRepoFullName(repoFullName)
	if want == "" {
		// Refuse to match every app on the source when the payload has no repo.
		return nil, nil
	}
	var out []Application
	for rows.Next() {
		a, err := scanApplication(rows.Scan)
		if err != nil {
			return nil, err
		}
		if !git.RepoNamesMatch(a.GitRepository, want) {
			continue
		}
		out = append(out, *a)
	}
	return out, rows.Err()
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

func (s *Store) UpdatePreviewStatus(ctx context.Context, teamID, appID uuid.UUID, prID int, status string) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE application_previews SET status=$4, updated_at=NOW()
		WHERE team_id=$1 AND application_id=$2 AND pull_request_id=$3
	`, teamID, appID, prID, status)
	return err
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
	ID           uuid.UUID  `json:"id"`
	TeamID       uuid.UUID  `json:"team_id"`
	ResourceType string     `json:"resource_type"`
	ResourceID   uuid.UUID  `json:"resource_id"`
	S3StorageID  *uuid.UUID `json:"s3_storage_id,omitempty"`
	VolumeID     *uuid.UUID `json:"volume_id,omitempty"`
	Frequency    string     `json:"frequency"`
	Enabled      bool       `json:"enabled"`
	Retention    int        `json:"retention"`
	CreatedAt    time.Time  `json:"created_at"`
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

func (s *Store) CreateScheduledBackup(ctx context.Context, teamID uuid.UUID, resourceType string, resourceID uuid.UUID, s3ID, volumeID *uuid.UUID, frequency string, retention int) (*ScheduledBackup, error) {
	if frequency == "" {
		frequency = "0 0 * * *"
	}
	if retention <= 0 {
		retention = 7
	}
	var b ScheduledBackup
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO scheduled_backups (team_id, resource_type, resource_id, s3_storage_id, volume_id, frequency, retention)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING id, team_id, resource_type, resource_id, s3_storage_id, volume_id, frequency, enabled, retention, created_at
	`, teamID, resourceType, resourceID, s3ID, volumeID, frequency, retention).Scan(
		&b.ID, &b.TeamID, &b.ResourceType, &b.ResourceID, &b.S3StorageID, &b.VolumeID, &b.Frequency, &b.Enabled, &b.Retention, &b.CreatedAt,
	)
	return &b, err
}

type UpdateScheduledBackupInput struct {
	Frequency   *string
	Retention   *int
	Enabled     *bool
	S3StorageID **uuid.UUID // pointer to pointer: nil = omit, &nil = clear, &id = set
}

func (s *Store) UpdateScheduledBackup(ctx context.Context, teamID, id uuid.UUID, in UpdateScheduledBackupInput) (*ScheduledBackup, error) {
	cur, err := s.GetScheduledBackup(ctx, teamID, id)
	if err != nil {
		return nil, err
	}
	freq, ret, en := cur.Frequency, cur.Retention, cur.Enabled
	s3 := cur.S3StorageID
	if in.Frequency != nil {
		freq = strings.TrimSpace(*in.Frequency)
		if freq == "" {
			freq = "0 0 * * *"
		}
	}
	if in.Retention != nil {
		ret = *in.Retention
		if ret <= 0 {
			ret = 7
		}
	}
	if in.Enabled != nil {
		en = *in.Enabled
	}
	if in.S3StorageID != nil {
		s3 = *in.S3StorageID
	}
	_, err = s.Pool.Exec(ctx, `
		UPDATE scheduled_backups
		SET frequency=$3, retention=$4, enabled=$5, s3_storage_id=$6, updated_at=NOW()
		WHERE id=$1 AND team_id=$2
	`, id, teamID, freq, ret, en, s3)
	if err != nil {
		return nil, err
	}
	return s.GetScheduledBackup(ctx, teamID, id)
}

func (s *Store) DeleteScheduledBackup(ctx context.Context, teamID, id uuid.UUID) error {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM scheduled_backups WHERE id=$1 AND team_id=$2`, id, teamID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) GetScheduledBackup(ctx context.Context, teamID, id uuid.UUID) (*ScheduledBackup, error) {
	var b ScheduledBackup
	err := s.Pool.QueryRow(ctx, `
		SELECT id, team_id, resource_type, resource_id, s3_storage_id, volume_id, frequency, enabled, retention, created_at
		FROM scheduled_backups WHERE id=$1 AND team_id=$2
	`, id, teamID).Scan(&b.ID, &b.TeamID, &b.ResourceType, &b.ResourceID, &b.S3StorageID, &b.VolumeID, &b.Frequency, &b.Enabled, &b.Retention, &b.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &b, err
}

func (s *Store) ListScheduledBackups(ctx context.Context, teamID uuid.UUID) ([]ScheduledBackup, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, team_id, resource_type, resource_id, s3_storage_id, volume_id, frequency, enabled, retention, created_at
		FROM scheduled_backups WHERE team_id=$1 ORDER BY created_at DESC
	`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ScheduledBackup
	for rows.Next() {
		var b ScheduledBackup
		if err := rows.Scan(&b.ID, &b.TeamID, &b.ResourceType, &b.ResourceID, &b.S3StorageID, &b.VolumeID, &b.Frequency, &b.Enabled, &b.Retention, &b.CreatedAt); err != nil {
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
	VolumeID     *uuid.UUID
	Frequency    string
	Enabled      bool
	Retention    int
}

func (s *Store) ListEnabledScheduledBackups(ctx context.Context) ([]ScheduledBackupRow, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, team_id, resource_type, resource_id, s3_storage_id, volume_id, frequency, enabled, retention
		FROM scheduled_backups WHERE enabled=TRUE
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ScheduledBackupRow
	for rows.Next() {
		var b ScheduledBackupRow
		if err := rows.Scan(&b.ID, &b.TeamID, &b.ResourceType, &b.ResourceID, &b.S3StorageID, &b.VolumeID, &b.Frequency, &b.Enabled, &b.Retention); err != nil {
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
