-- +goose Up
CREATE TABLE projects (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id         UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (team_id, name)
);

CREATE INDEX projects_team_id_idx ON projects(team_id);

CREATE TABLE environments (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id         UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    project_id      UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (project_id, name)
);

CREATE INDEX environments_team_id_idx ON environments(team_id);
CREATE INDEX environments_project_id_idx ON environments(project_id);

CREATE TABLE shared_environment_variables (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id         UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    scope_type      TEXT NOT NULL CHECK (scope_type IN ('team', 'project', 'environment', 'server')),
    scope_id        UUID,
    key             TEXT NOT NULL,
    value_enc       TEXT NOT NULL,
    is_literal      BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX shared_env_team_idx ON shared_environment_variables(team_id);
CREATE UNIQUE INDEX shared_env_unique_idx ON shared_environment_variables(team_id, scope_type, COALESCE(scope_id, '00000000-0000-0000-0000-000000000000'), key);

CREATE TABLE applications (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id             UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    environment_id      UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    destination_id      UUID REFERENCES destinations(id) ON DELETE SET NULL,
    name                TEXT NOT NULL,
    description         TEXT NOT NULL DEFAULT '',
    fqdn                TEXT NOT NULL DEFAULT '',
    status              TEXT NOT NULL DEFAULT 'exited',
    build_pack          TEXT NOT NULL DEFAULT 'dockerfile'
                        CHECK (build_pack IN ('dockerfile', 'dockercompose', 'dockerimage', 'nixpacks', 'static', 'railpack')),
    git_repository      TEXT NOT NULL DEFAULT '',
    git_branch          TEXT NOT NULL DEFAULT 'main',
    git_commit_sha      TEXT NOT NULL DEFAULT 'HEAD',
    dockerfile_location TEXT NOT NULL DEFAULT '/Dockerfile',
    docker_compose_location TEXT NOT NULL DEFAULT '/docker-compose.yaml',
    docker_registry_image_name TEXT NOT NULL DEFAULT '',
    docker_registry_image_tag  TEXT NOT NULL DEFAULT 'latest',
    ports_exposes       TEXT NOT NULL DEFAULT '3000',
    health_check_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    health_check_path   TEXT NOT NULL DEFAULT '/',
    health_check_port   INTEGER,
    health_check_method TEXT NOT NULL DEFAULT 'GET',
    health_check_return_code INTEGER NOT NULL DEFAULT 200,
    health_check_interval INTEGER NOT NULL DEFAULT 5,
    health_check_timeout INTEGER NOT NULL DEFAULT 5,
    health_check_retries INTEGER NOT NULL DEFAULT 10,
    pre_deployment_command TEXT NOT NULL DEFAULT '',
    post_deployment_command TEXT NOT NULL DEFAULT '',
    limits_memory       TEXT NOT NULL DEFAULT '',
    limits_cpus         TEXT NOT NULL DEFAULT '',
    custom_labels       TEXT NOT NULL DEFAULT '',
    http_basic_auth_username TEXT NOT NULL DEFAULT '',
    http_basic_auth_password_enc TEXT NOT NULL DEFAULT '',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX applications_team_id_idx ON applications(team_id);
CREATE INDEX applications_environment_id_idx ON applications(environment_id);

CREATE TABLE application_settings (
    application_id      UUID PRIMARY KEY REFERENCES applications(id) ON DELETE CASCADE,
    is_auto_deploy_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    is_force_https_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    is_git_submodules_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    is_preview_enabled  BOOLEAN NOT NULL DEFAULT FALSE,
    is_build_server_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    watch_paths         TEXT NOT NULL DEFAULT '',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE application_previews (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id             UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    application_id      UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    pull_request_id     INTEGER NOT NULL,
    pull_request_title  TEXT NOT NULL DEFAULT '',
    git_branch          TEXT NOT NULL DEFAULT '',
    fqdn                TEXT NOT NULL DEFAULT '',
    status              TEXT NOT NULL DEFAULT 'exited',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (application_id, pull_request_id)
);

CREATE TABLE environment_variables (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id             UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    resource_type       TEXT NOT NULL,
    resource_id         UUID NOT NULL,
    key                 TEXT NOT NULL,
    value_enc           TEXT NOT NULL,
    is_preview          BOOLEAN NOT NULL DEFAULT FALSE,
    is_runtime          BOOLEAN NOT NULL DEFAULT TRUE,
    is_buildtime        BOOLEAN NOT NULL DEFAULT FALSE,
    is_literal          BOOLEAN NOT NULL DEFAULT FALSE,
    is_multiline        BOOLEAN NOT NULL DEFAULT FALSE,
    is_shown_once       BOOLEAN NOT NULL DEFAULT FALSE,
    comment             TEXT NOT NULL DEFAULT '',
    sort_order          INTEGER NOT NULL DEFAULT 0,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX env_vars_unique_idx ON environment_variables(resource_type, resource_id, key, is_preview);
CREATE INDEX env_vars_team_id_idx ON environment_variables(team_id);

CREATE TABLE deployments (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id             UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    application_id      UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    server_id           UUID REFERENCES servers(id) ON DELETE SET NULL,
    destination_id      UUID REFERENCES destinations(id) ON DELETE SET NULL,
    status              TEXT NOT NULL DEFAULT 'queued'
                        CHECK (status IN ('queued', 'in_progress', 'finished', 'failed', 'cancelled')),
    commit_sha          TEXT NOT NULL DEFAULT '',
    commit_message      TEXT NOT NULL DEFAULT '',
    force_rebuild       BOOLEAN NOT NULL DEFAULT FALSE,
    pull_request_id     INTEGER NOT NULL DEFAULT 0,
    is_webhook          BOOLEAN NOT NULL DEFAULT FALSE,
    is_api              BOOLEAN NOT NULL DEFAULT FALSE,
    logs                JSONB NOT NULL DEFAULT '[]'::jsonb,
    current_stage       TEXT NOT NULL DEFAULT '',
    configuration_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    error_message       TEXT NOT NULL DEFAULT '',
    started_at          TIMESTAMPTZ,
    finished_at         TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX deployments_application_id_idx ON deployments(application_id);
CREATE INDEX deployments_team_id_idx ON deployments(team_id);
CREATE INDEX deployments_status_idx ON deployments(status);

-- +goose Down
DROP TABLE IF EXISTS deployments;
DROP TABLE IF EXISTS environment_variables;
DROP TABLE IF EXISTS application_previews;
DROP TABLE IF EXISTS application_settings;
DROP TABLE IF EXISTS applications;
DROP TABLE IF EXISTS shared_environment_variables;
DROP TABLE IF EXISTS environments;
DROP TABLE IF EXISTS projects;
