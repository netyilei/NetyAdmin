-- 初始化基础系统缓存开关
INSERT INTO sys_configs (group_name, config_key, config_value, value_type, description, is_system, created_at, updated_at)
VALUES 
('cache_switches', 'rbac_auth', 'true', 'boolean', 'RBAC权限与菜单总开关', TRUE, NOW(), NOW()),
('cache_switches', 'sys_config', 'true', 'boolean', '系统全局配置逻辑总开关', TRUE, NOW(), NOW()),
('cache_switches', 'rbac_menu', 'true', 'boolean', '菜单树格式化缓存开关', TRUE, NOW(), NOW()),
('cache_switches', 'admin', 'true', 'boolean', '管理员个人资料缓存开关', TRUE, NOW(), NOW()),
('cache_switches', 'dict', 'true', 'boolean', '字典数据缓存开关', TRUE, NOW(), NOW()),
('cache_switches', 'storage', 'true', 'boolean', '对象存储配置缓存开关', TRUE, NOW(), NOW()),
('system_params', 'admin_max_login_fails', '5', 'number', '管理员最多连续登录失败次数', TRUE, NOW(), NOW()),
-- 任务配置
('task_config', 'log_enabled', 'true', 'boolean', '是否开启任务日志记录', TRUE, NOW(), NOW()),
('task_config', 'retention_days', '30', 'number', '任务日志保留天数', TRUE, NOW(), NOW()),
-- 日志维护配置
('ops_config', 'retention_days', '90', 'number', '操作日志保留天数', TRUE, NOW(), NOW()),
('error_config', 'retention_days', '180', 'number', '错误日志保留天数', TRUE, NOW(), NOW())
ON CONFLICT (group_name, config_key) DO NOTHING;
