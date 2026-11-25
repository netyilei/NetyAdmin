-- +goose Up
-- super-admin → 通配 API 授权绑定。
INSERT INTO rbac_role_apis (role_id, api_id) VALUES
    ('rbac-role-super-admin', 'rbac-api-wildcard');

-- +goose Down
DELETE FROM rbac_role_apis WHERE role_id = 'rbac-role-super-admin';
