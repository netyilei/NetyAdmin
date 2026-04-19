-- =============================================
-- Admin System Module - Tables
-- =============================================

BEGIN;

-- 管理员用户表
CREATE TABLE IF NOT EXISTS admin_user (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at BIGINT DEFAULT 0,
    created_by BIGINT,
    updated_by BIGINT,
    username VARCHAR(50) NOT NULL,
    password VARCHAR(100) NOT NULL,
    nickname VARCHAR(50),
    phone VARCHAR(20),
    email VARCHAR(100),
    gender VARCHAR(1) DEFAULT '1',
    status VARCHAR(1) DEFAULT '1',
    last_login_at VARCHAR(30)
);

-- 确保旧表也包含新增字段
DO $$ 
BEGIN 
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='admin_user' AND column_name='created_by') THEN
        ALTER TABLE admin_user ADD COLUMN created_by BIGINT;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='admin_user' AND column_name='updated_by') THEN
        ALTER TABLE admin_user ADD COLUMN updated_by BIGINT;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='admin_user' AND column_name='last_login_at') THEN
        ALTER TABLE admin_user ADD COLUMN last_login_at VARCHAR(30);
    END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS admin_user_username_key ON admin_user(username) WHERE deleted_at = 0;
CREATE INDEX IF NOT EXISTS idx_admin_user_deleted ON admin_user(deleted_at);

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

-- 确保旧表也包含新增字段
DO $$ 
BEGIN 
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='admin_role' AND column_name='home_menu_id') THEN
        ALTER TABLE admin_role ADD COLUMN home_menu_id BIGINT;
    END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS admin_role_code_key ON admin_role(code) WHERE deleted_at = 0;
CREATE UNIQUE INDEX IF NOT EXISTS admin_role_name_key ON admin_role(name) WHERE deleted_at = 0;
CREATE INDEX IF NOT EXISTS idx_admin_role_deleted ON admin_role(deleted_at);

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

-- 确保旧表也包含新增字段
DO $$ 
BEGIN 
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='admin_menu' AND column_name='active_menu') THEN
        ALTER TABLE admin_menu ADD COLUMN active_menu VARCHAR(100);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='admin_menu' AND column_name='multi_tab') THEN
        ALTER TABLE admin_menu ADD COLUMN multi_tab BOOLEAN DEFAULT FALSE;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='admin_menu' AND column_name='fixed_index_in_tab') THEN
        ALTER TABLE admin_menu ADD COLUMN fixed_index_in_tab INT;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='admin_menu' AND column_name='query') THEN
        ALTER TABLE admin_menu ADD COLUMN query TEXT;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='admin_menu' AND column_name='href') THEN
        ALTER TABLE admin_menu ADD COLUMN href VARCHAR(200);
    END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS admin_menu_route_name_key ON admin_menu(route_name) WHERE deleted_at = 0;
CREATE INDEX IF NOT EXISTS idx_admin_menu_deleted ON admin_menu(deleted_at);

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

-- 确保旧表也包含新增字段
DO $$ 
BEGIN 
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='admin_api' AND column_name='auth') THEN
        ALTER TABLE admin_api ADD COLUMN auth VARCHAR(1) DEFAULT '1';
    END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS admin_api_method_path_key ON admin_api(method, path) WHERE deleted_at = 0;
CREATE INDEX IF NOT EXISTS idx_admin_api_deleted ON admin_api(deleted_at);

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

-- 确保旧表也包含新增字段
DO $$ 
BEGIN 
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='admin_button' AND column_name='created_by') THEN
        ALTER TABLE admin_button ADD COLUMN created_by BIGINT DEFAULT 0;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='admin_button' AND column_name='updated_by') THEN
        ALTER TABLE admin_button ADD COLUMN updated_by BIGINT DEFAULT 0;
    END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS admin_button_code_key ON admin_button(code) WHERE deleted_at = 0;
CREATE INDEX IF NOT EXISTS idx_admin_button_deleted ON admin_button(deleted_at);

-- 用户角色关联表
CREATE TABLE IF NOT EXISTS admin_user_roles (
    admin_user_id BIGINT NOT NULL,
    admin_role_id BIGINT NOT NULL,
    PRIMARY KEY (admin_user_id, admin_role_id)
);

-- 角色菜单关联表
CREATE TABLE IF NOT EXISTS admin_role_menus (
    admin_role_id BIGINT NOT NULL,
    admin_menu_id BIGINT NOT NULL,
    PRIMARY KEY (admin_role_id, admin_menu_id)
);

-- 角色API关联表
CREATE TABLE IF NOT EXISTS admin_role_apis (
    admin_role_id BIGINT NOT NULL,
    admin_api_id BIGINT NOT NULL,
    PRIMARY KEY (admin_role_id, admin_api_id)
);

-- 角色按钮关联表
CREATE TABLE IF NOT EXISTS admin_role_buttons (
    admin_role_id BIGINT NOT NULL,
    admin_button_id BIGINT NOT NULL,
    PRIMARY KEY (admin_role_id, admin_button_id)
);

-- 操作日志表
CREATE TABLE IF NOT EXISTS admin_operation_log (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at BIGINT DEFAULT 0,
    user_id BIGINT NOT NULL,
    username VARCHAR(50) NOT NULL,
    action VARCHAR(100) NOT NULL,
    resource VARCHAR(200) NOT NULL,
    detail TEXT,
    ip VARCHAR(50),
    user_agent VARCHAR(500),
    method VARCHAR(10),
    path VARCHAR(200),
    request_id VARCHAR(50),
    status_code INT,
    cost_time BIGINT
);

-- 确保旧表也包含新增字段
DO $$ 
BEGIN 
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='admin_operation_log' AND column_name='method') THEN
        ALTER TABLE admin_operation_log ADD COLUMN method VARCHAR(10);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='admin_operation_log' AND column_name='path') THEN
        ALTER TABLE admin_operation_log ADD COLUMN path VARCHAR(200);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='admin_operation_log' AND column_name='request_id') THEN
        ALTER TABLE admin_operation_log ADD COLUMN request_id VARCHAR(50);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='admin_operation_log' AND column_name='status_code') THEN
        ALTER TABLE admin_operation_log ADD COLUMN status_code INT;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='admin_operation_log' AND column_name='cost_time') THEN
        ALTER TABLE admin_operation_log ADD COLUMN cost_time BIGINT;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_admin_operation_log_deleted ON admin_operation_log(deleted_at);
CREATE INDEX IF NOT EXISTS idx_admin_operation_log_user_id ON admin_operation_log(user_id);
CREATE INDEX IF NOT EXISTS idx_admin_operation_log_action ON admin_operation_log(action);
CREATE INDEX IF NOT EXISTS idx_admin_operation_log_created_at ON admin_operation_log(created_at);

-- 错误日志表
CREATE TABLE IF NOT EXISTS admin_error_log (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at BIGINT DEFAULT 0,
    -- Basic Info
    level VARCHAR(20) NOT NULL,
    message TEXT NOT NULL,
    stack TEXT,
    request_id VARCHAR(50),
    path VARCHAR(200),
    method VARCHAR(10),
    user_id BIGINT,
    ip VARCHAR(50),
    user_agent VARCHAR(500),
    resolved BOOLEAN DEFAULT FALSE,
    resolved_at VARCHAR(30),
    resolved_by BIGINT,
    -- Aggregation Fields (Later added)
    hash VARCHAR(64),
    group_id BIGINT DEFAULT 0,
    occurrence_count INT DEFAULT 1,
    last_occurred_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- 确保旧表也包含聚合字段 (防止 CREATE TABLE IF NOT EXISTS 跳过新增列)
DO $$ 
BEGIN 
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='admin_error_log' AND column_name='hash') THEN
        ALTER TABLE admin_error_log ADD COLUMN hash VARCHAR(64);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='admin_error_log' AND column_name='group_id') THEN
        ALTER TABLE admin_error_log ADD COLUMN group_id BIGINT DEFAULT 0;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='admin_error_log' AND column_name='occurrence_count') THEN
        ALTER TABLE admin_error_log ADD COLUMN occurrence_count INT DEFAULT 1;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='admin_error_log' AND column_name='last_occurred_at') THEN
        ALTER TABLE admin_error_log ADD COLUMN last_occurred_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_admin_error_log_deleted ON admin_error_log(deleted_at);
CREATE INDEX IF NOT EXISTS idx_admin_error_log_hash ON admin_error_log(hash) WHERE deleted_at = 0;
CREATE INDEX IF NOT EXISTS idx_admin_error_log_group_id ON admin_error_log(group_id);

-- 后台任务日志表
CREATE TABLE IF NOT EXISTS sys_task_logs (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at BIGINT DEFAULT 0,
    name VARCHAR(100) NOT NULL,
    start_time TIMESTAMP WITH TIME ZONE NOT NULL,
    end_time TIMESTAMP WITH TIME ZONE NOT NULL,
    duration DOUBLE PRECISION NOT NULL,
    status VARCHAR(20) NOT NULL,
    message TEXT
);

CREATE INDEX IF NOT EXISTS idx_sys_task_logs_name ON sys_task_logs(name);
CREATE INDEX IF NOT EXISTS idx_sys_task_logs_status ON sys_task_logs(status);
CREATE INDEX IF NOT EXISTS idx_sys_task_logs_deleted ON sys_task_logs(deleted_at);

COMMIT;
