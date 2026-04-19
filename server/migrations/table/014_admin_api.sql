BEGIN;

-- API表
CREATE TABLE IF NOT EXISTS admin_api (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at BIGINT DEFAULT 0,
    menu_id BIGINT NOT NULL,
    name VARCHAR(100) NOT NULL,
    method VARCHAR(10) NOT NULL,
    path VARCHAR(200) NOT NULL,
    description VARCHAR(200),
    auth VARCHAR(1) DEFAULT '1'
);

CREATE UNIQUE INDEX IF NOT EXISTS admin_api_method_path_key ON admin_api(method, path) WHERE deleted_at = 0;
CREATE INDEX IF NOT EXISTS idx_admin_api_deleted ON admin_api(deleted_at);

COMMIT;