BEGIN;

-- 角色菜单关联表
CREATE TABLE IF NOT EXISTS admin_role_menus (
    admin_role_id BIGINT NOT NULL,
    admin_menu_id BIGINT NOT NULL,
    PRIMARY KEY (admin_role_id, admin_menu_id)
);

COMMIT;