-- Task 8.5: 为 sys_open_platform_logs 与 sys_task_logs 新增 request_id 列，
-- 支撑 request_id 全链路传播——日志中可关联到原始请求，
-- 便于通过 Sentry Issue / slog 跨节点追查异步任务链路。
--
-- admin_error_log.request_id 已在迁移 0017 中创建；
-- admin_operation_log.request_id 已在迁移 0016 中创建（本 spec 启用写入）。

ALTER TABLE sys_open_platform_logs
    ADD COLUMN IF NOT EXISTS request_id VARCHAR(50);

ALTER TABLE sys_task_logs
    ADD COLUMN IF NOT EXISTS request_id VARCHAR(50);

CREATE INDEX IF NOT EXISTS idx_open_log_request_id ON sys_open_platform_logs(request_id);
CREATE INDEX IF NOT EXISTS idx_sys_task_logs_request_id ON sys_task_logs(request_id);
