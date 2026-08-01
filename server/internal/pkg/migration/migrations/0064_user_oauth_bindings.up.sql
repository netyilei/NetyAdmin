-- Third-party OAuth account bindings (WeChat, Alipay, GitHub, Apple, etc.)
-- Each (provider, openid) pair maps to exactly one user.
-- A user can bind multiple providers (e.g. WeChat + Apple).
CREATE TABLE IF NOT EXISTS user_oauth_bindings (
    id         BIGSERIAL PRIMARY KEY,
    user_id    VARCHAR(26) NOT NULL,
    provider   VARCHAR(20) NOT NULL,
    openid     VARCHAR(64) NOT NULL,
    unionid    VARCHAR(64),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(provider, openid)
);

CREATE INDEX IF NOT EXISTS idx_oauth_user_id ON user_oauth_bindings(user_id);
CREATE INDEX IF NOT EXISTS idx_oauth_unionid ON user_oauth_bindings(unionid) WHERE unionid IS NOT NULL AND unionid != '';
