// Package keys 是 storage 的权限码唯一封装。
package keys

// 管理面权限码（act 与 rbac_apis 种子同源——12565）。
const (
	PermConfigsList  = "admin.storage.configs.list"
	PermConfigsWrite = "admin.storage.configs.write"
	PermRecordsList  = "admin.storage.records.list"
	PermCleanup      = "admin.storage.cleanup"
)

// client 面无权限码（authn=users JWT——上传凭证/complete 挂 authed 组）。
