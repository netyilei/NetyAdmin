-- +goose Up
-- bus_dlq 二级索引（11331 配对；migrations.md §11：索引放 12xxx）。
CREATE INDEX idx_bus_dlq_topic ON bus_dlq (topic, id);

-- +goose Down
DROP INDEX idx_bus_dlq_topic;
