BEGIN;

-- 角色表
CREATE TABLE IF NOT EXISTS admin_role (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at BIGINT DEFAULT 0,
    created_by BIGINT DEFAULT 0,
    updated_by BIGINT DEFAULT 0,
    name VARCHAR(50) NOT NULL,
    code VARCHAR(50) NOT NULL,
    description VARCHAR(200),
    status VARCHAR(1) DEFAULT '1',
    home_menu_id BIGINT
);

CREATE UNIQUE INDEX IF NOT EXISTS admin_role_code_key ON admin_role(code) WHERE deleted_at = 0;
CREATE UNIQUE INDEX IF NOT EXISTS admin_role_name_key ON admin_role(name) WHERE deleted_at = 0;
CREATE INDEX IF NOT EXISTS idx_admin_role_deleted ON admin_role(deleted_at);

COMMIT;