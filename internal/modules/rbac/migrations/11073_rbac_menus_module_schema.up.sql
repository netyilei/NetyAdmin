-- +goose Up
-- 菜单所属模块（空=常显）：模块未启用时 RouteTree 过滤该菜单，消除死菜单
-- （模块开关 message/storage/users.enabled 与菜单可见性对齐）。
-- expand 安全：带默认值的加列，旧行自动 ''。
ALTER TABLE rbac_menus ADD COLUMN module text NOT NULL DEFAULT '';
