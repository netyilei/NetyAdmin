BEGIN;

-- 外键约束
DO $$ 
BEGIN 
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.table_constraints 
        WHERE constraint_name = 'fk_upload_record_storage_config'
    ) THEN
        ALTER TABLE upload_record 
        ADD CONSTRAINT fk_upload_record_storage_config 
        FOREIGN KEY (storage_config_id) REFERENCES storage_config(id);
    END IF;
END $$;

COMMIT;
