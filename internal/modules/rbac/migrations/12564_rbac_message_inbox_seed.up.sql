-- +goose Up
--  站内信管理端点权限种子（G2）。
INSERT INTO rbac_apis (id, method, path, act, summary) VALUES
    ('rbac-api-msg-inbox-send', 'POST', '/admin/v1/message/inbox', 'admin.message.inbox.send', '发送站内信')
ON CONFLICT (id) DO NOTHING;
-- +goose Down
DELETE FROM rbac_apis WHERE id = 'rbac-api-msg-inbox-send';
