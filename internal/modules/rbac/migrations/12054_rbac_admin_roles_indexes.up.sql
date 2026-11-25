-- +goose Up
-- 反向查询：按角色找已绑定 admin（解绑时定位）。
CREATE INDEX idx_rbac_admin_roles_role ON rbac_admin_roles (role_id);
