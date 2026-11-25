-- +goose Up
SELECT 1;

-- +goose Down
-- scheduler_runs down：删除运行记录表。
DROP TABLE scheduler_runs;
