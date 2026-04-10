-- =============================================
-- System Dictionary Module - Tables
-- =============================================

-- 字典类型表
CREATE TABLE IF NOT EXISTS sys_dict_type (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at BIGINT DEFAULT 0,
    created_by BIGINT,
    updated_by BIGINT,
    name VARCHAR(100) NOT NULL,
    code VARCHAR(100) NOT NULL,
    status VARCHAR(1) DEFAULT '1',
    description VARCHAR(500)
);

CREATE UNIQUE INDEX IF NOT EXISTS sys_dict_type_code_key ON sys_dict_type(code) WHERE deleted_at = 0;
-- 为 ON CONFLICT 增加唯一约束
ALTER TABLE sys_dict_type DROP CONSTRAINT IF EXISTS sys_dict_type_code_unique;
ALTER TABLE sys_dict_type ADD CONSTRAINT sys_dict_type_code_unique UNIQUE (code);

CREATE INDEX IF NOT EXISTS idx_sys_dict_type_deleted ON sys_dict_type(deleted_at);

-- 字典数据表
CREATE TABLE IF NOT EXISTS sys_dict_data (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at BIGINT DEFAULT 0,
    created_by BIGINT,
    updated_by BIGINT,
    dict_code VARCHAR(100) NOT NULL,
    label VARCHAR(100) NOT NULL,
    value VARCHAR(100) NOT NULL,
    tag_type VARCHAR(20) DEFAULT 'default',
    order_by INT DEFAULT 0,
    status VARCHAR(1) DEFAULT '1',
    remark VARCHAR(500)
);

CREATE INDEX IF NOT EXISTS idx_sys_dict_data_code ON sys_dict_data(dict_code);
-- 为 ON CONFLICT 增加唯一约束 (同一字典类型下 value 唯一)
ALTER TABLE sys_dict_data DROP CONSTRAINT IF EXISTS sys_dict_data_code_value_unique;
ALTER TABLE sys_dict_data ADD CONSTRAINT sys_dict_data_code_value_unique UNIQUE (dict_code, value);

CREATE INDEX IF NOT EXISTS idx_sys_dict_data_deleted ON sys_dict_data(deleted_at);
