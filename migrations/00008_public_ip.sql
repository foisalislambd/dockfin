-- +goose Up
ALTER TABLE servers
    ADD COLUMN IF NOT EXISTS public_ip TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE servers DROP COLUMN IF EXISTS public_ip;
