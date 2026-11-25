-- +goose Up
-- 通配 API 行（super-admin 全权策略；keyMatch2 + act *——架构 §8.2 超管语义）。
INSERT INTO rbac_apis (id, method, path, act, summary) VALUES
    ('rbac-api-wildcard', '*', '/admin/v1/*', '*', 'super-admin 全权通配');

-- +goose Down
DELETE FROM rbac_apis WHERE id = 'rbac-api-wildcard';
