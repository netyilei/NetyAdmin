-- +goose Up
--  死信重放管理面权限种子（cmd/netyadmin/busdlq.go——运维面复用
-- admin.taskcenter.operate，不新增权限码）。
INSERT INTO rbac_apis (id, method, path, act, summary) VALUES
    ('rbac-api-tc-dlq-replay', 'POST', '/admin/v1/taskcenter/dlq/replay', 'admin.taskcenter.operate', '死信重放')
ON CONFLICT (id) DO NOTHING;
