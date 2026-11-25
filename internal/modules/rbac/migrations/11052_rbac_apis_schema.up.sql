-- +goose Up
-- rbac_apis：API 判定登记（Casbin obj=path act=权限码；通配行=超管语义）。
CREATE TABLE rbac_apis (
    id         text PRIMARY KEY,             -- ULID
    method     text NOT NULL DEFAULT '*',    -- * = 全方法（通配策略行）
    path       text NOT NULL,                -- Casbin keyMatch2 模式（如 /admin/v1/*）
    act        text NOT NULL,                -- 权限码（如 admin.users.list；* = 全动作）
    summary    text NOT NULL DEFAULT '',
    status     smallint NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
