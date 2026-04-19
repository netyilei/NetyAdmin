BEGIN;

CREATE TABLE IF NOT EXISTS sys_open_apis (
    id BIGSERIAL PRIMARY KEY,
    method VARCHAR(10) NOT NULL,
    path VARCHAR(255) NOT NULL,
    name VARCHAR(100) NOT NULL,
    group_name VARCHAR(50) NOT NULL DEFAULT 'default',
    description TEXT,
    status SMALLINT DEFAULT 1,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at BIGINT DEFAULT 0
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_open_api_method_path ON sys_open_apis(method, path) WHERE deleted_at = 0;
CREATE INDEX IF NOT EXISTS idx_open_api_group ON sys_open_apis(group_name) WHERE deleted_at = 0;

COMMIT;
