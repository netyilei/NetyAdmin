-- +goose Down
DROP INDEX IF EXISTS idx_rbac_role_buttons_button;
