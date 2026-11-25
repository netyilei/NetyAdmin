-- +goose Up
-- 查询清单：列表（type+created_at）、actor 溯源（actor+created_at）、retention 清理（created_at）。
CREATE INDEX idx_operation_logs_type_created ON operation_logs (type, created_at);
CREATE INDEX idx_operation_logs_actor_created ON operation_logs (actor_id, created_at);
CREATE INDEX idx_operation_logs_created_at ON operation_logs (created_at);

-- +goose Down
DROP INDEX IF EXISTS idx_operation_logs_type_created;
DROP INDEX IF EXISTS idx_operation_logs_actor_created;
DROP INDEX IF EXISTS idx_operation_logs_created_at;
