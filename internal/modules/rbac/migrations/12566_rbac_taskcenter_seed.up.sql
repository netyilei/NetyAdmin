-- +goose Up
--  任务中心管理面权限码种子（G2）。
INSERT INTO rbac_apis (id, method, path, act, summary) VALUES
    ('rbac-api-tc-jobs', 'GET', '/admin/v1/taskcenter/jobs', 'admin.taskcenter.view', '任务目录'),
    ('rbac-api-tc-runs', 'GET', '/admin/v1/taskcenter/jobs/:name/runs', 'admin.taskcenter.view', '运行记录'),
    ('rbac-api-tc-trigger', 'POST', '/admin/v1/taskcenter/jobs/:name/trigger', 'admin.taskcenter.operate', '立即触发'),
    ('rbac-api-tc-pause', 'POST', '/admin/v1/taskcenter/jobs/:name/pause', 'admin.taskcenter.operate', '暂停任务'),
    ('rbac-api-tc-resume', 'POST', '/admin/v1/taskcenter/jobs/:name/resume', 'admin.taskcenter.operate', '恢复任务')
ON CONFLICT (id) DO NOTHING;
-- +goose Down
DELETE FROM rbac_apis WHERE id LIKE 'rbac-api-tc-%';
