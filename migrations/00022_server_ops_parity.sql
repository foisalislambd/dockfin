-- +goose Up
-- Server ops parity: sentinel/cleanup settings, cleanup history, proxy dynamic configs, edge fields.

ALTER TABLE server_settings
    ADD COLUMN IF NOT EXISTS docker_cleanup_threshold INTEGER NOT NULL DEFAULT 80,
    ADD COLUMN IF NOT EXISTS force_docker_cleanup BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS sentinel_metrics_refresh_rate_seconds INTEGER NOT NULL DEFAULT 30,
    ADD COLUMN IF NOT EXISTS cloudflare_tunnel_token TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS cloudflare_tunnel_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS log_drain_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS log_drain_type TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS log_drain_config TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS ca_certificate TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS terminal_acl_user_ids JSONB NOT NULL DEFAULT '[]'::jsonb;

CREATE TABLE IF NOT EXISTS docker_cleanup_executions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id         UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    server_id       UUID NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    status          TEXT NOT NULL DEFAULT 'running'
                    CHECK (status IN ('running', 'finished', 'failed')),
    message         TEXT NOT NULL DEFAULT '',
    started_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS docker_cleanup_executions_server_idx
    ON docker_cleanup_executions(server_id, started_at DESC);

CREATE TABLE IF NOT EXISTS server_proxy_configurations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id         UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    server_id       UUID NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    value           TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (server_id, name)
);

CREATE INDEX IF NOT EXISTS server_proxy_configurations_server_idx
    ON server_proxy_configurations(server_id);

-- +goose Down
DROP TABLE IF EXISTS server_proxy_configurations;
DROP TABLE IF EXISTS docker_cleanup_executions;

ALTER TABLE server_settings
    DROP COLUMN IF EXISTS docker_cleanup_threshold,
    DROP COLUMN IF EXISTS force_docker_cleanup,
    DROP COLUMN IF EXISTS sentinel_metrics_refresh_rate_seconds,
    DROP COLUMN IF EXISTS cloudflare_tunnel_token,
    DROP COLUMN IF EXISTS cloudflare_tunnel_enabled,
    DROP COLUMN IF EXISTS log_drain_enabled,
    DROP COLUMN IF EXISTS log_drain_type,
    DROP COLUMN IF EXISTS log_drain_config,
    DROP COLUMN IF EXISTS ca_certificate,
    DROP COLUMN IF EXISTS terminal_acl_user_ids;
