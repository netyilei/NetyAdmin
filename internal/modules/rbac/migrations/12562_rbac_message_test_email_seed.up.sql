-- +goose Up
--  test-email 管理端点权限种子（message=首个邮件消费方，触发点清偿）。
INSERT INTO rbac_apis (id, method, path, act, summary) VALUES
    ('rbac-api-msg-test-email', 'POST', '/admin/v1/message/test-email', 'admin.message.send', '发送测试邮件（SMTP 联调）')
ON CONFLICT (id) DO NOTHING;
-- +goose Down
DELETE FROM rbac_apis WHERE id = 'rbac-api-msg-test-email';
