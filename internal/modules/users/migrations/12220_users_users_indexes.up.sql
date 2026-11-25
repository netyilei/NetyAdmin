-- +goose Up
CREATE UNIQUE INDEX uk_users_phone ON users (phone) WHERE phone <> '';
CREATE INDEX idx_users_status_created ON users (status, created_at DESC);
-- +goose Down
DROP INDEX IF EXISTS idx_users_status_created;
DROP INDEX IF EXISTS idx_users_phone;
