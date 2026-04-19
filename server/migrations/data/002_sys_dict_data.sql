BEGIN;

-- Insert sys_status data
INSERT INTO sys_dict_data (dict_code, label, value, tag_type, order_by) VALUES 
('sys_status', 'common.enable', '1', 'success', 1),
('sys_status', 'common.disable', '0', 'error', 2)
ON CONFLICT (dict_code, value) WHERE deleted_at = 0 DO UPDATE SET label = EXCLUDED.label;

-- Insert user_gender data
INSERT INTO sys_dict_data (dict_code, label, value, tag_type, order_by) VALUES 
('sys_gender', 'page.manage.admin.gender.unknown', '0', 'default', 1),
('sys_gender', 'page.manage.admin.gender.male', '1', 'primary', 2),
('sys_gender', 'page.manage.admin.gender.female', '2', 'error', 3)
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

INSERT INTO sys_dict_data (dict_code, label, value, tag_type, order_by) VALUES 
('menu_icon_type', 'page.manage.menu.iconType.iconify', '1', 'primary', 1),
('menu_icon_type', 'page.manage.menu.iconType.local', '2', 'info', 2),
('sys_yes_no', 'common.yes', '1', 'success', 1),
('sys_yes_no', 'common.no', '0', 'error', 2),
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
('sys_msg_priority', 'page.messageHub.priority.low', '3', 'info', 3),
('sys_ip_action_type', 'page.ops.ipac.typeAllow', '1', 'success', 1),
('sys_ip_action_type', 'page.ops.ipac.typeDeny', '2', 'error', 2)
ON CONFLICT (dict_code, value) WHERE deleted_at = 0 DO UPDATE SET label = EXCLUDED.label;

COMMIT;