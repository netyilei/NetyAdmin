BEGIN;

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
CREATE UNIQUE INDEX IF NOT EXISTS sys_dict_data_code_value_key ON sys_dict_data(dict_code, value) WHERE deleted_at = 0;

CREATE INDEX IF NOT EXISTS idx_sys_dict_data_deleted ON sys_dict_data(deleted_at);

COMMIT;