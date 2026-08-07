-- +goose Up
ALTER TABLE applications
    ADD COLUMN IF NOT EXISTS dockerfile TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE applications
    DROP COLUMN IF EXISTS dockerfile;
