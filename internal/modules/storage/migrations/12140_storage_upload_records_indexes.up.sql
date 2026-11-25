-- +goose Up
CREATE INDEX idx_storage_records_config_status ON storage_upload_records (config_id, status, created_at DESC);
CREATE INDEX idx_storage_records_uploader ON storage_upload_records (uploader_id, created_at DESC);
CREATE UNIQUE INDEX uk_storage_configs_default ON storage_configs (is_default) WHERE is_default;
-- +goose Down
DROP INDEX IF EXISTS uk_storage_configs_default;
DROP INDEX IF EXISTS idx_storage_records_uploader;
DROP INDEX IF EXISTS idx_storage_records_config_status;
