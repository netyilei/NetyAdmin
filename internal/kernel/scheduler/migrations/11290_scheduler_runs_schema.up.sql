-- +goose Up
-- scheduler_runs：调度运行记录（；架构 §6.4/§23.5）。
-- 先写 running 再执行（§23.5）；终态 succeeded/failed/unknown（崩溃/排空超时）；
-- failed 行=可重放死信（TriggerNow 手动重放），深度经 na_scheduler_dlq_depth。
-- trigger 列名用 trigger_kind（trigger 为 PG 保留字）；status 用 smallint 枚举
-- （0=running 1=succeeded 2=failed 3=unknown，migrations.md §11，Go 侧具名常量）。
CREATE TABLE scheduler_runs (
    id          BIGSERIAL   PRIMARY KEY,
    run_id      TEXT        NOT NULL,        -- 本次运行 ULID
    job         TEXT        NOT NULL,
    node        TEXT        NOT NULL,
    trigger_kind TEXT       NOT NULL,        -- cron|interval|once|delay|manual
    shard       INT         NOT NULL DEFAULT -1, -- -1=非分片
    status      SMALLINT    NOT NULL,        -- 0=running 1=succeeded 2=failed 3=unknown
    attempt     INT         NOT NULL DEFAULT 1,
    error       TEXT        NOT NULL DEFAULT '',
    trace_id    TEXT        NOT NULL DEFAULT '', -- §23.5 列集；调度触发多为空，manual 继承调用方
    started_at  TIMESTAMPTZ NOT NULL,        -- UTC 存储（§23.5）
    finished_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE scheduler_runs;
