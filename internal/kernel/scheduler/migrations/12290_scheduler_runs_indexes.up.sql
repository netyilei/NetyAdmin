-- +goose Up
-- scheduler_runs 二级索引（11290 配对；migrations.md §11：索引放 12xxx）。
-- 管理面查询模式：按 job 倒序取最近记录；运行中/失败穷实行过滤。
-- status 枚举：0=running 1=succeeded 2=failed 3=unknown。
CREATE INDEX idx_scheduler_runs_job ON scheduler_runs (job, id DESC);
CREATE INDEX idx_scheduler_runs_status ON scheduler_runs (status) WHERE status IN (0, 2);

-- +goose Down
DROP INDEX idx_scheduler_runs_status;
DROP INDEX idx_scheduler_runs_job;
