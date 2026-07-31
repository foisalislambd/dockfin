-- +goose Up
ALTER TABLE services
    ADD COLUMN IF NOT EXISTS webhook_secret_enc TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE services
    DROP COLUMN IF EXISTS webhook_secret_enc;
