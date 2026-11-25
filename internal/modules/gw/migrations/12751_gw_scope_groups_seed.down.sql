-- +goose Down
DELETE FROM gw_scope_groups WHERE id IN ('gw-scope-admin_audit', 'gw-scope-admin_configcenter', 'gw-scope-admin_gw', 'gw-scope-admin_identity', 'gw-scope-admin_message', 'gw-scope-admin_rbac', 'gw-scope-admin_storage', 'gw-scope-admin_users', 'gw-scope-msg_inbox', 'gw-scope-storage_rw', 'gw-scope-user_base', 'gw-scope-user_profile');
