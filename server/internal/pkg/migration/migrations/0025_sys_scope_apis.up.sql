CREATE TABLE IF NOT EXISTS sys_scope_apis (
    id BIGSERIAL PRIMARY KEY,
    scope_id BIGINT NOT NULL,
    api_id BIGINT NOT NULL,
    deleted_at BIGINT DEFAULT 0,
    CONSTRAINT fk_scope_api_group FOREIGN KEY (scope_id) REFERENCES sys_app_scope_groups(id),
    CONSTRAINT fk_scope_api_api FOREIGN KEY (api_id) REFERENCES sys_open_apis(id)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_scope_api ON sys_scope_apis(scope_id, api_id) WHERE deleted_at = 0;

