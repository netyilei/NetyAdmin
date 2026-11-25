-- +goose Up
-- bus_dlq：死信表（；架构 §5.4）。
-- Cluster 域消息达到 MaxAttempts 后经 XAck 移出 PEL 并入库留存，
-- ReplayDLQ 按行重建信封重发布（修复后运维动作）；深度经 na_bus_dlq_depth 观测。
-- payload/headers 用 TEXT：毒消息原文可能不是合法 JSON，须无损留存。
-- append-only + Replay 后 DELETE：无 updated_at（仅状态变更行需要，migrations.md §11）。
-- 二级索引放 12331（12xxx indexes 段）。
CREATE TABLE bus_dlq (
    id           BIGSERIAL   PRIMARY KEY,
    topic        TEXT        NOT NULL,
    message_id   TEXT        NOT NULL,
    kind         TEXT        NOT NULL DEFAULT '', -- 空=毒消息（原文留存，不可重放）
    schema_ver   INT         NOT NULL DEFAULT 1,
    payload      TEXT        NOT NULL,        -- 原始信封载荷（毒消息存整条原文）
    headers      TEXT        NOT NULL DEFAULT '{}',
    occurred_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    error        TEXT        NOT NULL,        -- 失败原因 / unmarshal 错误
    attempts     INT         NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE bus_dlq;
