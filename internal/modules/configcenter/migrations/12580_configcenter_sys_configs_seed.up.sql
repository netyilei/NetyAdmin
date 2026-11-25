-- +goose Up
-- sys_configs seed（11080 配对；配置键由部署者/管理面创建——运行时数据，
-- 首启无种子；示例键语义见 docs/rules/logging.md 与 wireConfigHotReload）。

-- +goose Down
SELECT 1;
