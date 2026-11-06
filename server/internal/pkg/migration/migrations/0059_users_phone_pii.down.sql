-- 回滚 0059：恢复明文列唯一索引，删除三列。
-- 注意：phone 明文已被惰性迁移置 NULL，数据本体在 phone_enc（应用侧 AES-GCM），
-- SQL 无法解密——down 仅恢复结构，数据恢复需应用侧用密钥解密 phone_enc 回写。
-- （应急恢复：由代码用 pii.Open 逐行解密，非本迁移职责。）

DROP INDEX IF EXISTS idx_admin_user_phone_idx;
DROP INDEX IF EXISTS idx_users_phone_tail4;
DROP INDEX IF EXISTS users_phone_idx_key;
CREATE UNIQUE INDEX IF NOT EXISTS users_phone_key ON users(phone)
    WHERE deleted_at = 0 AND phone IS NOT NULL AND phone != '';

ALTER TABLE admin_user
    DROP COLUMN IF EXISTS phone_enc,
    DROP COLUMN IF EXISTS phone_idx,
    DROP COLUMN IF EXISTS phone_tail4;

ALTER TABLE users
    DROP COLUMN IF EXISTS phone_enc,
    DROP COLUMN IF EXISTS phone_idx,
    DROP COLUMN IF EXISTS phone_tail4;
