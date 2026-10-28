-- +goose Up
ALTER TABLE server_settings
    ADD COLUMN IF NOT EXISTS traffic_analytics_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS traffic_sample_threshold INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS traffic_retention_days INTEGER NOT NULL DEFAULT 7,
    ADD COLUMN IF NOT EXISTS traffic_ip_mode TEXT NOT NULL DEFAULT 'anonymized';

CREATE TABLE IF NOT EXISTS traffic_requests (
    server_id     UUID NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    bucket        TIMESTAMPTZ NOT NULL,
    host          TEXT NOT NULL,
    path          TEXT NOT NULL,
    method        TEXT NOT NULL,
    status_class  TEXT NOT NULL,
    requests      BIGINT NOT NULL,
    PRIMARY KEY (server_id, bucket, host, path, method, status_class)
);

CREATE INDEX IF NOT EXISTS traffic_requests_server_bucket_idx
    ON traffic_requests (server_id, bucket);

CREATE TABLE IF NOT EXISTS traffic_clients (
    server_id UUID NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    bucket    TIMESTAMPTZ NOT NULL,
    host      TEXT NOT NULL,
    client    TEXT NOT NULL,
    requests  BIGINT NOT NULL,
    PRIMARY KEY (server_id, bucket, host, client)
);

CREATE INDEX IF NOT EXISTS traffic_clients_server_bucket_idx
    ON traffic_clients (server_id, bucket);

-- +goose Down
DROP TABLE IF EXISTS traffic_clients;
DROP TABLE IF EXISTS traffic_requests;
ALTER TABLE server_settings DROP COLUMN IF EXISTS traffic_ip_mode;
ALTER TABLE server_settings DROP COLUMN IF EXISTS traffic_retention_days;
ALTER TABLE server_settings DROP COLUMN IF EXISTS traffic_sample_threshold;
ALTER TABLE server_settings DROP COLUMN IF EXISTS traffic_analytics_enabled;
