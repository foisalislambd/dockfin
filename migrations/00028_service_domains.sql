-- +goose Up
-- Per-compose-service public domains (Coolify service domains page).
CREATE TABLE IF NOT EXISTS service_domains (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id        UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    service_id     UUID NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    unit_name      TEXT NOT NULL DEFAULT '',
    host           TEXT NOT NULL,
    scheme         TEXT NOT NULL DEFAULT 'https' CHECK (scheme IN ('http', 'https')),
    internal_port  INTEGER,
    force_https    BOOLEAN NOT NULL DEFAULT TRUE,
    redirect       TEXT NOT NULL DEFAULT 'both' CHECK (redirect IN ('both', 'www', 'non-www')),
    noindex        BOOLEAN NOT NULL DEFAULT FALSE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS service_domains_service_host_idx ON service_domains(service_id, host);
CREATE INDEX IF NOT EXISTS service_domains_service_id_idx ON service_domains(service_id);

-- +goose Down
DROP TABLE IF EXISTS service_domains;
