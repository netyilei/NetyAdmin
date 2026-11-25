// rbac.go：RBAC 领域模型与存储契约（RB-01 + 管理面）。
// 判定链：Casbin p 策略=role_apis×apis（role code → path/act），g 绑定=
// admin_roles（admin ID → role code）——**角色不进 token**：变更后 reload 即生效，
// 无需重登录（优于 token 携带角色的即时性）。判定主体=Principal.ID（admin ID）。
// 判定 act=权限码（路由经 transport.RequirePerm 声明，rbac_apis.act 登记同值；
// method 列是路由登记元数据，不进 Casbin 判定）。
// 展示面：menus/buttons 不进判定（前端树/按钮显隐）；apis 是唯一判定数据源。
package domain

import (
	"context"
	"errors"
	"time"
)

// RoleStatus 是角色/API 状态（rbac_roles/rbac_apis.status smallint 镜像）。
type Status int8

const (
	StatusEnabled  Status = 1
	StatusDisabled Status = 2
)

// 超管角色/通配 API 的种子固定 ID（12550552 seed 同值——bootstrap 绑定查找键）。
const (
	SuperAdminRoleID   = "rbac-role-super-admin"
	SuperAdminRoleCode = "super-admin"
	WildcardApiID      = "rbac-api-wildcard"
)

// PolicyRow 是一条 Casbin 策略行（loader 输出；persist.LoadPolicyArray 契约——
// 行首 ptype：["p", sub, obj, act] / ["g", role, adminID]）。
type PolicyRow = []string

// Role 是角色实体（管理面全字段；json 面向 HTTP 响应——api-style §2 camelCase）。
type Role struct {
	ID        string    `json:"id"`
	Code      string    `json:"code"`
	Name      string    `json:"name"`
	Status    Status    `json:"status"`
	Remark    string    `json:"remark"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Menu 是管理端菜单实体（前端树展示面；判定不消费）。
type Menu struct {
	ID        string `json:"id"`
	ParentID  string `json:"parentId"` // 空=根
	Name      string `json:"name"`
	Path      string `json:"path"`
	Component string `json:"component"` // 前端视图路径（Geeker 动态路由消费）
	Icon      string `json:"icon"`      // Element Plus 图标名
	// Module 是菜单所属模块（空=常显；非空时模块未启用则路由树不可见——
	// 与模块开关 message/storage/users.enabled 对齐，避免死菜单）。
	Module    string    `json:"module"`
	SortOrder int       `json:"sortOrder"`
	Hidden    bool      `json:"hidden"`
	Status    Status    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// MenuNode 是树化菜单节点（RouteTree 输出—— 前端动态路由消费）。
type MenuNode struct {
	Menu
	Children []MenuNode `json:"children,omitempty"`
}

// Api 是 API 判定登记实体（Casbin obj=path act=权限码；method=登记元数据）。
type Api struct {
	ID        string    `json:"id"`
	Method    string    `json:"method"`
	Path      string    `json:"path"`
	Act       string    `json:"act"`
	Summary   string    `json:"summary"`
	Status    Status    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Button 是按钮权限码实体（前端动作显隐；判定面仍走对应 API 行）。
type Button struct {
	ID        string    `json:"id"`
	MenuID    string    `json:"menuId"`
	Code      string    `json:"code"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

// RoleListFilter 是角色列表过滤（零值=不过滤）。
type RoleListFilter struct {
	Code   string
	Name   string
	Status Status
	Offset int
	Limit  int
}

// 领域错误。
var (
	// ErrRoleNotFound：角色不存在（绑定/查询面）。
	ErrRoleNotFound = errors.New("rbac: role not found")
	// ErrNotFound：通用无结果。
	ErrNotFound = errors.New("rbac: not found")
	// ErrAlreadyBound：绑定已存在（幂等写前的业务判定面）。
	ErrAlreadyBound = errors.New("rbac: already bound")
	// ErrCodeTaken：角色 code 唯一冲突（管理面创建）。
	ErrCodeTaken = errors.New("rbac: role code taken")
	// ErrSuperAdminProtected：超管角色写保护（通配语义载体——改/禁/删即全权面漂移）。
	ErrSuperAdminProtected = errors.New("rbac: super-admin role protected")

	// ErrSelfModify：操作者修改自身绑定（安-A3 防自锁——用其他
	// 管理员操作；bootstrap super-admin 摘除同受 ErrSuperAdminProtected 保护）。
	ErrSelfModify = errors.New("rbac: self modify forbidden")
)

// Repo 是 RBAC 存储契约（application 定义、adapters 实现——依赖向内）。
// 判定面（loader/绑定）与管理面（CRUD/授权绑定）共同构成 Service 全部存储需求。
type Repo interface {
	// —— 判定面（RB-01 loader/绑定）——

	// LoadPolicyRows 输出全部启用 Casbin 策略行（p = role_apis×apis×roles；
	// g = admin_roles×roles）。禁用角色/API 不输出。
	LoadPolicyRows(ctx context.Context) ([]PolicyRow, error)
	// FindRoleIDByCode 按角色码取 ID（bootstrap 绑定查找）。
	FindRoleIDByCode(ctx context.Context, code string) (string, error)
	// BindAdmin 绑定 admin↔角色（幂等 ON CONFLICT）。
	BindAdmin(ctx context.Context, adminID, roleID string) error
	// HasAdminBinding 判定绑定是否存在。
	HasAdminBinding(ctx context.Context, adminID, roleID string) (bool, error)

	// —— 角色管理——

	ListRoles(ctx context.Context, f RoleListFilter) ([]Role, int64, error)
	GetRole(ctx context.Context, id string) (Role, error)
	InsertRole(ctx context.Context, r Role) error
	UpdateRole(ctx context.Context, r Role) error
	// DeleteRole 删除角色并级联清理授权行（role_menus/role_apis/role_buttons/
	// admin_roles——单事务内，残留授权行会成为 loader 幽灵数据）。
	DeleteRole(ctx context.Context, id string) error

	// —— 菜单管理 ——

	ListMenus(ctx context.Context) ([]Menu, error)
	GetMenu(ctx context.Context, id string) (Menu, error)
	InsertMenu(ctx context.Context, m Menu) error
	UpdateMenu(ctx context.Context, m Menu) error
	// DeleteMenus 删除菜单并级联清理 role_menus（残留授权行=前端树幽灵数据）。
	DeleteMenus(ctx context.Context, ids []string) error

	// —— API 登记 ——

	ListApis(ctx context.Context) ([]Api, error)
	GetApi(ctx context.Context, id string) (Api, error)
	InsertApi(ctx context.Context, a Api) error
	UpdateApi(ctx context.Context, a Api) error
	DeleteApi(ctx context.Context, id string) error

	// —— 按钮管理 ——

	ListButtons(ctx context.Context, menuID string) ([]Button, error)
	InsertButton(ctx context.Context, b Button) error
	UpdateButton(ctx context.Context, b Button) error
	DeleteButton(ctx context.Context, id string) error

	// —— 授权绑定（全量替换语义——管理端树勾选保存）——

	// RoleMenuIDs/RoleApiIDs/RoleButtonIDs 读当前授权集（前端回显）。
	RoleMenuIDs(ctx context.Context, roleID string) ([]string, error)
	RoleApiIDs(ctx context.Context, roleID string) ([]string, error)
	RoleButtonIDs(ctx context.Context, roleID string) ([]string, error)
	// SetRoleMenus/SetRoleApis/SetRoleButtons 全量替换（单事务 delete+insert；
	// 空集=清空）。SetRoleApis 影响 Casbin p 策略——调用方发布 reload 事件。
	SetRoleMenus(ctx context.Context, roleID string, menuIDs []string) error
	SetRoleApis(ctx context.Context, roleID string, apiIDs []string) error
	SetRoleButtons(ctx context.Context, roleID string, buttonIDs []string) error

	// —— admin 角色绑定（判定面 g 行；管理员角色分配）——

	// AdminRoleIDs 读管理员已绑角色 ID 集。
	AdminRoleIDs(ctx context.Context, adminID string) ([]string, error)
	// SetAdminRoles 全量替换（单事务；影响 g 行——调用方发布 reload 事件）。
	RoleExists(ctx context.Context, roleID string) (bool, error)
	// UserPermissionActs 输出管理员角色权限码并集（按钮权限面）。
	UserPermissionActs(ctx context.Context, adminID string) ([]string, error)
	// AllPermissionActs 输出全部登记权限码（super-admin 分支）。
	AllPermissionActs(ctx context.Context) ([]string, error)
	SetAdminRoles(ctx context.Context, adminID string, roleIDs []string) error

	// RouteTree 按管理员已绑角色输出可见菜单（role_menus 并集；super-admin/
	// 通配语义由调用方先判——本方法只做数据面）。
	RouteTree(ctx context.Context, adminID string) ([]Menu, error)
}
