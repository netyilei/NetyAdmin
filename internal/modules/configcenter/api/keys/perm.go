// perm.go：configcenter 管理端权限码唯一封装。
// 契约：路由经 transport.RequirePerm 声明（Route policy），
// rbac_apis.act 登记同值（super-admin 通配行除外）——判定面与登记面单一语义。
package keys

// 动态配置权限码（public 读面免鉴权——无对应权限码；is_public 置位走 update）。
const (
	PermConfigsList     = "admin.configs.list"
	PermConfigsUpdate   = "admin.configs.update"
	PermConfigsRollback = "admin.configs.rollback" // 配置回滚权限（管理页消费）
)
