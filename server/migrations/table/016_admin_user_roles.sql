BEGIN;

-- 用户角色关联表
CREATE TABLE IF NOT EXISTS admin_user_roles (
    admin_user_id BIGINT NOT NULL,
    admin_role_id BIGINT NOT NULL,
    PRIMARY KEY (admin_user_id, admin_role_id)
);

COMMIT;