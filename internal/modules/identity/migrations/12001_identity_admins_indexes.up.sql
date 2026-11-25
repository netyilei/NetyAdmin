-- +goose Up
-- 查询清单：列表（status）；过期清理（expires_at）。
CREATE INDEX idx_admins_status ON admins (status);
CREATE INDEX idx_admin_token_revocations_expires ON admin_token_revocations (expires_at);
CREATE INDEX idx_admin_sessions_admin ON admin_sessions (admin_id);
CREATE INDEX idx_admin_sessions_expires ON admin_sessions (expires_at);
