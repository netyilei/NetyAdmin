BEGIN;

-- Banner组表
CREATE TABLE IF NOT EXISTS content_banner_group (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    code VARCHAR(50) NOT NULL,
    description VARCHAR(255),
    position VARCHAR(50),
    width INT,
    height INT,
    max_items INT DEFAULT 10,
    auto_play BOOLEAN DEFAULT TRUE,
    interval INT DEFAULT 5000,
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

CREATE UNIQUE INDEX IF NOT EXISTS idx_content_banner_group_code ON content_banner_group(code) WHERE deleted_at = 0;

COMMIT;