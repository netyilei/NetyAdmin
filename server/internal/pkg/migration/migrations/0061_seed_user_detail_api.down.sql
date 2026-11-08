-- admin_role_apis 无外键（0014），必须显式清理角色授权关联，避免孤儿引用
DELETE FROM admin_role_apis
 WHERE admin_api_id IN (SELECT id FROM admin_api
                         WHERE method = 'GET' AND path = '/admin/v1/systemManage/users/:id');
DELETE FROM admin_api WHERE method = 'GET' AND path = '/admin/v1/systemManage/users/:id';
