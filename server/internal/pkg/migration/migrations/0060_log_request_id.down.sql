-- 回滚 Task 8.5：移除 sys_open_platform_logs 与 sys_task_logs 的 request_id 列与索引

DROP INDEX IF EXISTS idx_sys_task_logs_request_id;
DROP INDEX IF EXISTS idx_open_log_request_id;

ALTER TABLE sys_task_logs
    DROP COLUMN IF EXISTS request_id;

ALTER TABLE sys_open_platform_logs
    DROP COLUMN IF EXISTS request_id;
