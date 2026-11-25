// perm.go：identity 管理端权限码唯一封装。
// 契约：路由经 transport.RequirePerm 声明（Route policy），
// rbac_apis.act 登记同值（super-admin 通配行除外）——判定面与登记面单一语义。
// 命名：<资源>.<动作>，资源段与路由对齐（/admin/v1/admins）。
package keys

// 管理员账号管理权限码。
const (
	PermAdminsList          = "admin.admins.list"
	PermAdminsGet           = "admin.admins.get"
	PermAdminsCreate        = "admin.admins.create"
	PermAdminsUpdate        = "admin.admins.update"
	PermAdminsStatus        = "admin.admins.status"
	PermAdminsResetPassword = "admin.admins.reset_password"
	PermAdminsDelete        = "admin.admins.delete"
)
