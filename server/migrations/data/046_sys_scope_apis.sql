BEGIN;

DO $$
DECLARE
    msg_read_scope_id BIGINT;
BEGIN
    SELECT id INTO msg_read_scope_id FROM sys_app_scope_groups WHERE code = 'msg_read' AND deleted_at = 0 LIMIT 1;
    IF msg_read_scope_id IS NULL THEN
        RAISE NOTICE 'msg_read scope group not found, skipping scope-api binding';
        RETURN;
    END IF;

    INSERT INTO sys_scope_apis (scope_id, api_id)
    SELECT msg_read_scope_id, id FROM sys_open_apis
    WHERE path IN (
        '/client/v1/message/internal',
        '/client/v1/message/internal/:id',
        '/client/v1/message/internal/read',
        '/client/v1/message/internal/read-all',
        '/client/v1/message/internal/unread-count'
    ) AND deleted_at = 0
    ON CONFLICT (scope_id, api_id) WHERE deleted_at = 0 DO NOTHING;
END $$;

DO $$
DECLARE
    content_view_scope_id BIGINT;
BEGIN
    SELECT id INTO content_view_scope_id FROM sys_app_scope_groups WHERE code = 'content_view' AND deleted_at = 0 LIMIT 1;
    IF content_view_scope_id IS NULL THEN
        RAISE NOTICE 'content_view scope group not found, skipping scope-api binding';
        RETURN;
    END IF;

    INSERT INTO sys_scope_apis (scope_id, api_id)
    SELECT content_view_scope_id, id FROM sys_open_apis
    WHERE path IN (
        '/client/v1/content/categories/tree',
        '/client/v1/content/articles',
        '/client/v1/content/article/:id',
        '/client/v1/content/banners/:code',
        '/client/v1/content/article/:id/like',
        '/client/v1/content/banners/:id/click'
    ) AND deleted_at = 0
    ON CONFLICT (scope_id, api_id) WHERE deleted_at = 0 DO NOTHING;
END $$;

DO $$
DECLARE
    user_base_scope_id BIGINT;
    user_profile_scope_id BIGINT;
    msg_send_scope_id BIGINT;
BEGIN
    SELECT id INTO user_base_scope_id FROM sys_app_scope_groups WHERE code = 'user_base' AND deleted_at = 0 LIMIT 1;
    SELECT id INTO user_profile_scope_id FROM sys_app_scope_groups WHERE code = 'user_profile' AND deleted_at = 0 LIMIT 1;
    SELECT id INTO msg_send_scope_id FROM sys_app_scope_groups WHERE code = 'msg_send' AND deleted_at = 0 LIMIT 1;

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
