-- 手机号 PII 静态加密（users + admin_user，见 CHANGELOG.md 2026-11-07）：
--   phone_enc   TEXT        AES-256-GCM 随机密文（存储列，应用侧解密）
--   phone_idx   VARCHAR(64) HMAC-SHA256 盲索引（检索列，精确匹配/唯一索引）
--   phone_tail4 VARCHAR(4)  归一化末 4 位（明文辅助检索列）
-- 原 phone 明文列暂保留（惰性迁移读取源）：应用启动后台任务将存量明文加密写入
-- 三列后置 phone = NULL。phone_idx 唯一索引在迁移即建——惰性迁移期间并发注册
-- 的唯一性由它保证（明文行 phone_idx 为空，不参与部分唯一索引）。
-- 中缀模糊搜索（LIKE '%…%'）在加密下无法安全保留，检索语义改为
-- 完整号精确（phone_idx）+ 尾号 4 位精确（phone_tail4）。

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS phone_enc   TEXT,
    ADD COLUMN IF NOT EXISTS phone_idx   VARCHAR(64),
    ADD COLUMN IF NOT EXISTS phone_tail4 VARCHAR(4);

ALTER TABLE admin_user
    ADD COLUMN IF NOT EXISTS phone_enc   TEXT,
    ADD COLUMN IF NOT EXISTS phone_idx   VARCHAR(64),
    ADD COLUMN IF NOT EXISTS phone_tail4 VARCHAR(4);

-- 唯一性从明文列移到盲索引列（保留原部分条件语义：未删除且已设置手机号）
DROP INDEX IF EXISTS users_phone_key;
CREATE UNIQUE INDEX IF NOT EXISTS users_phone_idx_key ON users(phone_idx)
    WHERE deleted_at = 0 AND phone_idx IS NOT NULL AND phone_idx <> '';
CREATE INDEX IF NOT EXISTS idx_users_phone_tail4 ON users(phone_tail4)
    WHERE deleted_at = 0 AND phone_tail4 IS NOT NULL AND phone_tail4 <> '';

-- admin_user.phone 原本无唯一约束（语义保持）：普通索引支撑精确筛选
CREATE INDEX IF NOT EXISTS idx_admin_user_phone_idx ON admin_user(phone_idx)
    WHERE deleted_at = 0 AND phone_idx IS NOT NULL AND phone_idx <> '';
