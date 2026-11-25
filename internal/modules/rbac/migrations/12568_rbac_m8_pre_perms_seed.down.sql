-- +goose Down
DELETE FROM rbac_apis WHERE id LIKE 'rbac-api-audit-%'
    OR id LIKE 'rbac-api-id-admins-%' OR id LIKE 'rbac-api-rb-%';
