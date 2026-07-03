-- 0058_token_version.down.sql
-- 回滚 BUG #5 Token 版本号机制

ALTER TABLE users DROP COLUMN IF EXISTS token_version;
ALTER TABLE admin_user DROP COLUMN IF EXISTS token_version;
