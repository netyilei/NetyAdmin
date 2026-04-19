BEGIN;

CREATE TABLE IF NOT EXISTS sys_configs (
    id SERIAL PRIMARY KEY,
    group_name VARCHAR(100) NOT NULL,
    config_key VARCHAR(100) NOT NULL,
    config_value TEXT NOT NULL,
    value_type VARCHAR(50) NOT NULL DEFAULT 'string',
    description VARCHAR(255) DEFAULT '',
    is_system BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at BIGINT DEFAULT 0,
    created_by BIGINT DEFAULT 0,
    updated_by BIGINT DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_sys_configs_deleted ON sys_configs(deleted_at);

-- 每个分组下的Key必须唯一
CREATE UNIQUE INDEX IF NOT EXISTS idx_sys_configs_group_key ON sys_configs(group_name, config_key) WHERE deleted_at = 0;
CREATE INDEX IF NOT EXISTS idx_sys_configs_group ON sys_configs(group_name);

COMMIT;