BEGIN;

-- 应用权限范围表
CREATE TABLE IF NOT EXISTS sys_app_scopes (
    id BIGSERIAL PRIMARY KEY,
    app_id VARCHAR(26) NOT NULL,
    scope VARCHAR(50) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_app_scopes_unique ON sys_app_scopes(app_id, scope);

COMMIT;