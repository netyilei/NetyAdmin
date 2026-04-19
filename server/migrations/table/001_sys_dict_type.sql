BEGIN;

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
CREATE INDEX IF NOT EXISTS idx_sys_dict_type_deleted ON sys_dict_type(deleted_at);

COMMIT;