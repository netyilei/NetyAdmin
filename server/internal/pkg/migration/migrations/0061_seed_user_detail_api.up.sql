-- 用户详细资料接口（PII 完整查看出口，列表/autocomplete 保持掩码）：
-- GET /admin/v1/systemManage/users/:id
-- 权限模型：超管中间件直接放行；普通角色需在「角色管理」勾选本 API 后生效。
DO $$
DECLARE
    user_menu_id BIGINT;
BEGIN
    SELECT id INTO user_menu_id FROM admin_menu WHERE route_name = 'manage_user' AND deleted_at = 0;
    IF user_menu_id IS NOT NULL THEN
        INSERT INTO admin_api (menu_id, name, method, path, description, auth, created_at, updated_at)
        VALUES
        (user_menu_id, '获取用户详情', 'GET', '/admin/v1/systemManage/users/:id',
         '获取终端用户详细资料（手机号/邮箱为完整值，列表为掩码）', '1', NOW(), NOW())
        ON CONFLICT (method, path) WHERE deleted_at = 0 DO NOTHING;
    END IF;
END $$;
