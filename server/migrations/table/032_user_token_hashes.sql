BEGIN;

-- 用户登录凭证哈希表 (用于支持独立 JWT 与单端/多端登录控制)
CREATE TABLE IF NOT EXISTS user_token_hashes (
    id BIGSERIAL PRIMARY KEY,
    user_id VARCHAR(26) NOT NULL,
    token_hash VARCHAR(64) NOT NULL, -- 存储 Token 的哈希值或特定标识
    expired_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_user_token_user_id ON user_token_hashes(user_id);
CREATE INDEX IF NOT EXISTS idx_user_token_expired ON user_token_hashes(expired_at);

COMMIT;