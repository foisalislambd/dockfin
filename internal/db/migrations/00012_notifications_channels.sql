-- +goose Up
ALTER TABLE notification_settings DROP CONSTRAINT IF EXISTS notification_settings_channel_check;
ALTER TABLE notification_settings
    ADD CONSTRAINT notification_settings_channel_check
    CHECK (channel IN ('email', 'discord', 'telegram', 'slack', 'pushover', 'webhook'));

-- +goose Down
ALTER TABLE notification_settings DROP CONSTRAINT IF EXISTS notification_settings_channel_check;
ALTER TABLE notification_settings
    ADD CONSTRAINT notification_settings_channel_check
    CHECK (channel IN ('email', 'discord', 'slack', 'telegram', 'webhook'));
