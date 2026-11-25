-- +goose Down
DELETE FROM gw_app_scopes WHERE app_id IN ('gw-app-admin-console', 'gw-app-frontend-app');
