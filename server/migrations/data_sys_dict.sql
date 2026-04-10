-- =============================================
-- System Dictionary Module - Seed Data
-- =============================================

-- Initial Dictionary Types
INSERT INTO sys_dict_type (name, code, description) VALUES 
('系统状态', 'sys_status', '通用启用/禁用状态'),
('用户性别', 'user_gender', '用户性别字典'),
('菜单类型', 'menu_type', '系统菜单分类类型')
ON CONFLICT (code) DO NOTHING;

-- Insert sys_status data (with i18n keys)
INSERT INTO sys_dict_data (dict_code, label, value, tag_type, order_by) VALUES 
('sys_status', 'page.manage.common.status.enable', '1', 'success', 1),
('sys_status', 'page.manage.common.status.disable', '0', 'error', 2)
ON CONFLICT (dict_code, value) DO NOTHING;

-- Insert user_gender data (with i18n keys)
INSERT INTO sys_dict_data (dict_code, label, value, tag_type, order_by) VALUES 
('user_gender', 'page.manage.admin.gender.male', '1', 'primary', 1),
('user_gender', 'page.manage.admin.gender.female', '2', 'error', 2),
('user_gender', 'page.manage.admin.gender.unknown', '3', 'default', 3)
ON CONFLICT (dict_code, value) DO NOTHING;

-- Insert menu_type data (with i18n keys)
INSERT INTO sys_dict_data (dict_code, label, value, tag_type, order_by) VALUES 
('menu_type', 'page.manage.menu.type.dir', '1', 'default', 1),
('menu_type', 'page.manage.menu.type.menu', '2', 'primary', 2),
('menu_type', 'page.manage.menu.type.button', '3', 'info', 3)
ON CONFLICT (dict_code, value) DO NOTHING;

-- Insert menu icon type data (with i18n keys)
INSERT INTO sys_dict_type (name, code, description) VALUES 
('菜单图标类型', 'menu_icon_type', '侧边栏图标渲染方式')
ON CONFLICT (code) DO NOTHING;

INSERT INTO sys_dict_data (dict_code, label, value, tag_type, order_by) VALUES 
('menu_icon_type', 'page.manage.menu.iconType.iconify', '1', 'primary', 1),
('menu_icon_type', 'page.manage.menu.iconType.local', '2', 'info', 2)
ON CONFLICT (dict_code, value) DO NOTHING;
