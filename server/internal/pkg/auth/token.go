// Package auth 提供会话令牌相关的公共工具，是 token 哈希与命名空间的唯一真相源。
//
// 设计原则（RULES.md §0.1）：禁止重复实现。
// service/user、service/system/admin、middleware 必须通过本包访问令牌工具。
package auth

import (
	"strconv"

	"NetyAdmin/internal/pkg/utils"
)

// adminNamespace 是 admin 在共享 tokenStore 中的 key 前缀。
// user 直接用 26 字符 ULID 作为 key，与 admin 前缀天然不冲突。
const adminNamespace = "a:"

// HashToken 返回 token 的 SHA256 十六进制哈希。
// 等价于 utils.TokenHash，封装一层语义化命名。
func HashToken(token string) string {
	return utils.TokenHash(token)
}

// AdminTokenKey 将 adminID（uint）转为 tokenStore 的 string 形式用户标识。
//
// 与 user 端的 ULID 通过 "a:" 前缀隔离，是该前缀的唯一生成入口。
// service 层与 middleware 层必须通过本函数获取 key，禁止散落拼接 "a:"。
func AdminTokenKey(adminID uint) string {
	return adminNamespace + strconv.FormatUint(uint64(adminID), 10)
}
