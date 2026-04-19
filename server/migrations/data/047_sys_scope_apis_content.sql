BEGIN;

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

COMMIT;
