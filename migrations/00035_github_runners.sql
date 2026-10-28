-- +goose Up
ALTER TABLE git_sources
    ADD COLUMN IF NOT EXISTS runner_group_id BIGINT;

CREATE TABLE IF NOT EXISTS github_runner_configs (
    id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id                UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    server_id              UUID NOT NULL UNIQUE REFERENCES servers(id) ON DELETE CASCADE,
    git_source_id          UUID NOT NULL REFERENCES git_sources(id) ON DELETE CASCADE,
    labels                 JSONB NOT NULL DEFAULT '[]'::jsonb,
    enabled                BOOLEAN NOT NULL DEFAULT FALSE,
    allow_pull_requests    BOOLEAN NOT NULL DEFAULT FALSE,
    max_runners            INTEGER NOT NULL DEFAULT 2,
    docker_mode            TEXT NOT NULL DEFAULT 'none',
    runner_image           TEXT NOT NULL DEFAULT '',
    cpu_limit              TEXT NOT NULL DEFAULT '',
    memory_limit           TEXT NOT NULL DEFAULT '',
    queue_wait_minutes     INTEGER NOT NULL DEFAULT 60,
    idle_timeout_minutes   INTEGER NOT NULL DEFAULT 10,
    job_timeout_minutes    INTEGER NOT NULL DEFAULT 360,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT github_runner_configs_mode_chk CHECK (docker_mode IN ('none', 'dind')),
    CONSTRAINT github_runner_configs_max_chk CHECK (max_runners BETWEEN 1 AND 32),
    CONSTRAINT github_runner_configs_queue_chk CHECK (queue_wait_minutes BETWEEN 1 AND 1440),
    CONSTRAINT github_runner_configs_idle_chk CHECK (idle_timeout_minutes BETWEEN 1 AND 1440),
    CONSTRAINT github_runner_configs_job_chk CHECK (job_timeout_minutes BETWEEN 1 AND 7200)
);

CREATE INDEX IF NOT EXISTS github_runner_configs_source_idx
    ON github_runner_configs (git_source_id);

CREATE TABLE IF NOT EXISTS github_runner_executions (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id             UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    git_source_id       UUID NOT NULL REFERENCES git_sources(id) ON DELETE CASCADE,
    config_id           UUID REFERENCES github_runner_configs(id) ON DELETE SET NULL,
    server_id           UUID REFERENCES servers(id) ON DELETE SET NULL,
    status              TEXT NOT NULL DEFAULT 'queued',
    trigger_job_id      BIGINT NOT NULL,
    workflow_job_id     BIGINT,
    html_url            TEXT NOT NULL DEFAULT '',
    workflow_name       TEXT NOT NULL DEFAULT '',
    job_name            TEXT NOT NULL DEFAULT '',
    repository          TEXT NOT NULL DEFAULT '',
    labels              JSONB NOT NULL DEFAULT '[]'::jsonb,
    is_pull_request     BOOLEAN NOT NULL DEFAULT FALSE,
    runner_name         TEXT NOT NULL DEFAULT '',
    runner_id           BIGINT,
    conclusion          TEXT NOT NULL DEFAULT '',
    error_message       TEXT NOT NULL DEFAULT '',
    provision_attempts  INTEGER NOT NULL DEFAULT 0,
    queue_wait_minutes  INTEGER NOT NULL DEFAULT 60,
    queued_at           TIMESTAMPTZ,
    ready_at            TIMESTAMPTZ,
    started_at          TIMESTAMPTZ,
    completed_at        TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (git_source_id, trigger_job_id)
);

CREATE INDEX IF NOT EXISTS github_runner_executions_status_idx
    ON github_runner_executions (status, updated_at);
CREATE INDEX IF NOT EXISTS github_runner_executions_server_idx
    ON github_runner_executions (server_id, created_at DESC);
CREATE INDEX IF NOT EXISTS github_runner_executions_name_idx
    ON github_runner_executions (git_source_id, runner_name);

-- +goose Down
DROP TABLE IF EXISTS github_runner_executions;
DROP TABLE IF EXISTS github_runner_configs;
ALTER TABLE git_sources DROP COLUMN IF EXISTS runner_group_id;
