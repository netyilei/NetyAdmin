-- +goose Up
-- rbac_admin_roles：admin↔角色绑定（跨模块只存 admin_id 文本——架构 §9，无 FK）。
CREATE TABLE rbac_admin_roles (
    admin_id   text NOT NULL,
    role_id    text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT pk_rbac_admin_roles PRIMARY KEY (admin_id, role_id)
);
