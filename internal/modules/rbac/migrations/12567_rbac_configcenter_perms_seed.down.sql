-- +goose Down
DELETE FROM rbac_apis WHERE id IN ('rbac-api-cc-list', 'rbac-api-cc-update', 'rbac-api-cc-rollback');
