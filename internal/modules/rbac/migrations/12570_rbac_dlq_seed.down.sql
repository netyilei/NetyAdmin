-- +goose Down
DELETE FROM rbac_apis WHERE id = 'rbac-api-tc-dlq-replay';
