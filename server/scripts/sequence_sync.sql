BEGIN;

-- 同步序列：防止导入 dump 数据后主键序列落后导致的冲突
DO $$
DECLARE
    seq_name TEXT;
    table_names TEXT[] := ARRAY[
        'admin_user', 'admin_role', 'admin_menu', 'admin_api', 'admin_button',
        'sys_dict_type', 'sys_dict_data', 'sys_configs', 'sys_ip_access_control',
        'content_category', 'content_article', 'content_banner_group', 'content_banner_item',
        'storage_config', 'upload_record', 'sys_open_apis', 'sys_app_scope_groups', 'sys_scope_apis'
    ];
    t_name TEXT;
BEGIN
    FOREACH t_name IN ARRAY table_names LOOP
        -- 尝试获取绑定的序列
        SELECT pg_get_serial_sequence(t_name, 'id') INTO seq_name;
        
        -- 如果获取不到（可能是 dump 导入导致的绑定丢失），尝试按标准命名规则猜测
        IF seq_name IS NULL THEN
            SELECT quote_ident(relname) INTO seq_name
            FROM pg_class c
            JOIN pg_namespace n ON n.oid = c.relnamespace
            WHERE relkind = 'S' 
              AND n.nspname = 'public'
              AND relname = t_name || '_id_seq';
        END IF;

        IF seq_name IS NOT NULL THEN
            EXECUTE format('SELECT setval(%L, COALESCE((SELECT MAX(id) FROM %I), 1))', seq_name, t_name);
        END IF;
    END LOOP;
END $$;

COMMIT;