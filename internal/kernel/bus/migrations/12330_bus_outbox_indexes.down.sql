-- +goose Up
SELECT 1;

-- +goose Down
-- bus_outbox indexes down。
DROP INDEX IF EXISTS idx_bus_outbox_pending;
