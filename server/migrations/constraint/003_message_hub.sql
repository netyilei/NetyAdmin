BEGIN;

-- 站内信外键约束
DO $$ 
BEGIN 
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.table_constraints 
        WHERE constraint_name = 'fk_msg_internal_msg_record'
    ) THEN
        ALTER TABLE msg_internal 
        ADD CONSTRAINT fk_msg_internal_msg_record 
        FOREIGN KEY (msg_record_id) REFERENCES msg_records(id) ON DELETE CASCADE;
    END IF;

    -- 站内信已读记录外键约束
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.table_constraints 
        WHERE constraint_name = 'fk_msg_internal_reads_msg_internal'
    ) THEN
        ALTER TABLE msg_internal_reads 
        ADD CONSTRAINT fk_msg_internal_reads_msg_internal 
        FOREIGN KEY (msg_internal_id) REFERENCES msg_internal(id) ON DELETE CASCADE;
    END IF;
END $$;

COMMIT;
