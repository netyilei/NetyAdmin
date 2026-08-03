-- 后台任务日志表（硬删除表，无 deleted_at 列）
CREATE TABLE IF NOT EXISTS sys_task_logs (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    name VARCHAR(100) NOT NULL,
    start_time TIMESTAMP WITH TIME ZONE NOT NULL,
    end_time TIMESTAMP WITH TIME ZONE NOT NULL,
    duration DOUBLE PRECISION NOT NULL,
    status VARCHAR(20) NOT NULL,
    request_id VARCHAR(50),
    message TEXT
);

CREATE INDEX IF NOT EXISTS idx_sys_task_logs_name ON sys_task_logs(name);
CREATE INDEX IF NOT EXISTS idx_sys_task_logs_status ON sys_task_logs(status);
-- request_id 全链路传播（Task 8.5）
CREATE INDEX IF NOT EXISTS idx_sys_task_logs_request_id ON sys_task_logs(request_id);
-- 高频查询: 获取最新任务日志 WHERE name = ? ORDER BY id DESC LIMIT 1
CREATE INDEX IF NOT EXISTS idx_task_logs_name_id ON sys_task_logs(name, id DESC);

