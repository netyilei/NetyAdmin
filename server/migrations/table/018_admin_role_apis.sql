BEGIN;

-- 角色API关联表
CREATE TABLE IF NOT EXISTS admin_role_apis (
    admin_role_id BIGINT NOT NULL,
    admin_api_id BIGINT NOT NULL,
    PRIMARY KEY (admin_role_id, admin_api_id)
);

COMMIT;