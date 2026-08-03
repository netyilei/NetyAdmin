-- 0040_seed_sys_dict_type.down.sql
-- 回滚 0040：删除种子初始化的字典类型（按 code 精确匹配）
DELETE FROM sys_dict_type
WHERE code IN (
    'sys_status',
    'sys_gender',
    'menu_type',
    'sys_operation_action',
    'menu_icon_type',
    'sys_yes_no',
    'sys_app_ip_strategy',
    'sys_msg_channel',
    'sys_msg_status',
    'sys_msg_priority',
    'sys_ip_action_type'
);
