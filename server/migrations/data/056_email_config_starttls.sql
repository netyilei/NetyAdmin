BEGIN;

INSERT INTO sys_configs (group_name, config_key, config_value, value_type, description, is_system, created_by, updated_by)
VALUES
('email_config', 'starttls_enabled', 'false', 'boolean', '是否启用 STARTTLS 加密连接 (适用于端口587)', FALSE, 1, 1),
('email_config', 'connect_timeout', '30', 'number', 'SMTP 连接超时时间(秒)', FALSE, 1, 1),
('email_config', 'send_timeout', '30', 'number', 'SMTP 发送超时时间(秒)', FALSE, 1, 1)
ON CONFLICT (group_name, config_key) WHERE deleted_at = 0 DO UPDATE SET
    description = EXCLUDED.description,
    is_system = EXCLUDED.is_system;

UPDATE sys_configs
SET description = 'SMTP 认证方式 (plain/login/crammd5/auto)'
WHERE group_name = 'email_config' AND config_key = 'auth_type' AND deleted_at = 0;

COMMIT;
