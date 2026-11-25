-- +goose Down
DROP INDEX IF EXISTS idx_config_history_created;
DROP INDEX IF EXISTS idx_sys_configs_group;
DROP INDEX IF EXISTS idx_sys_configs_public;
