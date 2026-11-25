// perm.go：audit 管理端权限码唯一封装。
// 契约：路由经 transport.RequirePerm 声明（Route policy），
// rbac_apis.act 登记同值（super-admin 通配行除外）——判定面与登记面单一语义。
package keys

// 审计日志权限码（无 delete 面——logging.md §3 追加写不可篡改）。
const (
	PermLogsList   = "admin.logs.list"
	PermLogsGet    = "admin.logs.get"
	PermLogsExport = "admin.logs.export"
)
