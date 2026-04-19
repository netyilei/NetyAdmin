BEGIN;

-- 菜单表
CREATE TABLE IF NOT EXISTS admin_menu (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at BIGINT DEFAULT 0,
    created_by BIGINT,
    updated_by BIGINT,
    parent_id BIGINT DEFAULT 0,
    type VARCHAR(1) NOT NULL,
    name VARCHAR(50) NOT NULL,
    route_name VARCHAR(100) NOT NULL,
    route_path VARCHAR(200) NOT NULL,
    component VARCHAR(100),
    i18_n_key VARCHAR(100),
    icon VARCHAR(100),
    icon_type VARCHAR(1) DEFAULT '1',
    status VARCHAR(1) DEFAULT '1',
    order_by INT DEFAULT 0,
    hide_in_menu BOOLEAN DEFAULT FALSE,
    keep_alive BOOLEAN DEFAULT TRUE,
    constant BOOLEAN DEFAULT FALSE,
    active_menu VARCHAR(100),
    multi_tab BOOLEAN DEFAULT FALSE,
    fixed_index_in_tab INT,
    query TEXT,
    href VARCHAR(200)
);

CREATE UNIQUE INDEX IF NOT EXISTS admin_menu_route_name_key ON admin_menu(route_name) WHERE deleted_at = 0;
CREATE INDEX IF NOT EXISTS idx_admin_menu_deleted ON admin_menu(deleted_at);

COMMIT;