// perm.go：gw 管理端权限码唯一封装。
// 契约：路由经 transport.RequirePerm 声明（Route policy），
// rbac_apis.act 登记同值（super-admin 通配行除外）——判定面与登记面单一语义。
package keys

// 应用管理权限码（scope 授权并入 UpdateApp——bind 码随独立授权端点删除，
// 接口权限页）。
const (
	PermAppsList        = "admin.gw.apps.list"
	PermAppsCreate      = "admin.gw.apps.create"
	PermAppsUpdate      = "admin.gw.apps.update"
	PermAppsDelete      = "admin.gw.apps.delete"
	PermAppsResetSecret = "admin.gw.apps.reset_secret"
)

// API 登记权限码。
const (
	PermApisList   = "admin.gw.apis.list"
	PermApisCreate = "admin.gw.apis.create"
	PermApisUpdate = "admin.gw.apis.update"
	PermApisDelete = "admin.gw.apis.delete"
	PermApisBind   = "admin.gw.apis.bind"
)
