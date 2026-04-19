BEGIN;

-- 消息记录表
CREATE TABLE IF NOT EXISTS msg_records (
    id BIGSERIAL PRIMARY KEY,
    user_id VARCHAR(26), -- 接收用户ID (ULID, 可选, 公告时为空)
    channel VARCHAR(20) NOT NULL,
    receiver VARCHAR(100) NOT NULL, -- 手机号/邮箱/Token
    title VARCHAR(200),
    content TEXT NOT NULL,
    status SMALLINT DEFAULT 0, -- 0:等待, 1:成功, 2:失败
    error_msg TEXT,
    node_id VARCHAR(50),
    priority SMALLINT DEFAULT 2, -- 1:高, 2:中, 3:低
    retry_count INTEGER DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_msg_rec_user ON msg_records(user_id);
CREATE INDEX IF NOT EXISTS idx_msg_rec_status ON msg_records(status);

COMMIT;