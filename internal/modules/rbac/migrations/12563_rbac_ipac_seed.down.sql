-- +goose Down
DELETE FROM rbac_apis WHERE id IN (
    'rbac-api-gw-ip-groups-list', 'rbac-api-gw-ip-groups-create', 'rbac-api-gw-ip-groups-update',
    'rbac-api-gw-ip-groups-delete', 'rbac-api-gw-ip-rules-list', 'rbac-api-gw-ip-rules-create',
    'rbac-api-gw-ip-rules-delete', 'rbac-api-gw-ip-reload');
