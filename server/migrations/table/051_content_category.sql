BEGIN;

-- 内容分类表
CREATE TABLE IF NOT EXISTS content_category (
    id BIGSERIAL PRIMARY KEY,
    parent_id BIGINT DEFAULT 0,
    name VARCHAR(50) NOT NULL,
    code VARCHAR(50),
    icon VARCHAR(100),
    content_type VARCHAR(20) DEFAULT 'richtext',
    storage_config_id BIGINT DEFAULT NULL,
    sort INT DEFAULT 0,
    status CHAR(1) DEFAULT '1',
    remark TEXT,
    created_by BIGINT,
    updated_by BIGINT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at BIGINT DEFAULT 0
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_content_category_code ON content_category(code) WHERE deleted_at = 0;
CREATE INDEX IF NOT EXISTS idx_content_category_parent ON content_category(parent_id);

COMMIT;