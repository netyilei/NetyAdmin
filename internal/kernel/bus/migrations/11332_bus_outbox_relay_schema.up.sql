-- +goose Up
-- bus_outbox 行级重试面（多 relay 实例毒行隔离——2026-11-22 multi 验证批）：
--   attempts        连续失败次数（达到 Bus.MaxAttempts → payload 进 bus_dlq、
--                   行置 status=2 dead 出列；成功发布清零）
--   last_error      最近一次失败原因（行级诊断；成功/清理为空）
--   next_attempt_at 行级退避窗口（NULL=立即可 claim；失败行按指数退避推迟——
--                   单行失败不阻塞同批其他行）
-- status：0=pending 1=published 2=dead（Go 侧具名常量参数化，禁裸数字）。
-- claim 维持 FOR UPDATE SKIP LOCKED + 上述谓词：多 relay 实例并行 claim，
-- 行锁粒度无全局锁（分区 claim 语义）。
ALTER TABLE bus_outbox
    ADD COLUMN attempts        INT         NOT NULL DEFAULT 0,
    ADD COLUMN last_error      TEXT        NOT NULL DEFAULT '',
    ADD COLUMN next_attempt_at TIMESTAMPTZ;

-- +goose Down
ALTER TABLE bus_outbox
    DROP COLUMN IF EXISTS next_attempt_at,
    DROP COLUMN IF EXISTS last_error,
    DROP COLUMN IF EXISTS attempts;
