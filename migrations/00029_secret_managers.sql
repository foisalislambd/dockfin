-- +goose Up
CREATE TABLE IF NOT EXISTS integration_tokens (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id     UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    provider    TEXT NOT NULL CHECK (provider IN ('doppler', 'infisical', 'vault')),
    name        TEXT NOT NULL,
    token_enc   TEXT NOT NULL,
    metadata    JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (team_id, provider, name)
);

CREATE INDEX IF NOT EXISTS idx_integration_tokens_team ON integration_tokens(team_id);

CREATE TABLE IF NOT EXISTS secret_manager_links (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id               UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    resource_type         TEXT NOT NULL CHECK (resource_type IN ('application', 'database', 'service')),
    resource_id           UUID NOT NULL,
    integration_token_id  UUID NOT NULL REFERENCES integration_tokens(id) ON DELETE CASCADE,
    settings              JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (resource_type, resource_id)
);

CREATE INDEX IF NOT EXISTS idx_secret_manager_links_team ON secret_manager_links(team_id);
CREATE INDEX IF NOT EXISTS idx_secret_manager_links_token ON secret_manager_links(integration_token_id);

-- +goose Down
DROP TABLE IF EXISTS secret_manager_links;
DROP TABLE IF EXISTS integration_tokens;
