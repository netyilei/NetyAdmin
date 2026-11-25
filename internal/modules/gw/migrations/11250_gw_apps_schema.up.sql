-- +goose Up
-- gw_apps：开放平台应用（；V1 sys_apps 对齐——secret AES-GCM 加密落库、
-- quota jsonb 桶配置、IP 组集引用（ADR 0035——组集空=不限，无独立开关/策略列）、
-- storage 绑定文本引用）。
CREATE TABLE gw_apps (
    id                text PRIMARY KEY,              -- ULID
    app_key           text NOT NULL,                 -- 对外 AppKey（26 位 ULID 同源）
    app_secret_enc    text NOT NULL,                 -- AES-256-GCM（primitives；key=Secret 注入）
    name              text NOT NULL,
    status            smallint NOT NULL DEFAULT 1,   -- 1=enabled 2=disabled
    rate_limit_enabled bool NOT NULL DEFAULT true,
    quota_config      jsonb NOT NULL DEFAULT '{}',   -- {"rate":N}（滑窗；rate<=0=不限流）
    remark            text NOT NULL DEFAULT '',
    storage_ref       text NOT NULL DEFAULT '',      -- 绑定存储配置 ID（文本引用——ADR 0027）
    ip_group_ids      text[] NOT NULL DEFAULT '{}',  -- 启用的 IP 策略组集（ADR 0035——空=不限 IP）
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uk_gw_apps_key UNIQUE (app_key)
);
-- +goose Down
DROP TABLE IF EXISTS gw_apps;
