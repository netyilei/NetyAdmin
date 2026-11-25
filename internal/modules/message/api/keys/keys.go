// Package keys 是 message 的权限码/key 唯一封装。
package keys

// 管理面权限码（act 与 rbac_apis 种子同源——12560）。
const (
	PermTemplatesList  = "admin.message.templates.list"
	PermTemplatesWrite = "admin.message.templates.write"
	PermRecordsList    = "admin.message.records.list"
	PermSend           = "admin.message.send"
)

// 站内信。
const PermInboxSend = "admin.message.inbox.send"
