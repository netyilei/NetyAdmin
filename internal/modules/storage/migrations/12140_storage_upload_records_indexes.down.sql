-- +goose Down
DROP INDEX IF EXISTS uk_storage_configs_default;
DROP INDEX IF EXISTS idx_storage_records_uploader;
DROP INDEX IF EXISTS idx_storage_records_config_status;
