-- 恢复 msg_blacklist 表（与 0034 up.sql 一致），仅在回滚到本迁移之前时执行。
CREATE TABLE IF NOT EXISTS msg_blacklist (
    id BIGSERIAL PRIMARY KEY,
    user_id VARCHAR(26) NOT NULL,
    channel VARCHAR(20) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_msg_black_user_channel ON msg_blacklist(user_id, channel);
