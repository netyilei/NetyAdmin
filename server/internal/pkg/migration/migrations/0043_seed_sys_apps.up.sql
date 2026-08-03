-- 默认测试应用种子数据
--
-- app_secret 为固定默认值（开箱即用）：明文 = netyadmin-default-app-secret
-- 密文用默认 [security].aes_key = "netyadmin-aes-key-32-chars-long!" 加密（AES-256-GCM）。
-- 克隆基座后默认应用可直接用该 secret 签名调用 client API（与默认账号 admin/admin123 同理）。
--
-- ⚠️ 安全提示：
--   1. 生产环境部署后请通过 PUT /admin/v1/open/apps/reset-secret 重置密钥，勿使用默认值。
--   2. 若修改了 [security].aes_key（生产环境必改），此密文将无法解密，
--      需对默认应用执行 reset-secret 重置（明文 secret 值见上方）。
INSERT INTO sys_apps (id, app_key, app_secret, name, status, ip_strategy, remark) VALUES
('01JQDEFAULTAPP001', '01JQDEFAULTAPP001', 'o48owixDW/VzZ/8T2zFwjy2LcEiJyGOuPZ1D20yiVZCBis+hQ4t+1c5auUziPGMHjc5FTSHoc2g=', '默认测试应用', 1, 1, '默认应用（开箱即用），生产环境请重置密钥后使用')
ON CONFLICT (app_key) WHERE deleted_at = 0 DO NOTHING;
