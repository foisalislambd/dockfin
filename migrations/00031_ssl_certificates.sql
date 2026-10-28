-- +goose Up
ALTER TABLE databases
    ADD COLUMN IF NOT EXISTS enable_ssl BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE IF NOT EXISTS ssl_certificates (
    id                          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id                     UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    server_id                   UUID NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    resource_type               TEXT NOT NULL DEFAULT '',
    resource_id                 UUID,
    cert_enc                    TEXT NOT NULL,
    key_enc                     TEXT NOT NULL,
    common_name                 TEXT NOT NULL,
    subject_alternative_names   JSONB NOT NULL DEFAULT '[]'::jsonb,
    valid_until                 TIMESTAMPTZ NOT NULL,
    is_ca                       BOOLEAN NOT NULL DEFAULT FALSE,
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS ssl_certificates_one_ca
    ON ssl_certificates (server_id) WHERE is_ca;

CREATE UNIQUE INDEX IF NOT EXISTS ssl_certificates_resource
    ON ssl_certificates (resource_type, resource_id)
    WHERE NOT is_ca AND resource_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_ssl_certificates_team ON ssl_certificates (team_id);

-- +goose Down
DROP TABLE IF EXISTS ssl_certificates;
ALTER TABLE databases DROP COLUMN IF EXISTS enable_ssl;
