-- =============================================
-- System Dictionary Module - Seed Data
-- =============================================

BEGIN;

-- 同步序列
DO $$
DECLARE
    seq_name TEXT;
    table_names TEXT[] := ARRAY['sys_dict_type', 'sys_dict_data'];
    t_name TEXT;
BEGIN
    FOREACH t_name IN ARRAY table_names LOOP
        SELECT pg_get_serial_sequence(t_name, 'id') INTO seq_name;
        IF seq_name IS NULL THEN
            SELECT quote_ident(relname) INTO seq_name FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
            WHERE relkind = 'S' AND n.nspname = 'public' AND relname = t_name || '_id_seq';
        END IF;
        IF seq_name IS NOT NULL THEN
            EXECUTE format('SELECT setval(%L, COALESCE((SELECT MAX(id) FROM %I), 1))', seq_name, t_name);
        END IF;
    END LOOP;
END $$;

-- Initial Dictionary Types
INSERT INTO sys_dict_type (name, code, description) VALUES 
('系统状态', 'sys_status', '通用启用/禁用状态'),
('用户性别', 'user_gender', '用户性别字典'),
('菜单类型', 'menu_type', '系统菜单分类类型'),
('操作动作', 'sys_operation_action', '管理员操作日志动作类型')
ON CONFLICT (code) WHERE deleted_at = 0 DO UPDATE SET name = EXCLUDED.name;

-- Insert sys_status data
INSERT INTO sys_dict_data (dict_code, label, value, tag_type, order_by) VALUES 
('sys_status', 'common.enable', '1', 'success', 1),
('sys_status', 'common.disable', '0', 'error', 2)
ON CONFLICT (dict_code, value) WHERE deleted_at = 0 DO UPDATE SET label = EXCLUDED.label;

-- Insert user_gender data
INSERT INTO sys_dict_data (dict_code, label, value, tag_type, order_by) VALUES 
('user_gender', 'page.manage.admin.gender.male', '1', 'primary', 1),
('user_gender', 'page.manage.admin.gender.female', '2', 'error', 2),
('user_gender', 'page.manage.admin.gender.unknown', '3', 'default', 3)
ON CONFLICT (dict_code, value) WHERE deleted_at = 0 DO UPDATE SET label = EXCLUDED.label;

-- Insert menu_type data
INSERT INTO sys_dict_data (dict_code, label, value, tag_type, order_by) VALUES 
('menu_type', 'page.manage.menu.type.dir', '1', 'default', 1),
('menu_type', 'page.manage.menu.type.menu', '2', 'primary', 2),
('menu_type', 'page.manage.menu.type.button', '3', 'info', 3)
ON CONFLICT (dict_code, value) WHERE deleted_at = 0 DO UPDATE SET label = EXCLUDED.label;

-- Insert sys_operation_action data
INSERT INTO sys_dict_data (dict_code, label, value, tag_type, order_by) VALUES 
('sys_operation_action', 'page.ops.operationLog.actionCreate', 'create', 'success', 1),
('sys_operation_action', 'page.ops.operationLog.actionUpdate', 'update', 'warning', 2),
('sys_operation_action', 'page.ops.operationLog.actionDelete', 'delete', 'error', 3),
('sys_operation_action', 'page.ops.operationLog.actionBatchDelete', 'batch_delete', 'error', 4)
ON CONFLICT (dict_code, value) WHERE deleted_at = 0 DO UPDATE SET label = EXCLUDED.label;

-- Insert menu icon type data
INSERT INTO sys_dict_type (name, code, description) VALUES 
('图标类型', 'menu_icon_type', '侧边栏图标渲染方式'),
('是否', 'sys_yes_no', '通用布尔状态字典'),
('应用类型', 'sys_app_type', '开放平台应用类型'),
('IP策略', 'sys_app_ip_strategy', '应用IP访问控制策略'),
('消息通道', 'sys_msg_channel', '消息发送通道 (sms, email, internal, push)'),
('消息状态', 'sys_msg_status', '消息发送状态 (0:等待, 1:成功, 2:失败)'),
('消息优先级', 'sys_msg_priority', '消息队列优先级 (1:高, 2:中, 3:低)')
ON CONFLICT (code) WHERE deleted_at = 0 DO UPDATE SET name = EXCLUDED.name;

INSERT INTO sys_dict_data (dict_code, label, value, tag_type, order_by) VALUES 
('menu_icon_type', 'page.manage.menu.iconType.iconify', '1', 'primary', 1),
('menu_icon_type', 'page.manage.menu.iconType.local', '2', 'info', 2),
('sys_yes_no', 'common.yes', '1', 'success', 1),
('sys_yes_no', 'common.no', '0', 'error', 2),
('sys_app_type', 'page.openPlatform.app.typeInternal', '1', 'success', 1),
('sys_app_type', 'page.openPlatform.app.typeExternal', '2', 'info', 2),
('sys_app_ip_strategy', 'page.openPlatform.app.ipStrategyBlacklist', '1', 'error', 1),
('sys_app_ip_strategy', 'page.openPlatform.app.ipStrategyWhitelist', '2', 'success', 2),
('sys_msg_channel', 'page.messageHub.channel.sms', 'sms', 'info', 1),
('sys_msg_channel', 'page.messageHub.channel.email', 'email', 'warning', 2),
('sys_msg_channel', 'page.messageHub.channel.internal', 'internal', 'success', 3),
('sys_msg_channel', 'page.messageHub.channel.push', 'push', 'primary', 4),
('sys_msg_status', 'page.messageHub.record.pending', '0', 'default', 1),
('sys_msg_status', 'page.messageHub.record.sendSuccess', '1', 'success', 2),
('sys_msg_status', 'page.messageHub.record.sendFailed', '2', 'error', 3),
('sys_msg_priority', 'page.messageHub.priority.high', '1', 'error', 1),
('sys_msg_priority', 'page.messageHub.priority.medium', '2', 'warning', 2),
('sys_msg_priority', 'page.messageHub.priority.low', '3', 'info', 3)
ON CONFLICT (dict_code, value) WHERE deleted_at = 0 DO UPDATE SET label = EXCLUDED.label;

COMMIT;
