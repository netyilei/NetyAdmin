-- +goose Up
SELECT 1;

-- +goose Down
-- bus_outbox down：删除 Outbox 事件表。
DROP TABLE bus_outbox;
