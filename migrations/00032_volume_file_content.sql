-- +goose Up
ALTER TABLE volumes
    ADD COLUMN IF NOT EXISTS content_enc TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE volumes DROP COLUMN IF EXISTS content_enc;
