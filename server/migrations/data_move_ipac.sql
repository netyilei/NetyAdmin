BEGIN;

DO $$
DECLARE
    settings_menu_id BIGINT;
BEGIN
    SELECT id INTO settings_menu_id FROM admin_menu WHERE route_name = 'settings' AND deleted_at = 0;

    IF settings_menu_id IS NOT NULL THEN
        UPDATE admin_menu
        SET parent_id = settings_menu_id
        WHERE route_name = 'ops_ip-access' AND deleted_at = 0;
    END IF;
END $$;

COMMIT;
