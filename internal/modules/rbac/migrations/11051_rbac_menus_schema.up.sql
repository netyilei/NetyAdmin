-- +goose Up
-- rbac_menus：管理端菜单树（前端展示面；判定面在 rbac_apis）。
CREATE TABLE rbac_menus (
    id         text PRIMARY KEY,             -- ULID
    parent_id  text NOT NULL DEFAULT '',     -- 空=根
    name       text NOT NULL,
    path       text NOT NULL DEFAULT '',
    component  text NOT NULL DEFAULT '',     -- 前端视图路径（/system/admins/index—— Geeker 动态路由）
    icon       text NOT NULL DEFAULT '',     -- Element Plus 图标名
    sort_order int NOT NULL DEFAULT 0,
    hidden     bool NOT NULL DEFAULT false,
    status     smallint NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
