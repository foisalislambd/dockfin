-- +goose Up
ALTER TABLE image_scans
    ADD COLUMN IF NOT EXISTS candidates JSONB NOT NULL DEFAULT '{}'::jsonb;

-- +goose Down
ALTER TABLE image_scans DROP COLUMN IF EXISTS candidates;
