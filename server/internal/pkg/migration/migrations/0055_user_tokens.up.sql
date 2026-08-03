-- Client (C-end) multi-device session storage.
-- One row per (user_id, platform): Login UPSERTs and increments token_version,
-- which invalidates older tokens on the same platform (single-device kick-in per platform).
-- Cross-platform sessions are independent (web + mobile can coexist).
CREATE TABLE IF NOT EXISTS user_tokens (
    id                 BIGSERIAL PRIMARY KEY,
    user_id            VARCHAR(26) NOT NULL,
    platform           VARCHAR(50) NOT NULL,                       -- business-defined (web/mobile/miniapp/...)
    token_version      BIGINT NOT NULL DEFAULT 0,                  -- per-platform version; bumped on each Login
    access_hash        VARCHAR(64) NOT NULL DEFAULT '',            -- current access token hash
    refresh_hash       VARCHAR(64) NOT NULL DEFAULT '',            -- current refresh token hash
    access_expires_at  TIMESTAMP WITH TIME ZONE,                   -- for cleanup of stale rows
    refresh_expires_at TIMESTAMP WITH TIME ZONE,
    created_at         TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at         TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, platform)                                      -- one row per user+platform; UPSERT target
);

CREATE INDEX IF NOT EXISTS idx_user_tokens_user ON user_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_user_tokens_access_expires ON user_tokens(access_expires_at);
