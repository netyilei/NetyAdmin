BEGIN;

-- 对象存储配置表
CREATE TABLE IF NOT EXISTS storage_config (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    provider VARCHAR(20) NOT NULL,
    endpoint VARCHAR(255) NOT NULL,
    region VARCHAR(50),
    bucket VARCHAR(100) NOT NULL,
    access_key VARCHAR(255) NOT NULL,
    secret_key VARCHAR(255) NOT NULL,
    domain VARCHAR(255),
    path_prefix VARCHAR(100),
    is_default BOOLEAN DEFAULT FALSE,
    status CHAR(1) DEFAULT '1',
    max_file_size BIGINT DEFAULT 104857600,
    allowed_types TEXT,
    sts_expire_time INT DEFAULT 3600,
    remark TEXT,
    created_by BIGINT,
    updated_by BIGINT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at BIGINT DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_storage_config_provider ON storage_config(provider);
CREATE INDEX IF NOT EXISTS idx_storage_config_status ON storage_config(status);
CREATE INDEX IF NOT EXISTS idx_storage_config_is_default ON storage_config(is_default);
CREATE UNIQUE INDEX IF NOT EXISTS idx_storage_config_name ON storage_config(name) WHERE deleted_at = 0;

COMMIT;