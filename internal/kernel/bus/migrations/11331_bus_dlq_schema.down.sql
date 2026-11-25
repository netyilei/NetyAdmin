-- +goose Up
SELECT 1;

-- +goose Down
-- bus_dlq down：删除死信表。
DROP TABLE bus_dlq;
