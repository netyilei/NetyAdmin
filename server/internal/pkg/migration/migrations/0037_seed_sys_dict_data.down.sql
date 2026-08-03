-- 0041_seed_sys_dict_data.down.sql
-- 回滚 0041：删除种子初始化的字典数据（按 dict_code + value 精确匹配）
-- 注意：所有 dict_code 均来自 0040 种子，单独删除本种子数据不影响其他字典

-- sys_status
DELETE FROM sys_dict_data WHERE dict_code = 'sys_status' AND value IN ('1', '0');

-- sys_gender
DELETE FROM sys_dict_data WHERE dict_code = 'sys_gender' AND value IN ('0', '1', '2');

-- menu_type
DELETE FROM sys_dict_data WHERE dict_code = 'menu_type' AND value IN ('1', '2', '3');

-- sys_operation_action
DELETE FROM sys_dict_data WHERE dict_code = 'sys_operation_action' AND value IN ('create', 'update', 'delete', 'batch_delete');

-- menu_icon_type
DELETE FROM sys_dict_data WHERE dict_code = 'menu_icon_type' AND value IN ('1', '2');

-- sys_yes_no
DELETE FROM sys_dict_data WHERE dict_code = 'sys_yes_no' AND value IN ('1', '0');

-- sys_app_ip_strategy
DELETE FROM sys_dict_data WHERE dict_code = 'sys_app_ip_strategy' AND value IN ('1', '2');

-- sys_msg_channel
DELETE FROM sys_dict_data WHERE dict_code = 'sys_msg_channel' AND value IN ('sms', 'email', 'internal', 'push');

-- sys_msg_status
DELETE FROM sys_dict_data WHERE dict_code = 'sys_msg_status' AND value IN ('0', '1', '2');

-- sys_msg_priority
DELETE FROM sys_dict_data WHERE dict_code = 'sys_msg_priority' AND value IN ('1', '2', '3');

-- sys_ip_action_type
DELETE FROM sys_dict_data WHERE dict_code = 'sys_ip_action_type' AND value IN ('1', '2');
