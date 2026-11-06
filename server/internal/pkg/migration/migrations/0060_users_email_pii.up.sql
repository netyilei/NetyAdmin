-- 邮箱 PII 静态加密（users + admin_user，与 0059 手机号同构，见 CHANGELOG.md）：
--   email_enc TEXT        AES-256-GCM 随机密文（保留原始大小写，应用侧解密）
--   email_idx VARCHAR(64) HMAC-SHA256 盲索引（小写归一化；检索列/唯一索引）
-- 与手机号的两处差异：无尾号辅助列（邮箱没有等价物），检索语义仅完整地址精确；
-- 大小写多形态映射同一盲索引（邮箱路由实践不区分大小写）。
-- 原 email 明文列暂保留（惰性迁移读取源），加密完成后由后台任务置 NULL。
-- 中缀模糊搜索永久下线，检索语义 = 完整地址精确。

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS email_enc TEXT,
    ADD COLUMN IF NOT EXISTS email_idx VARCHAR(64);

ALTER TABLE admin_user
    ADD COLUMN IF NOT EXISTS email_enc TEXT,
    ADD COLUMN IF NOT EXISTS email_idx VARCHAR(64);

-- 唯一性从明文列移到盲索引列
DROP INDEX IF EXISTS users_email_key;
CREATE UNIQUE INDEX IF NOT EXISTS users_email_idx_key ON users(email_idx)
    WHERE deleted_at = 0 AND email_idx IS NOT NULL AND email_idx <> '';
CREATE INDEX IF NOT EXISTS idx_admin_user_email_idx ON admin_user(email_idx)
    WHERE deleted_at = 0 AND email_idx IS NOT NULL AND email_idx <> '';
