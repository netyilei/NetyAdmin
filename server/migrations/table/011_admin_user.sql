BEGIN;

-- 管理员用户表
CREATE TABLE IF NOT EXISTS admin_user (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at BIGINT DEFAULT 0,
    created_by BIGINT,
    updated_by BIGINT,
    username VARCHAR(50) NOT NULL,
    password VARCHAR(100) NOT NULL,
    nickname VARCHAR(50),
    phone VARCHAR(20),
    email VARCHAR(100),
    gender VARCHAR(1) DEFAULT '1',
    status VARCHAR(1) DEFAULT '1',
    last_login_at VARCHAR(30)
);

CREATE UNIQUE INDEX IF NOT EXISTS admin_user_username_key ON admin_user(username) WHERE deleted_at = 0;
CREATE INDEX IF NOT EXISTS idx_admin_user_deleted ON admin_user(deleted_at);

COMMIT;