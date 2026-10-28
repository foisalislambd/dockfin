-- +goose Up
CREATE TABLE IF NOT EXISTS dns_zones (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id           UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    cloud_token_id    UUID NOT NULL REFERENCES cloud_provider_tokens(id) ON DELETE CASCADE,
    provider_zone_id  TEXT NOT NULL,
    name              TEXT NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (cloud_token_id, provider_zone_id)
);

CREATE INDEX IF NOT EXISTS idx_dns_zones_team ON dns_zones(team_id);

CREATE TABLE IF NOT EXISTS managed_dns_records (
    id                  UUID PRIMARY KEY,
    team_id             UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    zone_id             UUID NOT NULL REFERENCES dns_zones(id) ON DELETE CASCADE,
    cloud_token_id      UUID NOT NULL REFERENCES cloud_provider_tokens(id) ON DELETE CASCADE,
    provider_record_id  TEXT NOT NULL DEFAULT '',
    type                TEXT NOT NULL,
    name                TEXT NOT NULL,
    content             TEXT NOT NULL,
    owned               BOOLEAN NOT NULL DEFAULT FALSE,
    comment             TEXT NOT NULL DEFAULT '',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (team_id, name)
);

CREATE INDEX IF NOT EXISTS idx_managed_dns_records_zone ON managed_dns_records(zone_id);

CREATE TABLE IF NOT EXISTS managed_dns_refs (
    record_id      UUID NOT NULL REFERENCES managed_dns_records(id) ON DELETE CASCADE,
    resource_type  TEXT NOT NULL CHECK (resource_type IN ('application', 'service', 'application_preview')),
    resource_id    UUID NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (record_id, resource_type, resource_id)
);

CREATE INDEX IF NOT EXISTS idx_managed_dns_refs_resource ON managed_dns_refs(resource_type, resource_id);

-- +goose Down
DROP TABLE IF EXISTS managed_dns_refs;
DROP TABLE IF EXISTS managed_dns_records;
DROP TABLE IF EXISTS dns_zones;
