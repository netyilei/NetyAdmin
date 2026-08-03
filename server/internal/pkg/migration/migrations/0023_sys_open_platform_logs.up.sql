CREATE TABLE IF NOT EXISTS sys_open_platform_logs (
    id BIGSERIAL PRIMARY KEY,
    app_id VARCHAR(26) NOT NULL,
    app_key VARCHAR(26) NOT NULL,
    api_path VARCHAR(255) NOT NULL,
    api_method VARCHAR(20) NOT NULL,
    client_ip VARCHAR(50) NOT NULL,
    status_code INTEGER NOT NULL,
    latency BIGINT NOT NULL, -- 纳秒级耗时
    request_header TEXT,
    request_body TEXT,
    response_body TEXT,
    error_msg TEXT,
    request_id VARCHAR(50),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_open_log_app_id ON sys_open_platform_logs(app_id);
CREATE INDEX IF NOT EXISTS idx_open_log_created_at ON sys_open_platform_logs(created_at);
CREATE INDEX IF NOT EXISTS idx_open_log_status ON sys_open_platform_logs(status_code);
-- request_id 全链路传播（Task 8.5）
CREATE INDEX IF NOT EXISTS idx_open_log_request_id ON sys_open_platform_logs(request_id);
-- 高频查询: 日志管理（按应用 + 时间范围查询）
CREATE INDEX IF NOT EXISTS idx_open_log_app_time ON sys_open_platform_logs(app_id, created_at DESC);

