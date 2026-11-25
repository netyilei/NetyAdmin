// repo.go：RBAC 存储适配——策略行装载（Casbin loader 数据源）+ 管理面 CRUD +
// 授权绑定全量替换（单事务）。SQL 一律参数化、表名常量；只读 loader 按启用
// 状态过滤（禁用角色/API 不出策略）。
package adapters

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/jackc/pgx/v5"

	"github.com/netyilei/netyadmin/internal/kernel/database"
	"github.com/netyilei/netyadmin/internal/modules/rbac/domain"
)

// Repo 实现 domain.Repo（pool 构造期注入）。
type Repo struct {
	pool *database.Pool
}

// NewRepo 创建。
func NewRepo(pool *database.Pool) *Repo { return &Repo{pool: pool} }

// LoadPolicyRows 装载全部启用策略行：
//
//	p = 启用角色 × role_apis × 启用 API（role code → path/act）
//	g = admin_roles × 启用角色（role code → admin ID）
func (r *Repo) LoadPolicyRows(ctx context.Context) ([]domain.PolicyRow, error) {
	pRows, err := r.pool.Query(ctx, `
		SELECT ro.code, a.path, a.act
		FROM rbac_role_apis ra
		JOIN rbac_roles ro ON ro.id = ra.role_id AND ro.status = 1
		JOIN rbac_apis a ON a.id = ra.api_id AND a.status = 1`)
	if err != nil {
		return nil, fmt.Errorf("rbac: load p rows: %w", err)
	}
	defer pRows.Close()

	out := make([]domain.PolicyRow, 0, 64)
	for pRows.Next() {
		var code, path, act string
		if err := pRows.Scan(&code, &path, &act); err != nil {
			return nil, fmt.Errorf("rbac: scan p row: %w", err)
		}
		out = append(out, domain.PolicyRow{"p", code, path, act})
	}
	if err := pRows.Err(); err != nil {
		return nil, fmt.Errorf("rbac: iterate p rows: %w", err)
	}

	// g 行语义 = g(user, role)：admin 拥有角色（casbin 惯例；g 可链式表达角色继承）
	gRows, err := r.pool.Query(ctx, `
		SELECT ar.admin_id, ro.code
		FROM rbac_admin_roles ar
		JOIN rbac_roles ro ON ro.id = ar.role_id AND ro.status = 1`)
	if err != nil {
		return nil, fmt.Errorf("rbac: load g rows: %w", err)
	}
	defer gRows.Close()
	for gRows.Next() {
		var adminID, code string
		if err := gRows.Scan(&adminID, &code); err != nil {
			return nil, fmt.Errorf("rbac: scan g row: %w", err)
		}
		out = append(out, domain.PolicyRow{"g", adminID, code})
	}
	if err := gRows.Err(); err != nil {
		return nil, fmt.Errorf("rbac: iterate g rows: %w", err)
	}
	return out, nil
}

// FindRoleIDByCode 按角色码取 ID（未启用角色也返回——绑定先行，启用后生效）。
func (r *Repo) FindRoleIDByCode(ctx context.Context, code string) (string, error) {
	var id string
	err := r.pool.QueryRow(ctx, `SELECT id FROM rbac_roles WHERE code = $1`, code).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", domain.ErrRoleNotFound
		}
		return "", fmt.Errorf("rbac: find role %s: %w", code, err)
	}
	return id, nil
}

// BindAdmin 绑定 admin↔角色（ON CONFLICT 幂等）。
func (r *Repo) BindAdmin(ctx context.Context, adminID, roleID string) error {
	if _, err := r.pool.Exec(ctx,
		`INSERT INTO rbac_admin_roles (admin_id, role_id) VALUES ($1, $2)
		ON CONFLICT (admin_id, role_id) DO NOTHING`, adminID, roleID); err != nil {
		return fmt.Errorf("rbac: bind admin: %w", err)
	}
	return nil
}

// HasAdminBinding 判定绑定存在。
func (r *Repo) HasAdminBinding(ctx context.Context, adminID, roleID string) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM rbac_admin_roles WHERE admin_id = $1 AND role_id = $2)`,
		adminID, roleID).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("rbac: has binding: %w", err)
	}
	return ok, nil
}

// —— 角色管理 ——

const roleCols = `id, code, name, status, remark, created_at, updated_at`

func scanRole(row pgx.Row) (domain.Role, error) {
	var v domain.Role
	var status int16
	err := row.Scan(&v.ID, &v.Code, &v.Name, &status, &v.Remark, &v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		return domain.Role{}, err
	}
	v.Status = domain.Status(status)
	return v, nil
}

// ListRoles 角色分页列表（过滤零值=不过滤；LIKE 输入已由 Service 层转义）。
func (r *Repo) ListRoles(ctx context.Context, f domain.RoleListFilter) ([]domain.Role, int64, error) {
	where := ` WHERE 1=1`
	var args []any
	if f.Code != "" {
		args = append(args, "%"+f.Code+"%")
		where += fmt.Sprintf(` AND code LIKE $%d`, len(args))
	}
	if f.Name != "" {
		args = append(args, "%"+f.Name+"%")
		where += fmt.Sprintf(` AND name LIKE $%d`, len(args))
	}
	if f.Status != 0 {
		args = append(args, int16(f.Status))
		where += fmt.Sprintf(` AND status = $%d`, len(args))
	}
	var total int64
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM rbac_roles`+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("rbac: count roles: %w", err)
	}
	q := `SELECT ` + roleCols + ` FROM rbac_roles` + where +
		` ORDER BY created_at DESC, id LIMIT $` + strconv.Itoa(len(args)+1) +
		` OFFSET $` + strconv.Itoa(len(args)+2)
	rows, err := r.pool.Query(ctx, q, append(args, f.Limit, f.Offset)...)
	if err != nil {
		return nil, 0, fmt.Errorf("rbac: list roles: %w", err)
	}
	defer rows.Close()
	out := make([]domain.Role, 0, f.Limit)
	for rows.Next() {
		v, err := scanRole(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("rbac: scan role: %w", err)
		}
		out = append(out, v)
	}
	return out, total, rows.Err()
}

func (r *Repo) GetRole(ctx context.Context, id string) (domain.Role, error) {
	v, err := scanRole(r.pool.QueryRow(ctx,
		`SELECT `+roleCols+` FROM rbac_roles WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Role{}, domain.ErrRoleNotFound
	}
	return v, err
}

func (r *Repo) InsertRole(ctx context.Context, v domain.Role) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO rbac_roles (id, code, name, status, remark) VALUES ($1, $2, $3, $4, $5)`,
		v.ID, v.Code, v.Name, int16(v.Status), v.Remark)
	if err != nil {
		return mapPgUnique(err, domain.ErrCodeTaken, "rbac: insert role")
	}
	return nil
}

func (r *Repo) UpdateRole(ctx context.Context, v domain.Role) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE rbac_roles SET code = $1, name = $2, status = $3, remark = $4, updated_at = now()
		WHERE id = $5`, v.Code, v.Name, int16(v.Status), v.Remark, v.ID)
	if err != nil {
		return mapPgUnique(err, domain.ErrCodeTaken, "rbac: update role")
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrRoleNotFound
	}
	return nil
}

// DeleteRole 删除角色 + 级联清理授权行（单事务；残留=loader 幽灵数据）。
func (r *Repo) DeleteRole(ctx context.Context, id string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("rbac: delete role tx: %w", err)
	}
	defer tx.Rollback(context.WithoutCancel(ctx)) //nolint:errcheck // 成功路径 Commit 覆盖
	for _, q := range []string{
		`DELETE FROM rbac_role_menus WHERE role_id = $1`,
		`DELETE FROM rbac_role_apis WHERE role_id = $1`,
		`DELETE FROM rbac_role_buttons WHERE role_id = $1`,
		`DELETE FROM rbac_admin_roles WHERE role_id = $1`,
		`DELETE FROM rbac_roles WHERE id = $1`,
	} {
		if _, err := tx.Exec(ctx, q, id); err != nil {
			return fmt.Errorf("rbac: delete role cascade: %w", err)
		}
	}
	return tx.Commit(ctx)
}

// —— 菜单管理 ——

const menuCols = `id, parent_id, name, path, component, icon, module, sort_order, hidden, status, created_at, updated_at`

func scanMenu(row pgx.Row) (domain.Menu, error) {
	var v domain.Menu
	var status int16
	err := row.Scan(&v.ID, &v.ParentID, &v.Name, &v.Path, &v.Component, &v.Icon, &v.Module, &v.SortOrder, &v.Hidden, &status, &v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		return domain.Menu{}, err
	}
	v.Status = domain.Status(status)
	return v, nil
}

func (r *Repo) ListMenus(ctx context.Context) ([]domain.Menu, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+menuCols+` FROM rbac_menus ORDER BY sort_order ASC, created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("rbac: list menus: %w", err)
	}
	defer rows.Close()
	out := make([]domain.Menu, 0, 32)
	for rows.Next() {
		v, err := scanMenu(rows)
		if err != nil {
			return nil, fmt.Errorf("rbac: scan menu: %w", err)
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r *Repo) GetMenu(ctx context.Context, id string) (domain.Menu, error) {
	v, err := scanMenu(r.pool.QueryRow(ctx,
		`SELECT `+menuCols+` FROM rbac_menus WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Menu{}, domain.ErrNotFound
	}
	return v, err
}

func (r *Repo) InsertMenu(ctx context.Context, v domain.Menu) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO rbac_menus (id, parent_id, name, path, component, icon, sort_order, hidden, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		v.ID, v.ParentID, v.Name, v.Path, v.Component, v.Icon, v.SortOrder, v.Hidden, int16(v.Status))
	if err != nil {
		return fmt.Errorf("rbac: insert menu: %w", err)
	}
	return nil
}

func (r *Repo) UpdateMenu(ctx context.Context, v domain.Menu) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE rbac_menus SET parent_id = $1, name = $2, path = $3, component = $4, icon = $5,
		sort_order = $6, hidden = $7, status = $8, updated_at = now() WHERE id = $9`,
		v.ParentID, v.Name, v.Path, v.Component, v.Icon, v.SortOrder, v.Hidden, int16(v.Status), v.ID)
	if err != nil {
		return fmt.Errorf("rbac: update menu: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// DeleteMenus 批删 + 级联清理 role_menus（单事务）。
func (r *Repo) DeleteMenus(ctx context.Context, ids []string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("rbac: delete menus tx: %w", err)
	}
	defer tx.Rollback(context.WithoutCancel(ctx)) //nolint:errcheck // 成功路径 Commit 覆盖
	if _, err := tx.Exec(ctx, `DELETE FROM rbac_role_menus WHERE menu_id = ANY($1)`, ids); err != nil {
		return fmt.Errorf("rbac: delete menu grants: %w", err)
	}
	tag, err := tx.Exec(ctx, `DELETE FROM rbac_menus WHERE id = ANY($1)`, ids)
	if err != nil {
		return fmt.Errorf("rbac: delete menus: %w", err)
	}
	if tag.RowsAffected() != int64(len(ids)) {
		return domain.ErrNotFound
	}
	return tx.Commit(ctx)
}

// —— API 登记 ——

const apiCols = `id, method, path, act, summary, status, created_at, updated_at`

func scanApi(row pgx.Row) (domain.Api, error) {
	var v domain.Api
	var status int16
	err := row.Scan(&v.ID, &v.Method, &v.Path, &v.Act, &v.Summary, &status, &v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		return domain.Api{}, err
	}
	v.Status = domain.Status(status)
	return v, nil
}

func (r *Repo) ListApis(ctx context.Context) ([]domain.Api, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+apiCols+` FROM rbac_apis ORDER BY path ASC, act ASC`)
	if err != nil {
		return nil, fmt.Errorf("rbac: list apis: %w", err)
	}
	defer rows.Close()
	out := make([]domain.Api, 0, 64)
	for rows.Next() {
		v, err := scanApi(rows)
		if err != nil {
			return nil, fmt.Errorf("rbac: scan api: %w", err)
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r *Repo) GetApi(ctx context.Context, id string) (domain.Api, error) {
	v, err := scanApi(r.pool.QueryRow(ctx,
		`SELECT `+apiCols+` FROM rbac_apis WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Api{}, domain.ErrNotFound
	}
	return v, err
}

func (r *Repo) InsertApi(ctx context.Context, v domain.Api) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO rbac_apis (id, method, path, act, summary, status) VALUES ($1, $2, $3, $4, $5, $6)`,
		v.ID, v.Method, v.Path, v.Act, v.Summary, int16(v.Status))
	if err != nil {
		return fmt.Errorf("rbac: insert api: %w", err)
	}
	return nil
}

func (r *Repo) UpdateApi(ctx context.Context, v domain.Api) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE rbac_apis SET method = $1, path = $2, act = $3, summary = $4, status = $5,
		updated_at = now() WHERE id = $6`,
		v.Method, v.Path, v.Act, v.Summary, int16(v.Status), v.ID)
	if err != nil {
		return fmt.Errorf("rbac: update api: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *Repo) DeleteApi(ctx context.Context, id string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("rbac: delete api tx: %w", err)
	}
	defer tx.Rollback(context.WithoutCancel(ctx)) //nolint:errcheck // 成功路径 Commit 覆盖
	if _, err := tx.Exec(ctx, `DELETE FROM rbac_role_apis WHERE api_id = $1`, id); err != nil {
		return fmt.Errorf("rbac: delete api grants: %w", err)
	}
	tag, err := tx.Exec(ctx, `DELETE FROM rbac_apis WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("rbac: delete api: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return tx.Commit(ctx)
}

// —— 按钮管理 ——

const buttonCols = `id, menu_id, code, name, created_at`

func scanButton(row pgx.Row) (domain.Button, error) {
	var v domain.Button
	err := row.Scan(&v.ID, &v.MenuID, &v.Code, &v.Name, &v.CreatedAt)
	return v, err
}

func (r *Repo) ListButtons(ctx context.Context, menuID string) ([]domain.Button, error) {
	q := `SELECT ` + buttonCols + ` FROM rbac_buttons`
	args := []any{}
	if menuID != "" {
		q += ` WHERE menu_id = $1`
		args = append(args, menuID)
	}
	q += ` ORDER BY created_at ASC`
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("rbac: list buttons: %w", err)
	}
	defer rows.Close()
	out := make([]domain.Button, 0, 32)
	for rows.Next() {
		v, err := scanButton(rows)
		if err != nil {
			return nil, fmt.Errorf("rbac: scan button: %w", err)
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r *Repo) InsertButton(ctx context.Context, v domain.Button) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO rbac_buttons (id, menu_id, code, name) VALUES ($1, $2, $3, $4)`,
		v.ID, v.MenuID, v.Code, v.Name)
	if err != nil {
		return fmt.Errorf("rbac: insert button: %w", err)
	}
	return nil
}

func (r *Repo) UpdateButton(ctx context.Context, v domain.Button) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE rbac_buttons SET menu_id = $1, code = $2, name = $3 WHERE id = $4`,
		v.MenuID, v.Code, v.Name, v.ID)
	if err != nil {
		return fmt.Errorf("rbac: update button: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *Repo) DeleteButton(ctx context.Context, id string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("rbac: delete button tx: %w", err)
	}
	defer tx.Rollback(context.WithoutCancel(ctx)) //nolint:errcheck // 成功路径 Commit 覆盖
	if _, err := tx.Exec(ctx, `DELETE FROM rbac_role_buttons WHERE button_id = $1`, id); err != nil {
		return fmt.Errorf("rbac: delete button grants: %w", err)
	}
	tag, err := tx.Exec(ctx, `DELETE FROM rbac_buttons WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("rbac: delete button: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return tx.Commit(ctx)
}

// —— 授权绑定（全量替换）——

func (r *Repo) RoleMenuIDs(ctx context.Context, roleID string) ([]string, error) {
	return r.queryStrings(ctx,
		`SELECT menu_id FROM rbac_role_menus WHERE role_id = $1 ORDER BY menu_id`, roleID, "rbac: role menu ids")
}

func (r *Repo) RoleApiIDs(ctx context.Context, roleID string) ([]string, error) {
	return r.queryStrings(ctx,
		`SELECT api_id FROM rbac_role_apis WHERE role_id = $1 ORDER BY api_id`, roleID, "rbac: role api ids")
}

func (r *Repo) RoleButtonIDs(ctx context.Context, roleID string) ([]string, error) {
	return r.queryStrings(ctx,
		`SELECT button_id FROM rbac_role_buttons WHERE role_id = $1 ORDER BY button_id`, roleID, "rbac: role button ids")
}

// SetRoleMenus 全量替换角色↔菜单授权（单事务 delete+insert；空集=清空）。
func (r *Repo) SetRoleMenus(ctx context.Context, roleID string, menuIDs []string) error {
	return r.setGrants(ctx, "rbac_role_menus", "menu_id", roleID, menuIDs, "rbac: set role menus")
}

// SetRoleApis 全量替换角色↔API 授权（Casbin p 策略数据源——调用方发布 reload）。
func (r *Repo) SetRoleApis(ctx context.Context, roleID string, apiIDs []string) error {
	return r.setGrants(ctx, "rbac_role_apis", "api_id", roleID, apiIDs, "rbac: set role apis")
}

// SetRoleButtons 全量替换角色↔按钮授权。
func (r *Repo) SetRoleButtons(ctx context.Context, roleID string, buttonIDs []string) error {
	return r.setGrants(ctx, "rbac_role_buttons", "button_id", roleID, buttonIDs, "rbac: set role buttons")
}

// setGrants 授权表全量替换（单事务；表/列名由调用方传入——本包内常量字面量，
// 非用户输入，无注入面）。
func (r *Repo) setGrants(ctx context.Context, table, col, roleID string, ids []string, label string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("%s tx: %w", label, err)
	}
	defer tx.Rollback(context.WithoutCancel(ctx)) //nolint:errcheck // 成功路径 Commit 覆盖
	if _, err := tx.Exec(ctx, `DELETE FROM `+table+` WHERE role_id = $1`, roleID); err != nil {
		return fmt.Errorf("%s clear: %w", label, err)
	}
	for _, id := range ids {
		if _, err := tx.Exec(ctx,
			`INSERT INTO `+table+` (role_id, `+col+`) VALUES ($1, $2)
			ON CONFLICT DO NOTHING`, roleID, id); err != nil {
			return fmt.Errorf("%s insert: %w", label, err)
		}
	}
	return tx.Commit(ctx)
}

// —— admin 角色绑定 ——

func (r *Repo) AdminRoleIDs(ctx context.Context, adminID string) ([]string, error) {
	return r.queryStrings(ctx,
		`SELECT role_id FROM rbac_admin_roles WHERE admin_id = $1 ORDER BY role_id`, adminID, "rbac: admin role ids")
}

// RoleExists 实现防呆存在性校验（unknown role 静默插入幽灵绑定——安-A3）。
func (r *Repo) RoleExists(ctx context.Context, roleID string) (bool, error) {
	var exists bool
	if err := r.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM rbac_roles WHERE id = $1)`, roleID).Scan(&exists); err != nil {
		return false, fmt.Errorf("rbac: role exists: %w", err)
	}
	return exists, nil
}

// UserPermissionActs 输出管理员角色可见权限码并集（role_apis→apis.act；去重
// 排序—— 前端按钮权限消费）。super-admin 全量分支在 application。
func (r *Repo) UserPermissionActs(ctx context.Context, adminID string) ([]string, error) {
	return r.queryStrings(ctx, `
		SELECT DISTINCT a.act
		FROM rbac_apis a
		JOIN rbac_role_apis ra ON ra.api_id = a.id
		JOIN rbac_admin_roles ar ON ar.role_id = ra.role_id
		WHERE ar.admin_id = $1 AND a.act <> '*' ORDER BY a.act`, adminID, "rbac: user permission acts")
}

// AllPermissionActs 输出全部登记权限码（super-admin 分支；通配行排除）。
func (r *Repo) AllPermissionActs(ctx context.Context) ([]string, error) {
	rows, err := r.pool.Query(ctx, `SELECT DISTINCT act FROM rbac_apis WHERE act <> '*' ORDER BY act`)
	if err != nil {
		return nil, fmt.Errorf("rbac: all permission acts: %w", err)
	}
	defer rows.Close()
	out := make([]string, 0, 32)
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, fmt.Errorf("rbac: all permission acts scan: %w", err)
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// HasAdminBinding 判定管理员是否绑定指定角色（super-admin 分支判定面）。

func (r *Repo) SetAdminRoles(ctx context.Context, adminID string, roleIDs []string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("rbac: set admin roles tx: %w", err)
	}
	defer tx.Rollback(context.WithoutCancel(ctx)) //nolint:errcheck // 成功路径 Commit 覆盖
	if _, err := tx.Exec(ctx, `DELETE FROM rbac_admin_roles WHERE admin_id = $1`, adminID); err != nil {
		return fmt.Errorf("rbac: set admin roles clear: %w", err)
	}
	for _, roleID := range roleIDs {
		if _, err := tx.Exec(ctx,
			`INSERT INTO rbac_admin_roles (admin_id, role_id) VALUES ($1, $2)
			ON CONFLICT DO NOTHING`, adminID, roleID); err != nil {
			return fmt.Errorf("rbac: set admin roles insert: %w", err)
		}
	}
	return tx.Commit(ctx)
}

// RouteTree 按管理员已绑角色输出可见菜单（role_menus 并集；排序输出平列表，
// 组树由调用方/前端处理）。
func (r *Repo) RouteTree(ctx context.Context, adminID string) ([]domain.Menu, error) {
	// menuCols 带表别名限定（JOIN 下 created_at 与 rbac_admin_roles 撞列）。
	menuAliased := "m.id, m.parent_id, m.name, m.path, m.component, m.icon, m.module, m.sort_order, m.hidden, m.status, m.created_at, m.updated_at"
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT `+menuAliased+`
		FROM rbac_menus m
		JOIN rbac_role_menus rm ON rm.menu_id = m.id
		JOIN rbac_admin_roles ar ON ar.role_id = rm.role_id
		WHERE ar.admin_id = $1 AND m.status = 1
		ORDER BY m.sort_order ASC, m.created_at ASC`, adminID)
	if err != nil {
		return nil, fmt.Errorf("rbac: route tree: %w", err)
	}
	defer rows.Close()
	out := make([]domain.Menu, 0, 16)
	for rows.Next() {
		v, err := scanMenu(rows)
		if err != nil {
			return nil, fmt.Errorf("rbac: scan route tree: %w", err)
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// —— 内部 ——

// queryStrings 单列字符串查询统一出口。
func (r *Repo) queryStrings(ctx context.Context, q, arg, label string) ([]string, error) {
	rows, err := r.pool.Query(ctx, q, arg)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", label, err)
	}
	defer rows.Close()
	out := make([]string, 0, 16)
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, fmt.Errorf("%s scan: %w", label, err)
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// mapPgUnique 唯一冲突 → 业务哨兵（管理面创建判定面），其余包装透传。
func mapPgUnique(err error, sentinel error, label string) error {
	if database.IsUniqueViolation(err) {
		return sentinel
	}
	return fmt.Errorf("%s: %w", label, err)
}
