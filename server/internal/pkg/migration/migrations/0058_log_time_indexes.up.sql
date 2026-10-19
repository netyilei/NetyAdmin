-- created_at indexes for log tables missing them (governance follow-up):
-- DELETE-batch and time-range queries on admin_error_log / sys_task_logs
-- were full scans (0017/0006 only indexed hash/group/name/status/request_id).

CREATE INDEX IF NOT EXISTS idx_admin_error_log_created_at ON admin_error_log(created_at);
CREATE INDEX IF NOT EXISTS idx_sys_task_logs_created_at ON sys_task_logs(created_at);
