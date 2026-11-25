-- +goose Down
DROP INDEX IF EXISTS idx_admins_status;
DROP INDEX IF EXISTS idx_admin_token_revocations_expires;
DROP INDEX IF EXISTS idx_admin_sessions_admin;
DROP INDEX IF EXISTS idx_admin_sessions_expires;
