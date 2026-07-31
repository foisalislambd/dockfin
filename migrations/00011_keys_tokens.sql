-- +goose Up
CREATE TABLE IF NOT EXISTS cloud_provider_tokens (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id         UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    provider        TEXT NOT NULL CHECK (provider IN ('hetzner', 'digitalocean', 'vultr')),
    name            TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    token_enc       TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (team_id, provider, name)
);

CREATE INDEX IF NOT EXISTS idx_cloud_provider_tokens_team ON cloud_provider_tokens(team_id);

CREATE TABLE IF NOT EXISTS cloud_init_scripts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id         UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    script_enc      TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (team_id, name)
);

CREATE INDEX IF NOT EXISTS idx_cloud_init_scripts_team ON cloud_init_scripts(team_id);

-- +goose Down
DROP TABLE IF EXISTS cloud_init_scripts;
DROP TABLE IF EXISTS cloud_provider_tokens;
