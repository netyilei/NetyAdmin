-- +goose Down
DELETE FROM rbac_apis WHERE id LIKE 'rbac-api-users-%';
