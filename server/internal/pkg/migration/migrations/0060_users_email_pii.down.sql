-- 回滚 0060：恢复明文列唯一索引，删除两列。
-- 数据恢复同 0059：明文已由惰性迁移置 NULL，本体在 email_enc（应用侧 AES-GCM），
-- SQL 无法解密——应急恢复由代码用 pii.OpenEmail 逐行解密回写，非本迁移职责。

DROP INDEX IF EXISTS idx_admin_user_email_idx;
DROP INDEX IF EXISTS users_email_idx_key;
CREATE UNIQUE INDEX IF NOT EXISTS users_email_key ON users(email)
    WHERE deleted_at = 0 AND email IS NOT NULL AND email != '';

ALTER TABLE admin_user
    DROP COLUMN IF EXISTS email_enc,
    DROP COLUMN IF EXISTS email_idx;

ALTER TABLE users
    DROP COLUMN IF EXISTS email_enc,
    DROP COLUMN IF EXISTS email_idx;
