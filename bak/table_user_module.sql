-- =============================================
-- User Module - Tables
-- =============================================

BEGIN;

-- 终端用户表
CREATE TABLE IF NOT EXISTS users (
    id VARCHAR(26) PRIMARY KEY, -- ULID
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at BIGINT DEFAULT 0,
    username VARCHAR(50) NOT NULL,
    password VARCHAR(100) NOT NULL,
    nickname VARCHAR(50),
    phone VARCHAR(20),
    email VARCHAR(100),
    avatar VARCHAR(255),
    gender VARCHAR(1) DEFAULT '0', -- 0: 未知, 1: 男, 2: 女
    status VARCHAR(1) DEFAULT '1', -- 1: 正常, 0: 禁用
    last_login_at TIMESTAMP WITH TIME ZONE,
    last_login_ip VARCHAR(50),
    remark TEXT,
    last_read_announcement_id BIGINT DEFAULT 0
);

CREATE UNIQUE INDEX IF NOT EXISTS users_username_key ON users(username) WHERE deleted_at = 0;
CREATE UNIQUE INDEX IF NOT EXISTS users_phone_key ON users(phone) WHERE deleted_at = 0 AND phone IS NOT NULL AND phone != '';
CREATE UNIQUE INDEX IF NOT EXISTS users_email_key ON users(email) WHERE deleted_at = 0 AND email IS NOT NULL AND email != '';
CREATE INDEX IF NOT EXISTS idx_users_deleted ON users(deleted_at);

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
