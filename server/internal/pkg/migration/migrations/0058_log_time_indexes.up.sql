-- created_at indexes for log tables missing them (governance follow-up):
-- DELETE-batch and time-range queries on admin_error_log / sys_task_logs
-- were full scans (0017/0006 only indexed hash/group/name/status/request_id).

CREATE INDEX IF NOT EXISTS idx_admin_error_log_created_at ON admin_error_log(created_at);
CREATE INDEX IF NOT EXISTS idx_sys_task_logs_created_at ON sys_task_logs(created_at);

-- msg_records 仅 (channel, created_at) 复合索引，纯时间维度的清理子查询无法命中
CREATE INDEX IF NOT EXISTS idx_msg_records_created_at ON msg_records(created_at);
