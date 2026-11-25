-- +goose Up
-- config_history：配置变更历史（回滚面；AGENTS §4「版本+校验+回滚」）。
-- 语义：每次变更（create/update/delete/rollback）事务内落一条「变更前整表快照」
-- + 全局递增 version；回滚 = 取历史快照整表写回并生成新版本（历史只增不改——
-- 回滚本身也是变更，可再次回滚）。管理端点随消费者登记（当前仅机制面）。
CREATE TABLE config_history (
    id         text PRIMARY KEY,            -- ULID
    version    bigint NOT NULL,             -- 全局递增（事务内 max+1）
    snapshot   jsonb NOT NULL,              -- 变更前整表快照（[{group,key,value,...}]）
    reason     text NOT NULL DEFAULT '',    -- 变更说明（操作者输入/rollback 标记）
    created_by text NOT NULL DEFAULT 'system',
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uk_config_history_version UNIQUE (version)
);
-- +goose Down
DROP TABLE IF EXISTS config_history;
