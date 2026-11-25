-- +goose Up
--  存储管理面 API 权限码种子（G2）。
INSERT INTO rbac_apis (id, method, path, act, summary) VALUES
    ('rbac-api-storage-configs-list', 'GET', '/admin/v1/storage/configs', 'admin.storage.configs.list', '存储配置列表'),
    ('rbac-api-storage-configs-create', 'POST', '/admin/v1/storage/configs', 'admin.storage.configs.write', '创建存储配置'),
    ('rbac-api-storage-configs-update', 'PUT', '/admin/v1/storage/configs/:id', 'admin.storage.configs.write', '更新存储配置'),
    ('rbac-api-storage-configs-delete', 'DELETE', '/admin/v1/storage/configs/:id', 'admin.storage.configs.write', '删除存储配置'),
    ('rbac-api-storage-configs-default', 'PUT', '/admin/v1/storage/configs/:id/default', 'admin.storage.configs.write', '设默认存储配置'),
    ('rbac-api-storage-records-list', 'GET', '/admin/v1/storage/records', 'admin.storage.records.list', '上传记录列表'),
    ('rbac-api-storage-cleanup', 'POST', '/admin/v1/storage/cleanup', 'admin.storage.cleanup', '手动清理过期上传记录')
ON CONFLICT (id) DO NOTHING;
-- +goose Down
DELETE FROM rbac_apis WHERE id LIKE 'rbac-api-storage-%';
