BEGIN;

-- 按钮表
CREATE TABLE IF NOT EXISTS admin_button (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at BIGINT DEFAULT 0,
    created_by BIGINT DEFAULT 0,
    updated_by BIGINT DEFAULT 0,
    menu_id BIGINT NOT NULL,
    code VARCHAR(100) NOT NULL,
    label VARCHAR(50) NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS admin_button_code_key ON admin_button(code) WHERE deleted_at = 0;
CREATE INDEX IF NOT EXISTS idx_admin_button_deleted ON admin_button(deleted_at);

COMMIT;