-- +goose Down
DROP INDEX IF EXISTS idx_operation_logs_type_created;
DROP INDEX IF EXISTS idx_operation_logs_actor_created;
DROP INDEX IF EXISTS idx_operation_logs_created_at;
