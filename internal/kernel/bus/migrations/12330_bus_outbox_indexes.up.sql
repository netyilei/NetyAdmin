-- +goose Up
-- bus_outbox 二级索引（11330 配对；migrations.md §11：索引放 12xxx）。
CREATE INDEX idx_bus_outbox_pending ON bus_outbox (id) WHERE status = 0;

-- +goose Down
DROP INDEX idx_bus_outbox_pending;
