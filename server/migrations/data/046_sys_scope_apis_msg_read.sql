BEGIN;

DO $$
DECLARE
    msg_read_scope_id BIGINT;
    api_id_val BIGINT;
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

COMMIT;
