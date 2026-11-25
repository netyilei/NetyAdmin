-- +goose Up
-- rbac_role_buttons：角色↔按钮授权。
CREATE TABLE rbac_role_buttons (
    role_id   text NOT NULL,
    button_id text NOT NULL,
    CONSTRAINT pk_rbac_role_buttons PRIMARY KEY (role_id, button_id)
);
