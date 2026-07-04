-- 错误日志表（硬删除表，无 deleted_at 列）
CREATE TABLE IF NOT EXISTS admin_error_log (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    -- Basic Info
    level VARCHAR(20) NOT NULL,
    message TEXT NOT NULL,
    stack TEXT,
    request_id VARCHAR(50),
    path VARCHAR(200),
    method VARCHAR(10),
    admin_id BIGINT,
    ip VARCHAR(50),
    user_agent VARCHAR(500),
    resolved BOOLEAN DEFAULT FALSE,
    resolved_at VARCHAR(30),
    resolved_by BIGINT,
    -- Aggregation Fields (Later added)
    hash VARCHAR(64),
    group_id BIGINT DEFAULT 0,
    occurrence_count INT DEFAULT 1,
    last_occurred_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_admin_error_log_hash ON admin_error_log(hash);
CREATE INDEX IF NOT EXISTS idx_admin_error_log_group_id ON admin_error_log(group_id);

