BEGIN;

-- Initial Dictionary Types
INSERT INTO sys_dict_type (name, code, description) VALUES 
('系统状态', 'sys_status', '通用启用/禁用状态'),
('用户性别', 'sys_gender', '用户性别字典'),
('菜单类型', 'menu_type', '系统菜单分类类型'),
('操作动作', 'sys_operation_action', '管理员操作日志动作类型'),
('图标类型', 'menu_icon_type', '侧边栏图标渲染方式'),
('是否', 'sys_yes_no', '通用布尔状态字典'),
('IP策略', 'sys_app_ip_strategy', '应用IP访问控制策略'),
('消息通道', 'sys_msg_channel', '消息发送通道 (sms, email, internal, push)'),
('消息状态', 'sys_msg_status', '消息发送状态 (0:等待, 1:成功, 2:失败)'),
('消息优先级', 'sys_msg_priority', '消息队列优先级 (1:高, 2:中, 3:低)'),
('IP 访问控制类型', 'sys_ip_action_type', 'IP 访问控制动作类型 (1:放行, 2:封禁)')
ON CONFLICT (code) WHERE deleted_at = 0 DO UPDATE SET name = EXCLUDED.name;

COMMIT;