-- +goose Up
-- bus_outbox：Outbox 事件表（；架构 §5.4/§23.10）。
-- PublishToOutbox 与业务写同事务 INSERT；Relay 以 FOR UPDATE SKIP LOCKED
-- 批量 claim、顺序发布（status=0 → 1），完成后按保留窗口清理。
-- 二级索引放 12330（12xxx indexes 段，migrations.md §11）。
CREATE TABLE bus_outbox (
    id           BIGSERIAL    PRIMARY KEY,      -- 单调序：claim ORDER BY id 保发布顺序
    message_id   TEXT         NOT NULL,         -- ULID，消费方幂等依据
    topic        TEXT         NOT NULL,
    kind         TEXT         NOT NULL,
    schema_ver   INT          NOT NULL DEFAULT 1,
    aggregate_id TEXT         NOT NULL DEFAULT '', -- 按聚合保序（同聚合事件顺序发布）
    payload      TEXT         NOT NULL,            -- 信封 Payload 原文（JSON）
    headers      TEXT         NOT NULL DEFAULT '{}',
    occurred_at  TIMESTAMPTZ  NOT NULL,
    status       SMALLINT     NOT NULL DEFAULT 0,  -- 0=pending 1=published（Go 侧具名常量参数化）
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ
);

-- +goose Down
DROP TABLE bus_outbox;
