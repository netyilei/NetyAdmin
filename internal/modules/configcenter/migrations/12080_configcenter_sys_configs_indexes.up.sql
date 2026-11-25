-- +goose Up
-- 查询清单：watcher 全量拉取（无过滤——全表）；public 面（is_public）；组列表（group_name）。
CREATE INDEX idx_sys_configs_public ON sys_configs (is_public) WHERE is_public;
CREATE INDEX idx_sys_configs_group ON sys_configs (group_name);
CREATE INDEX idx_config_history_created ON config_history (created_at DESC);
-- +goose Down
DROP INDEX IF EXISTS idx_config_history_created;
DROP INDEX IF EXISTS idx_sys_configs_group;
DROP INDEX IF EXISTS idx_sys_configs_public;
