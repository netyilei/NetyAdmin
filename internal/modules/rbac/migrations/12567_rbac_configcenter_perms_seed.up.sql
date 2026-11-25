-- +goose Up
-- configcenter 管理面权限码种子（对偶漂移补登记——list/update 原缺种子，
-- 精细角色无法授权；rollback=装-A2 清偿端点）。
INSERT INTO rbac_apis (id, method, path, act, summary) VALUES
    ('rbac-api-cc-list', 'GET', '/admin/v1/configs', 'admin.configs.list', '配置列表'),
    ('rbac-api-cc-update', 'PUT', '/admin/v1/configs', 'admin.configs.update', '配置批量更新'),
    ('rbac-api-cc-rollback', 'POST', '/admin/v1/configs/rollback', 'admin.configs.rollback', '回滚到目标版本前状态')
ON CONFLICT (id) DO NOTHING;
-- +goose Down
DELETE FROM rbac_apis WHERE id IN ('rbac-api-cc-list', 'rbac-api-cc-update', 'rbac-api-cc-rollback');
