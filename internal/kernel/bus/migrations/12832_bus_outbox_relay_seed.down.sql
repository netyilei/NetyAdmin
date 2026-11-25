-- +goose Up
SELECT 1;

-- +goose Down
-- bus_outbox relay seed down（无种子数据）。
SELECT 1;
