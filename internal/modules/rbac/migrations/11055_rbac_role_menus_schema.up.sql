-- +goose Up
-- rbac_role_menus：角色↔菜单授权（前端树渲染口径）。
CREATE TABLE rbac_role_menus (
    role_id text NOT NULL,
    menu_id text NOT NULL,
    CONSTRAINT pk_rbac_role_menus PRIMARY KEY (role_id, menu_id)
);
