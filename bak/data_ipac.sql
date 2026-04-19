-- =============================================
-- IP Access Control Module - Data
-- =============================================

BEGIN;

-- 模块子菜单初始化 
DO $$ 
DECLARE 
    ops_menu_id BIGINT; 
    ipac_menu_id BIGINT;
BEGIN 
    SELECT id INTO ops_menu_id FROM admin_menu WHERE route_name = 'ops' AND deleted_at = 0; 
 
    -- 运维管理子菜单: IP 访问控制
    IF ops_menu_id IS NOT NULL THEN 
        INSERT INTO admin_menu (parent_id, name, route_name, route_path, component, icon, order_by, hide_in_menu, status, type, i18_n_key, created_at, updated_at) 
        VALUES 
        (ops_menu_id, 'IP 访问控制', 'ops_ip-access', '/ops/ip-access', 'view.ops_ip-access', 'ic:baseline-security', 4, false, '1', '2', 'route.ops_ip-access', NOW(), NOW()) 
        ON CONFLICT (route_name) WHERE deleted_at = 0 DO UPDATE SET 
            name = EXCLUDED.name,
            route_path = EXCLUDED.route_path,
            component = EXCLUDED.component,
            icon = EXCLUDED.icon,
            order_by = EXCLUDED.order_by,
            i18_n_key = EXCLUDED.i18_n_key,
            updated_at = NOW(); 
    END IF; 

    SELECT id INTO ipac_menu_id FROM admin_menu WHERE route_name = 'ops_ip-access' AND deleted_at = 0;

    -- IP 访问控制 API 初始化
    IF ipac_menu_id IS NOT NULL THEN
        INSERT INTO admin_api (menu_id, name, method, path, description, auth, created_at, updated_at)
        VALUES 
        (ipac_menu_id, '获取 IP 规则列表', 'GET', '/admin/v1/ops/ip-access', '获取 IP 规则列表', '1', NOW(), NOW()),
        (ipac_menu_id, '新增 IP 规则', 'POST', '/admin/v1/ops/ip-access', '新增 IP 规则', '1', NOW(), NOW()),
        (ipac_menu_id, '修改 IP 规则', 'PUT', '/admin/v1/ops/ip-access', '修改 IP 规则', '1', NOW(), NOW()),
        (ipac_menu_id, '删除 IP 规则', 'DELETE', '/admin/v1/ops/ip-access', '删除 IP 规则', '1', NOW(), NOW()),
        (ipac_menu_id, '批量删除规则', 'DELETE', '/admin/v1/ops/ip-access/batch', '批量删除规则', '1', NOW(), NOW())
        ON CONFLICT (method, path) WHERE deleted_at = 0 DO NOTHING;

        -- 按钮权限
        INSERT INTO admin_button (menu_id, label, code, created_at, updated_at)
        VALUES
        (ipac_menu_id, '查询', 'ip:access:query', NOW(), NOW()),
        (ipac_menu_id, '新增', 'ip:access:add', NOW(), NOW()),
        (ipac_menu_id, '编辑', 'ip:access:edit', NOW(), NOW()),
        (ipac_menu_id, '删除', 'ip:access:delete', NOW(), NOW()),
        (ipac_menu_id, '批量删除', 'ip:access:batchDelete', NOW(), NOW())
        ON CONFLICT (code) WHERE deleted_at = 0 DO NOTHING;
    END IF;

    -- 字典数据初始化
    INSERT INTO sys_dict_type (name, code, description) VALUES 
    ('IP 访问控制类型', 'sys_ip_action_type', 'IP 访问控制动作类型 (1:放行, 2:封禁)')
    ON CONFLICT (code) WHERE deleted_at = 0 DO UPDATE SET name = EXCLUDED.name;

    INSERT INTO sys_dict_data (dict_code, label, value, tag_type, order_by) VALUES 
    ('sys_ip_action_type', 'page.ops.ipac.typeAllow', '1', 'success', 1),
    ('sys_ip_action_type', 'page.ops.ipac.typeDeny', '2', 'error', 2)
    ON CONFLICT (dict_code, value) WHERE deleted_at = 0 DO UPDATE SET label = EXCLUDED.label;
END $$;

COMMIT;
