BEGIN;

DO $$
DECLARE
    new_user_menu_id BIGINT;
    old_parent_id BIGINT;
BEGIN
    INSERT INTO admin_menu (parent_id, name, route_name, route_path, component, icon, order_by, hide_in_menu, status, type, i18_n_key, created_at, updated_at)
    VALUES (0, '用户', 'user', '/user', 'layout', 'ic:outline-people', 5, false, '1', '1', 'route.user', NOW(), NOW())
    ON CONFLICT (route_name) WHERE deleted_at = 0 DO UPDATE SET name = EXCLUDED.name, i18_n_key = EXCLUDED.i18_n_key, icon = EXCLUDED.icon
    RETURNING id INTO new_user_menu_id;

    UPDATE admin_menu
    SET parent_id = new_user_menu_id,
        name = '用户管理',
        i18_n_key = 'route.manage_user'
    WHERE route_name = 'manage_user' AND deleted_at = 0;
END $$;

COMMIT;
