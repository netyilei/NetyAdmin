-- 操作日志表（硬删除表，无 deleted_at 列）
CREATE TABLE IF NOT EXISTS admin_operation_log (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    admin_id BIGINT NOT NULL,
    username VARCHAR(50) NOT NULL,
    action VARCHAR(100) NOT NULL,
    resource VARCHAR(200) NOT NULL,
    detail TEXT,
    ip VARCHAR(50),
    user_agent VARCHAR(500),
    method VARCHAR(10),
    path VARCHAR(200),
    request_id VARCHAR(50),
    status_code INT,
    cost_time BIGINT
);

CREATE INDEX IF NOT EXISTS idx_admin_operation_log_admin_id ON admin_operation_log(admin_id);
CREATE INDEX IF NOT EXISTS idx_admin_operation_log_action ON admin_operation_log(action);
CREATE INDEX IF NOT EXISTS idx_admin_operation_log_created_at ON admin_operation_log(created_at);
-- 高频查询: 操作日志按管理员+时间查询
CREATE INDEX IF NOT EXISTS idx_operation_log_admin_time ON admin_operation_log(admin_id, created_at DESC);

