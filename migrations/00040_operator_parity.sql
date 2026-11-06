-- +goose Up
-- Operator parity: paused API tokens, per-resource maintenance and permanent
-- www redirects, server health snapshots, and proxy catch-all settings.

ALTER TABLE api_tokens
    ADD COLUMN IF NOT EXISTS paused_at TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS resource_controls (
    team_id              UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    resource_type        TEXT NOT NULL CHECK (resource_type IN ('application', 'service')),
    resource_id          UUID NOT NULL,
    maintenance_enabled  BOOLEAN NOT NULL DEFAULT FALSE,
    maintenance_message  TEXT NOT NULL DEFAULT '',
    redirect_permanent   BOOLEAN NOT NULL DEFAULT FALSE,
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (resource_type, resource_id)
);

CREATE INDEX IF NOT EXISTS resource_controls_team_idx ON resource_controls(team_id);

ALTER TABLE service_domains
    ADD COLUMN IF NOT EXISTS redirect_permanent BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE server_settings
    ADD COLUMN IF NOT EXISTS disk_usage_notification_threshold INTEGER NOT NULL DEFAULT 80,
    ADD COLUMN IF NOT EXISTS proxy_catchall_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS proxy_catchall_url TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS proxy_catchall_permanent BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE IF NOT EXISTS server_health (
    server_id              UUID PRIMARY KEY REFERENCES servers(id) ON DELETE CASCADE,
    team_id                UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    reachable              BOOLEAN NOT NULL DEFAULT FALSE,
    consecutive_failures   INTEGER NOT NULL DEFAULT 0,
    disk_used_percent      INTEGER,
    disk_mount             TEXT NOT NULL DEFAULT '/',
    traefik_image          TEXT NOT NULL DEFAULT '',
    traefik_outdated       BOOLEAN NOT NULL DEFAULT FALSE,
    patch_count            INTEGER NOT NULL DEFAULT 0,
    last_checked_at        TIMESTAMPTZ,
    last_reachable_at      TIMESTAMPTZ,
    last_patch_check_at    TIMESTAMPTZ,
    last_error             TEXT NOT NULL DEFAULT '',
    unreachable_notified   BOOLEAN NOT NULL DEFAULT FALSE,
    disk_notified          BOOLEAN NOT NULL DEFAULT FALSE,
    traefik_notified_image TEXT NOT NULL DEFAULT '',
    patch_notified_count   INTEGER NOT NULL DEFAULT -1
);

CREATE INDEX IF NOT EXISTS server_health_team_idx ON server_health(team_id);

-- +goose Down
DROP TABLE IF EXISTS server_health;

ALTER TABLE server_settings
    DROP COLUMN IF EXISTS disk_usage_notification_threshold,
    DROP COLUMN IF EXISTS proxy_catchall_enabled,
    DROP COLUMN IF EXISTS proxy_catchall_url,
    DROP COLUMN IF EXISTS proxy_catchall_permanent;

ALTER TABLE service_domains DROP COLUMN IF EXISTS redirect_permanent;

DROP TABLE IF EXISTS resource_controls;

ALTER TABLE api_tokens DROP COLUMN IF EXISTS paused_at;
