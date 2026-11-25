// Package api 是 users 模块的跨模块端口（C 端用户只读面；
// 禁止 import application/domain）。
package api

import "context"

// Users 是 C 端用户只读端口（注册名 `users.Users`）：
//
//	udir, err := module.ResolveAs[usersapi.Users](registry.Services, "users.Users")
//	u, err := udir.Get(ctx, userID)
//
// 注册/登录/资料修改属用户自身操作面（client API），管理面 CRUD 由管理端承担。
type Users interface {
	// Profile 按登录主体 ID 取用户（等价 client 面 /users/me）。
	Profile(ctx context.Context, principalID string) (User, error)
	// Get 按用户 ID 取用户。
	Get(ctx context.Context, id string) (User, error)
	// TokenVersion 取用户令牌版本（会话吊销判定面）。
	TokenVersion(ctx context.Context, id string) (int64, error)
}

// User 是用户投影（Email/Phone 属 PII，调用方按需脱敏展示）。
type User struct {
	ID           string
	Username     string
	Email        string
	Phone        string
	Status       int16 // 1=enabled / 2=disabled
	TokenVersion int64
}
