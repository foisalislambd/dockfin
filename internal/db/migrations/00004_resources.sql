-- +goose Up
CREATE TABLE databases (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id             UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    environment_id      UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    destination_id      UUID REFERENCES destinations(id) ON DELETE SET NULL,
    name                TEXT NOT NULL,
    description         TEXT NOT NULL DEFAULT '',
    engine              TEXT NOT NULL CHECK (engine IN (
                            'postgresql', 'mysql', 'mariadb', 'mongodb',
                            'redis', 'keydb', 'dragonfly', 'clickhouse'
                        )),
    image               TEXT NOT NULL,
    status              TEXT NOT NULL DEFAULT 'exited',
    is_public           BOOLEAN NOT NULL DEFAULT FALSE,
    public_port         INTEGER,
    engine_config       JSONB NOT NULL DEFAULT '{}'::jsonb,
    credentials_enc     TEXT NOT NULL DEFAULT '',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX databases_team_id_idx ON databases(team_id);
CREATE INDEX databases_environment_id_idx ON databases(environment_id);

CREATE TABLE services (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id             UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    environment_id      UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    server_id           UUID REFERENCES servers(id) ON DELETE SET NULL,
    destination_id      UUID REFERENCES destinations(id) ON DELETE SET NULL,
    name                TEXT NOT NULL,
    description         TEXT NOT NULL DEFAULT '',
    service_type        TEXT NOT NULL DEFAULT 'custom',
    docker_compose_raw  TEXT NOT NULL DEFAULT '',
    docker_compose      TEXT NOT NULL DEFAULT '',
    status              TEXT NOT NULL DEFAULT 'exited',
    config_hash         TEXT NOT NULL DEFAULT '',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at          TIMESTAMPTZ
);

CREATE INDEX services_team_id_idx ON services(team_id);
CREATE INDEX services_environment_id_idx ON services(environment_id);

CREATE TABLE service_components (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id             UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    service_id          UUID NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    name                TEXT NOT NULL,
    kind                TEXT NOT NULL CHECK (kind IN ('application', 'database')),
    image               TEXT NOT NULL DEFAULT '',
    fqdn                TEXT NOT NULL DEFAULT '',
    status              TEXT NOT NULL DEFAULT 'exited',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX service_components_service_id_idx ON service_components(service_id);

CREATE TABLE volumes (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id             UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    resource_type       TEXT NOT NULL,
    resource_id         UUID NOT NULL,
    name                TEXT NOT NULL,
    mount_path          TEXT NOT NULL,
    host_path           TEXT NOT NULL DEFAULT '',
    is_file             BOOLEAN NOT NULL DEFAULT FALSE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX volumes_resource_idx ON volumes(resource_type, resource_id);

CREATE TABLE s3_storages (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id             UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    name                TEXT NOT NULL,
    endpoint            TEXT NOT NULL,
    bucket              TEXT NOT NULL,
    region              TEXT NOT NULL DEFAULT 'us-east-1',
    access_key_enc      TEXT NOT NULL,
    secret_key_enc      TEXT NOT NULL,
    path_style          BOOLEAN NOT NULL DEFAULT TRUE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE scheduled_backups (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id             UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    resource_type       TEXT NOT NULL,
    resource_id         UUID NOT NULL,
    s3_storage_id       UUID REFERENCES s3_storages(id) ON DELETE SET NULL,
    frequency           TEXT NOT NULL DEFAULT '0 0 * * *',
    enabled             BOOLEAN NOT NULL DEFAULT TRUE,
    retention           INTEGER NOT NULL DEFAULT 7,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE backup_executions (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id             UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    scheduled_backup_id UUID REFERENCES scheduled_backups(id) ON DELETE SET NULL,
    resource_type       TEXT NOT NULL,
    resource_id         UUID NOT NULL,
    status              TEXT NOT NULL DEFAULT 'running',
    size_bytes          BIGINT NOT NULL DEFAULT 0,
    filename            TEXT NOT NULL DEFAULT '',
    error_message       TEXT NOT NULL DEFAULT '',
    started_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at         TIMESTAMPTZ
);

CREATE TABLE scheduled_tasks (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id             UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    resource_type       TEXT NOT NULL,
    resource_id         UUID NOT NULL,
    name                TEXT NOT NULL,
    command             TEXT NOT NULL,
    frequency           TEXT NOT NULL,
    container_name      TEXT NOT NULL DEFAULT '',
    enabled             BOOLEAN NOT NULL DEFAULT TRUE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE scheduled_task_executions (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id             UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    scheduled_task_id   UUID NOT NULL REFERENCES scheduled_tasks(id) ON DELETE CASCADE,
    status              TEXT NOT NULL DEFAULT 'running',
    output              TEXT NOT NULL DEFAULT '',
    started_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at         TIMESTAMPTZ
);

CREATE TABLE git_sources (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id             UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    provider            TEXT NOT NULL CHECK (provider IN ('github', 'gitlab', 'bitbucket', 'gitea')),
    name                TEXT NOT NULL,
    app_id              TEXT NOT NULL DEFAULT '',
    installation_id     TEXT NOT NULL DEFAULT '',
    client_id           TEXT NOT NULL DEFAULT '',
    client_secret_enc   TEXT NOT NULL DEFAULT '',
    webhook_secret_enc  TEXT NOT NULL DEFAULT '',
    private_key_enc     TEXT NOT NULL DEFAULT '',
    html_url            TEXT NOT NULL DEFAULT '',
    api_url             TEXT NOT NULL DEFAULT '',
    is_public           BOOLEAN NOT NULL DEFAULT FALSE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE notification_settings (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id             UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    channel             TEXT NOT NULL CHECK (channel IN ('email', 'discord', 'slack', 'telegram', 'webhook')),
    enabled             BOOLEAN NOT NULL DEFAULT FALSE,
    config_enc          TEXT NOT NULL DEFAULT '',
    events              TEXT[] NOT NULL DEFAULT ARRAY['deployment_success', 'deployment_failed'],
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (team_id, channel)
);

CREATE TABLE server_metrics (
    id                  BIGSERIAL PRIMARY KEY,
    team_id             UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    server_id           UUID NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    cpu_percent         DOUBLE PRECISION NOT NULL DEFAULT 0,
    memory_used_bytes   BIGINT NOT NULL DEFAULT 0,
    memory_total_bytes  BIGINT NOT NULL DEFAULT 0,
    disk_used_bytes     BIGINT NOT NULL DEFAULT 0,
    disk_total_bytes    BIGINT NOT NULL DEFAULT 0,
    recorded_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX server_metrics_server_recorded_idx ON server_metrics(server_id, recorded_at DESC);

CREATE TABLE tags (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id             UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    name                TEXT NOT NULL,
    color               TEXT NOT NULL DEFAULT '#14b8a6',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (team_id, name)
);

CREATE TABLE taggables (
    tag_id              UUID NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    resource_type       TEXT NOT NULL,
    resource_id         UUID NOT NULL,
    PRIMARY KEY (tag_id, resource_type, resource_id)
);

CREATE TABLE secret_audit_logs (
    id                  BIGSERIAL PRIMARY KEY,
    team_id             UUID NOT NULL,
    user_id             UUID,
    action              TEXT NOT NULL,
    resource_type       TEXT NOT NULL,
    resource_id         UUID,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS secret_audit_logs;
DROP TABLE IF EXISTS taggables;
DROP TABLE IF EXISTS tags;
DROP TABLE IF EXISTS server_metrics;
DROP TABLE IF EXISTS notification_settings;
DROP TABLE IF EXISTS git_sources;
DROP TABLE IF EXISTS scheduled_task_executions;
DROP TABLE IF EXISTS scheduled_tasks;
DROP TABLE IF EXISTS backup_executions;
DROP TABLE IF EXISTS scheduled_backups;
DROP TABLE IF EXISTS s3_storages;
DROP TABLE IF EXISTS volumes;
DROP TABLE IF EXISTS service_components;
DROP TABLE IF EXISTS services;
DROP TABLE IF EXISTS databases;
