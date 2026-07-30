-- +goose Up
ALTER TABLE servers
    ADD COLUMN IF NOT EXISTS host_key_fingerprint TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS host_key_type TEXT NOT NULL DEFAULT '';

ALTER TABLE applications
    ADD COLUMN IF NOT EXISTS webhook_secret_enc TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS watch_paths TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS is_force_https BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS http_basic_auth_username TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS http_basic_auth_password_enc TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS additional_destinations (
    application_id UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    destination_id UUID NOT NULL REFERENCES destinations(id) ON DELETE CASCADE,
    PRIMARY KEY (application_id, destination_id)
);

-- +goose Down
DROP TABLE IF EXISTS additional_destinations;
ALTER TABLE applications
    DROP COLUMN IF EXISTS webhook_secret_enc,
    DROP COLUMN IF EXISTS watch_paths,
    DROP COLUMN IF EXISTS is_force_https,
    DROP COLUMN IF EXISTS http_basic_auth_username,
    DROP COLUMN IF EXISTS http_basic_auth_password_enc;
ALTER TABLE servers
    DROP COLUMN IF EXISTS host_key_fingerprint,
    DROP COLUMN IF EXISTS host_key_type;
