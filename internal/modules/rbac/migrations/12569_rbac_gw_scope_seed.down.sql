-- +goose Down
DELETE FROM rbac_apis WHERE id IN (
    'rbac-api-gw-scope-groups-list', 'rbac-api-gw-scope-groups-create',
    'rbac-api-gw-scope-groups-update', 'rbac-api-gw-scope-groups-delete');
