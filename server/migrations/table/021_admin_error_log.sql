BEGIN;

-- 错误日志表
CREATE TABLE IF NOT EXISTS admin_error_log (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at BIGINT DEFAULT 0,
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

CREATE INDEX IF NOT EXISTS idx_admin_error_log_deleted ON admin_error_log(deleted_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_admin_error_log_hash ON admin_error_log(hash) WHERE deleted_at = 0;
CREATE INDEX IF NOT EXISTS idx_admin_error_log_group_id ON admin_error_log(group_id);

COMMIT;