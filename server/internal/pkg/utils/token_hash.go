package utils

import (
	"crypto/sha256"
	"encoding/hex"
)

// TokenHash 计算访问令牌的 SHA256 十六进制哈希。
//
// 用途：用户/管理员登录时把 token 哈希写入会话存储（TokenStore），
// 中间件解析 token 后用同一函数计算哈希并查询会话存储，
// 以支持改密/禁用/登出/Token 版本号失效后立即拉黑旧 token。
//
// 全项目唯一真相源（RULES.md §0.1：禁止重复实现）：
//   - service/user/user.go
//   - service/system/admin.go
//   - middleware/auth.go（admin / user 两路）
// 都必须调用本函数，禁止再各自实现一份 sha256。
func TokenHash(token string) string {
	h := sha256.New()
	h.Write([]byte(token))
	return hex.EncodeToString(h.Sum(nil))
}
