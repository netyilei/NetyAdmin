BEGIN;

-- 默认测试应用种子数据
-- 注意: app_secret 为占位密文，管理员需通过「重置密钥」功能获取真实密钥
INSERT INTO sys_apps (id, app_key, app_secret, name, status, ip_strategy, remark) VALUES
('01JQDEFAULTAPP001', '01JQDEFAULTAPP001', 'placeholder-reset-secret-required', '默认测试应用', 1, 1, '系统自动创建的测试应用，请重置密钥后使用')
ON CONFLICT (app_key) WHERE deleted_at = 0 DO NOTHING;

COMMIT;