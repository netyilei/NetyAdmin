-- +goose Up
-- bus_outbox relay seed（11332 配对；行级重试为运行时设施，无种子数据）。

-- +goose Down
SELECT 1;
