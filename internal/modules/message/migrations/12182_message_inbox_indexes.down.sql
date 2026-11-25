-- +goose Down
DROP INDEX IF EXISTS idx_message_inbox_user_created;
DROP INDEX IF EXISTS idx_message_inbox_user_read;
