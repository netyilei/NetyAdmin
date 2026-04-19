BEGIN;

-- 权限组与API关联外键
DO $$ 
BEGIN 
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.table_constraints 
        WHERE constraint_name = 'fk_scope_api_group'
    ) THEN
        ALTER TABLE sys_scope_apis 
        ADD CONSTRAINT fk_scope_api_group 
        FOREIGN KEY (scope_id) REFERENCES sys_app_scope_groups(id);
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM information_schema.table_constraints 
        WHERE constraint_name = 'fk_scope_api_api'
    ) THEN
        ALTER TABLE sys_scope_apis 
        ADD CONSTRAINT fk_scope_api_api 
        FOREIGN KEY (api_id) REFERENCES sys_open_apis(id);
    END IF;
END $$;

COMMIT;
