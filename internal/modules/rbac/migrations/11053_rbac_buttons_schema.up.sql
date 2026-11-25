-- +goose Up
-- rbac_buttons：按钮权限码（前端动作显隐；判定面仍走对应 API 行）。
CREATE TABLE rbac_buttons (
    id         text PRIMARY KEY,             -- ULID
    menu_id    text NOT NULL DEFAULT '',     -- 归属菜单
    code       text NOT NULL,                -- 权限码（如 admin.users.delete）
    name       text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
