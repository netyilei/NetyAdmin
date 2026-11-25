-- +goose Up
--  管理面 API 权限码种子（G2：功能交付必须带权限数据）。
INSERT INTO rbac_apis (id, method, path, act, summary) VALUES
    ('rbac-api-msg-templates-list', 'GET', '/admin/v1/message/templates', 'admin.message.templates.list', '消息模板列表'),
    ('rbac-api-msg-templates-create', 'POST', '/admin/v1/message/templates', 'admin.message.templates.write', '创建消息模板'),
    ('rbac-api-msg-templates-update', 'PUT', '/admin/v1/message/templates/:id', 'admin.message.templates.write', '更新消息模板'),
    ('rbac-api-msg-templates-delete', 'DELETE', '/admin/v1/message/templates/:id', 'admin.message.templates.write', '删除消息模板'),
    ('rbac-api-msg-send', 'POST', '/admin/v1/message/send', 'admin.message.send', '按模板发送消息'),
    ('rbac-api-msg-records-list', 'GET', '/admin/v1/message/records', 'admin.message.records.list', '发送记录列表'),
    ('rbac-api-msg-records-resend', 'POST', '/admin/v1/message/records/:id/resend', 'admin.message.send', '重发消息')
ON CONFLICT (id) DO NOTHING;
-- +goose Down
DELETE FROM rbac_apis WHERE id IN (
    'rbac-api-msg-templates-list', 'rbac-api-msg-templates-create', 'rbac-api-msg-templates-update',
    'rbac-api-msg-templates-delete', 'rbac-api-msg-send', 'rbac-api-msg-records-list',
    'rbac-api-msg-records-resend');
