-- +goose Down
DROP INDEX IF EXISTS idx_message_records_template;
DROP INDEX IF EXISTS idx_message_records_created;
DROP INDEX IF EXISTS idx_message_records_channel_status;
