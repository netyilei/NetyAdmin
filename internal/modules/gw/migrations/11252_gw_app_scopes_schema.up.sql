-- +goose Up
-- gw_app_scopes：应用↔scope 授权（scope code 引用 gw_scope_groups.code）。
CREATE TABLE gw_app_scopes (
    app_id     text NOT NULL,
    scope      text NOT NULL,                  -- scope code（组 code 引用）
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT pk_gw_app_scopes PRIMARY KEY (app_id, scope)
);
-- +goose Down
DROP TABLE IF EXISTS gw_app_scopes;
