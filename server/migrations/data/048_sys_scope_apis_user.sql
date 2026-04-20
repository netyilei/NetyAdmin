BEGIN;

DO $$
DECLARE
    user_base_scope_id BIGINT;
    user_profile_scope_id BIGINT;
    msg_send_scope_id BIGINT;
BEGIN
    -- 1. 获取 Scope ID
    SELECT id INTO user_base_scope_id FROM sys_app_scope_groups WHERE code = 'user_base' AND deleted_at = 0 LIMIT 1;
    SELECT id INTO user_profile_scope_id FROM sys_app_scope_groups WHERE code = 'user_profile' AND deleted_at = 0 LIMIT 1;
    SELECT id INTO msg_send_scope_id FROM sys_app_scope_groups WHERE code = 'msg_send' AND deleted_at = 0 LIMIT 1;

    -- 2. 绑定 user_base (虽然目前是 Public，但预留在 OpenPlatform 中)
    IF user_base_scope_id IS NOT NULL THEN
        INSERT INTO sys_scope_apis (scope_id, api_id)
        SELECT user_base_scope_id, id FROM sys_open_apis
        WHERE path IN (
            '/client/v1/user/register',
            '/client/v1/user/login',
            '/client/v1/user/refresh-token',
            '/client/v1/user/reset-password',
            '/client/v1/auth/captcha',
            '/client/v1/auth/captcha-status',
            '/client/v1/auth/verify-config'
        ) AND deleted_at = 0
        ON CONFLICT (scope_id, api_id) WHERE deleted_at = 0 DO NOTHING;
    END IF;

    -- 3. 绑定 user_profile
    IF user_profile_scope_id IS NOT NULL THEN
        INSERT INTO sys_scope_apis (scope_id, api_id)
        SELECT user_profile_scope_id, id FROM sys_open_apis
        WHERE path IN (
            '/client/v1/user/profile',
            '/client/v1/user/password',
            '/client/v1/user/account',
            '/client/v1/user/upload-token',
            '/client/v1/user/logout'
        ) AND deleted_at = 0
        ON CONFLICT (scope_id, api_id) WHERE deleted_at = 0 DO NOTHING;
    END IF;

    -- 4. 绑定 msg_send
    IF msg_send_scope_id IS NOT NULL THEN
        INSERT INTO sys_scope_apis (scope_id, api_id)
        SELECT msg_send_scope_id, id FROM sys_open_apis
        WHERE path IN (
            '/client/v1/auth/send-code'
        ) AND deleted_at = 0
        ON CONFLICT (scope_id, api_id) WHERE deleted_at = 0 DO NOTHING;
    END IF;

END $$;

COMMIT;
