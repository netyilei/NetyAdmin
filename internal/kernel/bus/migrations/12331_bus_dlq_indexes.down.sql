-- +goose Up
SELECT 1;

-- +goose Down
-- bus_dlq indexes down。
DROP INDEX IF EXISTS idx_bus_dlq_topic;
