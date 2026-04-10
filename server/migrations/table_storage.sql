-- =============================================
-- Storage Module - Tables
-- =============================================

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

-- 上传记录表
CREATE TABLE IF NOT EXISTS upload_record (
    id BIGSERIAL PRIMARY KEY,
    storage_config_id BIGINT NOT NULL,
    file_name VARCHAR(255) NOT NULL,
    stored_name VARCHAR(255) NOT NULL,
    file_path VARCHAR(500) NOT NULL,
    file_url VARCHAR(500),
    file_size BIGINT NOT NULL,
    mime_type VARCHAR(100),
    file_ext VARCHAR(20),
    md5 VARCHAR(32),
    source VARCHAR(20) NOT NULL,
    source_id BIGINT,
    source_info TEXT,
    uploader_ip VARCHAR(50),
    user_agent VARCHAR(500),
    business_type VARCHAR(50),
    business_id BIGINT,
    uploaded_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at BIGINT DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_upload_record_storage_config ON upload_record(storage_config_id);
CREATE INDEX IF NOT EXISTS idx_upload_record_source ON upload_record(source, source_id);
CREATE INDEX IF NOT EXISTS idx_upload_record_business ON upload_record(business_type, business_id);
CREATE INDEX IF NOT EXISTS idx_upload_record_uploaded_at ON upload_record(uploaded_at);
CREATE INDEX IF NOT EXISTS idx_upload_record_md5 ON upload_record(md5);

-- 外键约束
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.table_constraints 
        WHERE constraint_name = 'fk_upload_record_storage_config' 
        AND table_name = 'upload_record'
    ) THEN
        ALTER TABLE upload_record 
        ADD CONSTRAINT fk_upload_record_storage_config 
        FOREIGN KEY (storage_config_id) REFERENCES storage_config(id);
    END IF;
END $$;
