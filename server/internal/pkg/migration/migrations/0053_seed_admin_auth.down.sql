-- 0057_seed_admin_auth.down.sql
-- 回滚 0057：删除种子初始化的超级管理员账号、角色与权限分配
-- 删除顺序（与 up 相反）：先删关联表，最后删 admin_user / admin_role 主表
-- 注意：仅删除 R_SUPER / R_ADMIN 种子角色与 admin 种子账号；
--       其他角色与账号（运维人员后续创建的）不受影响

-- 1. 删除 R_SUPER 角色的菜单 / API / 按钮绑定（up 文件中"自动授权逻辑"插入的行）
DELETE FROM admin_role_menus
WHERE admin_role_id IN (
    SELECT id FROM admin_role WHERE code IN ('R_SUPER', 'R_ADMIN')
);

DELETE FROM admin_role_apis
WHERE admin_role_id IN (
    SELECT id FROM admin_role WHERE code IN ('R_SUPER', 'R_ADMIN')
);

DELETE FROM admin_role_buttons
WHERE admin_role_id IN (
    SELECT id FROM admin_role WHERE code IN ('R_SUPER', 'R_ADMIN')
);

-- 2. 删除 admin 用户与 R_SUPER 角色的关联
DELETE FROM admin_user_roles
WHERE admin_user_id IN (
    SELECT id FROM admin_user WHERE username = 'admin'
);

-- 3. 删除种子 admin 用户（仅删除 username='admin' 的种子账号）
DELETE FROM admin_user WHERE username = 'admin';

-- 4. 删除种子角色 R_SUPER / R_ADMIN
DELETE FROM admin_role
WHERE code IN ('R_SUPER', 'R_ADMIN');
