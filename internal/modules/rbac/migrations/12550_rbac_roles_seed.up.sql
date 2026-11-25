-- +goose Up
-- super-admin 角色（bootstrap 绑定 + 通配策略主体；security.md §1 首启管理员直连全权）。
INSERT INTO rbac_roles (id, code, name, status, remark) VALUES
    ('rbac-role-super-admin', 'super-admin', '超级管理员', 1, 'bootstrap 全权角色');

-- +goose Down
DELETE FROM rbac_roles WHERE code = 'super-admin';
