BEGIN;

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
    source_id VARCHAR(26),
    source_info TEXT,
    uploader_ip VARCHAR(50),
    user_agent VARCHAR(500),
    business_type VARCHAR(50),
    business_id VARCHAR(26),
    app_id VARCHAR(26) DEFAULT '', -- 开放平台应用ID，空字符串表示非应用上传
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
CREATE INDEX IF NOT EXISTS idx_upload_record_app_id ON upload_record(app_id);

COMMIT;