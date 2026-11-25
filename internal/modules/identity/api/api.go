// Package api 是 identity 模块的跨模块端口（管理员身份只读面；
// 禁止 import application/domain）。
package api

import "context"

// Identity 是管理员身份只读端口（注册名 `identity.Service`）：
//
//	id, err := module.ResolveAs[identityapi.Identity](registry.Services, "identity.Service")
//	profile, err := id.Profile(ctx, adminID)
//
// 创建/改密/启停属管理端职责，不经业务模块。
type Identity interface {
	// Profile 按管理员 ID 取资料（不存在返回错误）。
	Profile(ctx context.Context, adminID string) (Profile, error)
	// ListAdmins 分页列出管理员（操作人展示等场景）。
	ListAdmins(ctx context.Context, offset, limit int) ([]Profile, int64, error)
}

// Profile 是管理员资料（Email/Phone 属 PII，调用方按需脱敏展示）。
type Profile struct {
	ID        string
	Username  string
	Name      string
	Email     string
	Phone     string
	AvatarURL string
	Status    int8 // 1=enabled / 2=disabled
}
