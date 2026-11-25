-- +goose Up
-- 邮件设置（SMTP）权限与菜单种子：configcenter 存储 + 热更生效；
-- 权限码与 cmd/netyadmin/smtpconfig.go 常量同名（check-rbac-perms 对偶）。
INSERT INTO rbac_apis (id, method, path, act, summary) VALUES
    ('rbac-api-smtp-get', 'GET', '/admin/v1/system/smtp', 'admin.settings.smtp.view', '读取 SMTP 配置'),
    ('rbac-api-smtp-put', 'PUT', '/admin/v1/system/smtp', 'admin.settings.smtp.operate', '保存 SMTP 配置'),
    ('rbac-api-smtp-reset', 'DELETE', '/admin/v1/system/smtp', 'admin.settings.smtp.operate', '重置 SMTP 配置'),
    ('rbac-api-smtp-test', 'POST', '/admin/v1/system/smtp/test', 'admin.settings.smtp.operate', '发送测试邮件')
ON CONFLICT (id) DO NOTHING;

INSERT INTO rbac_menus (id, parent_id, name, path, component, icon, sort_order, hidden, status) VALUES
    ('rbac-menu-smtp', 'rbac-menu-system', '邮件设置', '/system/smtp', '/system/smtp/index', 'SetUp', 22, false, 1)
ON CONFLICT (id) DO NOTHING;
