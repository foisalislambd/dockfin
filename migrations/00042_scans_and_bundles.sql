-- +goose Up
ALTER TABLE instance_settings
    ADD COLUMN IF NOT EXISTS image_scan_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS image_scan_gate TEXT NOT NULL DEFAULT 'off';

ALTER TABLE instance_settings
    DROP CONSTRAINT IF EXISTS instance_settings_image_scan_gate_check;

ALTER TABLE instance_settings
    ADD CONSTRAINT instance_settings_image_scan_gate_check
    CHECK (image_scan_gate IN ('off', 'warn', 'block'));

CREATE TABLE IF NOT EXISTS image_scans (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id         UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    resource_type   TEXT NOT NULL,
    resource_id     UUID NOT NULL,
    deployment_id   UUID,
    server_id       UUID,
    image_ref       TEXT NOT NULL,
    status          TEXT NOT NULL,
    critical        INT NOT NULL DEFAULT 0,
    high            INT NOT NULL DEFAULT 0,
    medium          INT NOT NULL DEFAULT 0,
    low             INT NOT NULL DEFAULT 0,
    unknown         INT NOT NULL DEFAULT 0,
    findings        JSONB NOT NULL DEFAULT '[]',
    error           TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at     TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS image_scans_team_created_idx
    ON image_scans (team_id, created_at DESC);

CREATE INDEX IF NOT EXISTS image_scans_resource_idx
    ON image_scans (team_id, resource_type, resource_id, created_at DESC);

CREATE TABLE IF NOT EXISTS image_scan_ignores (
    team_id     UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    vuln_id     TEXT NOT NULL,
    reason      TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (team_id, vuln_id)
);

-- +goose Down
DROP TABLE IF EXISTS image_scan_ignores;
DROP TABLE IF EXISTS image_scans;
ALTER TABLE instance_settings DROP CONSTRAINT IF EXISTS instance_settings_image_scan_gate_check;
ALTER TABLE instance_settings
    DROP COLUMN IF EXISTS image_scan_gate,
    DROP COLUMN IF EXISTS image_scan_enabled;
