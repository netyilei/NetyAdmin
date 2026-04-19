BEGIN;

-- 消息黑名单表
CREATE TABLE IF NOT EXISTS msg_blacklist (
    id BIGSERIAL PRIMARY KEY,
    user_id VARCHAR(26) NOT NULL,
    channel VARCHAR(20) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_msg_black_user_channel ON msg_blacklist(user_id, channel);

COMMIT;