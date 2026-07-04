-- 0061_sequence_sync.up.sql
-- 同步主键序列：防止导入 dump 数据后主键序列落后导致的冲突
-- 来源：scripts/sequence_sync.sql（集成为正式迁移以纳入版本控制）
--
-- 幂等性：本迁移可重复执行，setval 仅以当前 MAX(id) 为基准重置序列，
--        不会破坏数据。生产库基线 force 后执行 Up 也不会产生副作用。
-- 注意：仅含 BIGINT serial 主键表；users / sys_apps 等使用 ULID 字符串主键的表不在此列。
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
        -- 仅当表存在时才尝试同步（IF EXISTS 守卫，避免表不存在时 SQL 错误）
        IF EXISTS (
            SELECT 1 FROM information_schema.tables
            WHERE table_schema = current_schema()
              AND table_name = t_name
        ) THEN
            -- 尝试获取绑定的序列
            SELECT pg_get_serial_sequence(t_name, 'id') INTO seq_name;

            -- 如果获取不到（可能是 dump 导入导致的绑定丢失），尝试按标准命名规则猜测
            IF seq_name IS NULL THEN
                SELECT quote_ident(relname) INTO seq_name
                FROM pg_class c
                JOIN pg_namespace n ON n.oid = c.relnamespace
                WHERE relkind = 'S'
                  AND n.nspname = current_schema()
                  AND relname = t_name || '_id_seq';
            END IF;

            -- 仅当序列存在时才执行 setval（幂等守卫）
            IF seq_name IS NOT NULL THEN
                EXECUTE format('SELECT setval(%L, COALESCE((SELECT MAX(id) FROM %I), 1))', seq_name, t_name);
            END IF;
        END IF;
    END LOOP;
END $$;
