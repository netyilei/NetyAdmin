BEGIN;

-- 角色按钮关联表
CREATE TABLE IF NOT EXISTS admin_role_buttons (
    admin_role_id BIGINT NOT NULL,
    admin_button_id BIGINT NOT NULL,
    PRIMARY KEY (admin_role_id, admin_button_id)
);

COMMIT;