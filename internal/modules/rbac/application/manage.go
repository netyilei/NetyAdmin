// manage.go：RBAC 管理面编排——角色/菜单/API/按钮 CRUD + 授权绑定 +
// admin 角色分配。p/g 数据源（角色状态、API 行、role_apis、admin_roles）变更后
// 发布 platform.rbac.changed → 各节点 Casbin reload（本进程订阅同步 reload，
// 跨节点 at-most-once 与 Bus 语义对齐；发布失败告警不回滚 DB——reload 恢复面=
// 下次变更/重启，判定侧 fail-closed 不会放大权限）。
// 展示面（menus/buttons/role_menus/role_buttons）不进 Casbin，不发布。
// super-admin 种子角色写保护（通配语义载体——改/禁/删即全权面漂移）。
package application

import (
	"context"
	"slices"

	"github.com/netyilei/netyadmin/internal/kernel/bus"
	"github.com/netyilei/netyadmin/internal/kernel/security/primitives"
	"github.com/netyilei/netyadmin/internal/modules/rbac/api/events"
	"github.com/netyilei/netyadmin/internal/modules/rbac/domain"
)

// publishPolicyChanged 发布策略变更广播（p/g 数据源变更后必调；失败仅告警）。
func (s *Service) publishPolicyChanged(ctx context.Context) {
	if s.bus == nil {
		return
	}
	// Broadcast 域（审计修：缺省 Local 不出节点——multi 下他节点 stale）
	if err := s.bus.Publish(ctx, bus.Message{Topic: events.TopicPolicyChanged}, bus.WithDomain(bus.DomainBroadcast)); err != nil {
		s.logger.Error("rbac: publish policy changed", "err", err)
	}
}

// ready 是存储接线守卫（repo 未 Attach 一律 ErrNotAttached——identity 同先例）。
func (s *Service) ready() error {
	if s.repo == nil {
		return ErrNotAttached
	}
	return nil
}

// EnsureNotSuperAdmin 角色写保护判定（ID 或 code 命中超管即拒）。
func EnsureNotSuperAdmin(id, code string) error {
	if id == domain.SuperAdminRoleID || code == domain.SuperAdminRoleCode {
		return domain.ErrSuperAdminProtected
	}
	return nil
}

// ---- 角色 ----

// ListRoles 分页列表（LIKE 转义收口）。
func (s *Service) ListRoles(ctx context.Context, f domain.RoleListFilter) ([]domain.Role, int64, error) {
	if err := s.ready(); err != nil {
		return nil, 0, err
	}
	f.Code = primitives.EscapeLike(f.Code)
	f.Name = primitives.EscapeLike(f.Name)
	return s.repo.ListRoles(ctx, f)
}

// GetRole 详情。
func (s *Service) GetRole(ctx context.Context, id string) (domain.Role, error) {
	if err := s.ready(); err != nil {
		return domain.Role{}, err
	}
	return s.repo.GetRole(ctx, id)
}

// CreateRole 创建（code 唯一——adapters 冲突映射 ErrCodeTaken）。
func (s *Service) CreateRole(ctx context.Context, r domain.Role) (domain.Role, error) {
	if err := s.ready(); err != nil {
		return domain.Role{}, err
	}
	r.ID = primitives.NewULID(primitives.SystemClock{})
	if r.Status == 0 {
		r.Status = domain.StatusEnabled
	}
	if err := s.repo.InsertRole(ctx, r); err != nil {
		return domain.Role{}, err
	}
	return s.repo.GetRole(ctx, r.ID)
}

// UpdateRole 更新（code/status 变更影响 loader 输出 → 发布 reload）。
func (s *Service) UpdateRole(ctx context.Context, r domain.Role) (domain.Role, error) {
	if err := s.ready(); err != nil {
		return domain.Role{}, err
	}
	if err := EnsureNotSuperAdmin(r.ID, ""); err != nil {
		return domain.Role{}, err
	}
	if err := s.repo.UpdateRole(ctx, r); err != nil {
		return domain.Role{}, err
	}
	s.publishPolicyChanged(ctx)
	return s.repo.GetRole(ctx, r.ID)
}

// DeleteRole 删除（级联清理授权行 → 发布 reload）。
func (s *Service) DeleteRole(ctx context.Context, id string) error {
	if s.repo == nil {
		return ErrNotAttached
	}
	if err := EnsureNotSuperAdmin(id, ""); err != nil {
		return err
	}
	if err := s.repo.DeleteRole(ctx, id); err != nil {
		return err
	}
	s.publishPolicyChanged(ctx)
	return nil
}

// ---- 菜单（展示面，不发布）----

// ListMenus 全量平列表（前端组树）。
func (s *Service) ListMenus(ctx context.Context) ([]domain.Menu, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	return s.repo.ListMenus(ctx)
}

// GetMenu 详情。
func (s *Service) GetMenu(ctx context.Context, id string) (domain.Menu, error) {
	if err := s.ready(); err != nil {
		return domain.Menu{}, err
	}
	return s.repo.GetMenu(ctx, id)
}

// CreateMenu 创建。
func (s *Service) CreateMenu(ctx context.Context, m domain.Menu) (domain.Menu, error) {
	if err := s.ready(); err != nil {
		return domain.Menu{}, err
	}
	m.ID = primitives.NewULID(primitives.SystemClock{})
	if m.Status == 0 {
		m.Status = domain.StatusEnabled
	}
	if err := s.repo.InsertMenu(ctx, m); err != nil {
		return domain.Menu{}, err
	}
	return s.repo.GetMenu(ctx, m.ID)
}

// UpdateMenu 更新。
func (s *Service) UpdateMenu(ctx context.Context, m domain.Menu) (domain.Menu, error) {
	if err := s.ready(); err != nil {
		return domain.Menu{}, err
	}
	if err := s.repo.UpdateMenu(ctx, m); err != nil {
		return domain.Menu{}, err
	}
	return s.repo.GetMenu(ctx, m.ID)
}

// DeleteMenus 批删（级联清 role_menus）。
func (s *Service) DeleteMenus(ctx context.Context, ids []string) error {
	if s.repo == nil {
		return ErrNotAttached
	}
	return s.repo.DeleteMenus(ctx, ids)
}

// ---- API 登记 ----

// ListApis 全量列表。
func (s *Service) ListApis(ctx context.Context) ([]domain.Api, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	return s.repo.ListApis(ctx)
}

// GetApi 详情。
func (s *Service) GetApi(ctx context.Context, id string) (domain.Api, error) {
	if err := s.ready(); err != nil {
		return domain.Api{}, err
	}
	return s.repo.GetApi(ctx, id)
}

// CreateApi 登记判定行（新行无授权授予 → 不发布；绑定后经 SetRoleApis 发布）。
func (s *Service) CreateApi(ctx context.Context, a domain.Api) (domain.Api, error) {
	if err := s.ready(); err != nil {
		return domain.Api{}, err
	}
	a.ID = primitives.NewULID(primitives.SystemClock{})
	if a.Status == 0 {
		a.Status = domain.StatusEnabled
	}
	if err := s.repo.InsertApi(ctx, a); err != nil {
		return domain.Api{}, err
	}
	return s.repo.GetApi(ctx, a.ID)
}

// UpdateApi 更新（path/act/status 影响 p 行 → 发布 reload）。
func (s *Service) UpdateApi(ctx context.Context, a domain.Api) (domain.Api, error) {
	if err := s.ready(); err != nil {
		return domain.Api{}, err
	}
	if err := s.repo.UpdateApi(ctx, a); err != nil {
		return domain.Api{}, err
	}
	s.publishPolicyChanged(ctx)
	return s.repo.GetApi(ctx, a.ID)
}

// DeleteApi 删除（级联清 role_apis → 发布 reload）。
func (s *Service) DeleteApi(ctx context.Context, id string) error {
	if s.repo == nil {
		return ErrNotAttached
	}
	if err := s.repo.DeleteApi(ctx, id); err != nil {
		return err
	}
	s.publishPolicyChanged(ctx)
	return nil
}

// ---- 按钮 ----

// ListButtons 按菜单过滤列表（空 menuID=全量）。
func (s *Service) ListButtons(ctx context.Context, menuID string) ([]domain.Button, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	return s.repo.ListButtons(ctx, menuID)
}

// CreateButton 创建。
func (s *Service) CreateButton(ctx context.Context, b domain.Button) (domain.Button, error) {
	if err := s.ready(); err != nil {
		return domain.Button{}, err
	}
	b.ID = primitives.NewULID(primitives.SystemClock{})
	if err := s.repo.InsertButton(ctx, b); err != nil {
		return domain.Button{}, err
	}
	return b, nil
}

// UpdateButton 更新。
func (s *Service) UpdateButton(ctx context.Context, b domain.Button) error {
	if s.repo == nil {
		return ErrNotAttached
	}
	return s.repo.UpdateButton(ctx, b)
}

// DeleteButton 删除（级联清 role_buttons）。
func (s *Service) DeleteButton(ctx context.Context, id string) error {
	if s.repo == nil {
		return ErrNotAttached
	}
	return s.repo.DeleteButton(ctx, id)
}

// ---- 授权绑定 ----

// RoleGrants 读角色三面授权 ID 集（前端树勾选回显）。
func (s *Service) RoleGrants(ctx context.Context, roleID string) (menus, apis, buttons []string, err error) {
	if err := s.ready(); err != nil {
		return nil, nil, nil, err
	}
	if menus, err = s.repo.RoleMenuIDs(ctx, roleID); err != nil {
		return nil, nil, nil, err
	}
	if apis, err = s.repo.RoleApiIDs(ctx, roleID); err != nil {
		return nil, nil, nil, err
	}
	buttons, err = s.repo.RoleButtonIDs(ctx, roleID)
	return menus, apis, buttons, err
}

// SetRoleMenus 替换菜单授权（展示面，不发布）。
func (s *Service) SetRoleMenus(ctx context.Context, roleID string, menuIDs []string) error {
	if s.repo == nil {
		return ErrNotAttached
	}
	return s.repo.SetRoleMenus(ctx, roleID, menuIDs)
}

// SetRoleApis 替换 API 授权（p 数据源 → 发布 reload）。
func (s *Service) SetRoleApis(ctx context.Context, roleID string, apiIDs []string) error {
	if s.repo == nil {
		return ErrNotAttached
	}
	if err := s.repo.SetRoleApis(ctx, roleID, apiIDs); err != nil {
		return err
	}
	s.publishPolicyChanged(ctx)
	return nil
}

// SetRoleButtons 替换按钮授权（展示面，不发布）。
func (s *Service) SetRoleButtons(ctx context.Context, roleID string, buttonIDs []string) error {
	if s.repo == nil {
		return ErrNotAttached
	}
	return s.repo.SetRoleButtons(ctx, roleID, buttonIDs)
}

// ---- admin 角色分配 ----

// AdminRoles 读管理员已绑角色。
func (s *Service) AdminRoles(ctx context.Context, adminID string) ([]string, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	return s.repo.AdminRoleIDs(ctx, adminID)
}

// SetAdminRoles 分配管理员角色（g 数据源 → 发布 reload；含 super-admin 绑定
// 移除——绑定属管理操作，操作者权限自担：有分配权即持有该面）。
// SetAdminRoles 全量替换管理员↔角色绑定（防呆三面——安-A3 清偿：
// ①未知 role 拒绝（原静默 INSERT 幽灵绑定）②操作者自操作拒绝（自锁面）
// ③super-admin 绑定摘除拒绝（bootstrap 全权面漂移=系统自锁））。
func (s *Service) SetAdminRoles(ctx context.Context, adminID, operatorID string, roleIDs []string) error {
	if s.repo == nil {
		return ErrNotAttached
	}
	if operatorID != "" && operatorID == adminID {
		return domain.ErrSelfModify
	}
	for _, id := range roleIDs {
		exists, err := s.repo.RoleExists(ctx, id)
		if err != nil {
			return err
		}
		if !exists {
			return domain.ErrRoleNotFound
		}
	}
	current, err := s.repo.AdminRoleIDs(ctx, adminID)
	if err != nil {
		return err
	}
	for _, cur := range current {
		if cur == domain.SuperAdminRoleID && !slices.Contains(roleIDs, cur) {
			return domain.ErrSuperAdminProtected
		}
	}
	if err := s.repo.SetAdminRoles(ctx, adminID, roleIDs); err != nil {
		return err
	}
	s.publishPolicyChanged(ctx)
	return nil
}

// RouteTree 管理员可见菜单：绑定 super-admin（通配主体）返回全部启用菜单，
// 否则 role_menus 并集（展示面，不发布）。
func (s *Service) RouteTree(ctx context.Context, adminID string) ([]domain.MenuNode, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	superID, err := s.repo.FindRoleIDByCode(ctx, domain.SuperAdminRoleCode)
	if err == nil { // super-admin 种子必在（12550）；查询失败走数据面
		bound, berr := s.repo.HasAdminBinding(ctx, adminID, superID)
		if berr == nil && bound {
			menus, lerr := s.repo.ListMenus(ctx)
			if lerr != nil {
				return nil, lerr
			}
			enabled := make([]domain.Menu, 0, len(menus))
			for _, m := range menus {
				if m.Status == domain.StatusEnabled {
					enabled = append(enabled, m)
				}
			}
			return pruneEmptyDirs(buildMenuTree(s.filterByModule(enabled))), nil
		}
	}
	flat, err := s.repo.RouteTree(ctx, adminID)
	if err != nil {
		return nil, err
	}
	return pruneEmptyDirs(buildMenuTree(s.filterByModule(flat))), nil
}

// filterByModule 过滤所属模块未启用的菜单（module 空=常显；说明见 domain.Menu）。
func (s *Service) filterByModule(menus []domain.Menu) []domain.Menu {
	if s.moduleEnabled == nil {
		return menus
	}
	out := make([]domain.Menu, 0, len(menus))
	for _, m := range menus {
		if m.Module == "" || s.moduleEnabled(m.Module) {
			out = append(out, m)
		}
	}
	return out
}

// pruneEmptyDirs 移除过滤后变空的目录节点（component 空 + 无子节点）——
// 模块关闭时其唯一子项被过滤掉，目录不应以空分组形式残留。
func pruneEmptyDirs(nodes []domain.MenuNode) []domain.MenuNode {
	out := make([]domain.MenuNode, 0, len(nodes))
	for _, n := range nodes {
		n.Children = pruneEmptyDirs(n.Children)
		if n.Component == "" && len(n.Children) == 0 {
			continue
		}
		out = append(out, n)
	}
	return out
}

// PermissionActs 当前管理员可见权限码集（前端按钮权限/指令消费——
// super-admin=全部登记 act，否则 role_apis 并集；自查面无权限码守卫）。
func (s *Service) PermissionActs(ctx context.Context, adminID string) ([]string, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	superID, err := s.repo.FindRoleIDByCode(ctx, domain.SuperAdminRoleCode)
	if err == nil {
		bound, berr := s.repo.HasAdminBinding(ctx, adminID, superID)
		if berr == nil && bound {
			return s.repo.AllPermissionActs(ctx)
		}
	}
	return s.repo.UserPermissionActs(ctx, adminID)
}

// buildMenuTree 平列表 → 树（parent_id 组装；孤儿/环防护—— 前端动态路由
// 消费 []domain.MenuNode）。递归实现：根=ParentID 空/父不存在/自身成环护栏。
func buildMenuTree(flat []domain.Menu) []domain.MenuNode {
	byID := make(map[string]bool, len(flat))
	for _, m := range flat {
		byID[m.ID] = true
	}
	taken := make(map[string]bool, len(flat))

	var take func(parentID string, depth int) []domain.MenuNode
	take = func(parentID string, depth int) []domain.MenuNode {
		var out []domain.MenuNode
		for i := range flat {
			m := flat[i]
			if taken[m.ID] || m.ParentID != parentID || m.ID == parentID {
				continue
			}
			taken[m.ID] = true
			node := domain.MenuNode{Menu: m}
			if depth < 8 { // 环护栏（数据损坏防炸）
				node.Children = take(m.ID, depth+1)
			}
			out = append(out, node)
		}
		return out
	}

	roots := take("", 0)
	// 孤儿挂根：父声明存在但集合中缺失（数据不一致防丢失）
	for i := range flat {
		m := flat[i]
		if taken[m.ID] || m.ParentID == "" || byID[m.ParentID] {
			continue
		}
		taken[m.ID] = true
		node := domain.MenuNode{Menu: m}
		if len(m.ParentID) > 0 {
			node.Children = take(m.ID, 1)
		}
		roots = append(roots, node)
	}
	return roots
}
