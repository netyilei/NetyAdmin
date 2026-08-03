-- Admin session token hashes (admin-side login credential storage).
-- Each row records one issued access/refresh token hash; RequireAuth verifies
-- the presented admin token against this table.
-- Note: client (C-end) sessions live in user_tokens (0055) — separated to give
--       each side its own session model (admin single-session vs client multi-device).
CREATE TABLE IF NOT EXISTS admin_tokens (
    id BIGSERIAL PRIMARY KEY,
    user_id VARCHAR(26) NOT NULL,                              -- stores admin_id
    token_hash VARCHAR(64) NOT NULL,                           -- hash of the issued access/refresh token
    expired_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_admin_tokens_admin_id ON admin_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_admin_tokens_expired ON admin_tokens(expired_at);
