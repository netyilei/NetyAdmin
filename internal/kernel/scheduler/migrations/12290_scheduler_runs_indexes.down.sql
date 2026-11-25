-- +goose Up
SELECT 1;

-- +goose Down
-- scheduler_runs indexes down。
DROP INDEX IF EXISTS idx_scheduler_runs_status;
DROP INDEX IF EXISTS idx_scheduler_runs_job;
