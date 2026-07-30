-- +goose Up
CREATE TABLE private_keys (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id         UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    public_key      TEXT NOT NULL,
    private_key_enc TEXT NOT NULL,
    fingerprint     TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX private_keys_team_id_idx ON private_keys(team_id);

CREATE TABLE servers (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id         UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    private_key_id  UUID REFERENCES private_keys(id) ON DELETE SET NULL,
    name            TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    ip              TEXT NOT NULL,
    port            INTEGER NOT NULL DEFAULT 22,
    user_name       TEXT NOT NULL DEFAULT 'root',
    is_reachable    BOOLEAN NOT NULL DEFAULT FALSE,
    is_usable       BOOLEAN NOT NULL DEFAULT FALSE,
    docker_version  TEXT NOT NULL DEFAULT '',
    proxy_type      TEXT NOT NULL DEFAULT 'traefik' CHECK (proxy_type IN ('traefik', 'caddy', 'none')),
    proxy_status    TEXT NOT NULL DEFAULT 'unknown',
    last_checked_at TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX servers_team_id_idx ON servers(team_id);

CREATE TABLE server_settings (
    server_id                   UUID PRIMARY KEY REFERENCES servers(id) ON DELETE CASCADE,
    deployment_queue_limit      INTEGER NOT NULL DEFAULT 25,
    is_build_server             BOOLEAN NOT NULL DEFAULT FALSE,
    sentinel_enabled            BOOLEAN NOT NULL DEFAULT FALSE,
    sentinel_token              TEXT NOT NULL DEFAULT '',
    docker_cleanup_frequency    TEXT NOT NULL DEFAULT '0 0 * * *',
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE destinations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id         UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    server_id       UUID NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    kind            TEXT NOT NULL DEFAULT 'standalone' CHECK (kind IN ('standalone', 'swarm')),
    network         TEXT NOT NULL DEFAULT 'goolify',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (server_id, network)
);

CREATE INDEX destinations_team_id_idx ON destinations(team_id);
CREATE INDEX destinations_server_id_idx ON destinations(server_id);

-- +goose Down
DROP TABLE IF EXISTS destinations;
DROP TABLE IF EXISTS server_settings;
DROP TABLE IF EXISTS servers;
DROP TABLE IF EXISTS private_keys;
