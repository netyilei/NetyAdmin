-- +goose Up
-- US-01 管理面 API 权限码种子（G2）。
INSERT INTO rbac_apis (id, method, path, act, summary) VALUES
    ('rbac-api-users-list', 'GET', '/admin/v1/users', 'admin.users.list', '用户列表（PII 掩码）'),
    ('rbac-api-users-disable', 'PUT', '/admin/v1/users/:id/status', 'admin.users.disable', '用户禁用/启用')
ON CONFLICT (id) DO NOTHING;
-- +goose Down
DELETE FROM rbac_apis WHERE id IN ('rbac-api-users-list', 'rbac-api-users-disable');
