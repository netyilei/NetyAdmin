-- +goose Down
DROP INDEX IF EXISTS uk_users_phone;
DROP INDEX IF EXISTS idx_users_status_created;
DROP INDEX IF EXISTS idx_users_phone;
