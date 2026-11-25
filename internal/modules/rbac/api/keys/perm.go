// perm.go：rbac 管理端权限码唯一封装。
// 契约：路由经 transport.RequirePerm 声明（Route policy），
// rbac_apis.act 登记同值（super-admin 通配行除外）——判定面与登记面单一语义。
package keys

// 角色/授权分配权限码。
const (
	PermRolesList = "admin.roles.list"
	PermRolesGet  = "admin.roles.get"
	PermRolesCrea = "admin.roles.create"
	PermRolesUpd  = "admin.roles.update"
	PermRolesDel  = "admin.roles.delete"
	PermRolesBind = "admin.roles.bind" // 角色三面授权 + admin 角色分配写面
)

// 菜单权限码。
const (
	PermMenusList = "admin.menus.list"
	PermMenusGet  = "admin.menus.get"
	PermMenusCrea = "admin.menus.create"
	PermMenusUpd  = "admin.menus.update"
	PermMenusDel  = "admin.menus.delete"
)

// API 登记权限码。
const (
	PermApisList = "admin.apis.list"
	PermApisGet  = "admin.apis.get"
	PermApisCrea = "admin.apis.create"
	PermApisUpd  = "admin.apis.update"
	PermApisDel  = "admin.apis.delete"
)

// 按钮权限码。
const (
	PermButtonsList = "admin.buttons.list"
	PermButtonsCrea = "admin.buttons.create"
	PermButtonsUpd  = "admin.buttons.update"
	PermButtonsDel  = "admin.buttons.delete"
)
