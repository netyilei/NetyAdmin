DELETE FROM admin_api WHERE method = 'GET' AND path = '/admin/v1/systemManage/users/:id';
-- 关联的角色授权随 admin_role_apis 级联清理（外键 ON DELETE CASCADE）
