-- +goose Up
-- admin_roles seed（11054 配对；bootstrap admin 绑定由 cmd 迁移命令 Go 侧写入
-- ——首启账户 ID 运行期生成，静态种子无法预知）。

-- +goose Down
SELECT 1;
