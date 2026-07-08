// Package mask 提供敏感字段统一脱敏列表与匹配工具，供 operation_log 中间件、
// config service、open_platform_auth 等模块引用。
//
// 本包是敏感字段脱敏的单一事实源（RULES.md §11.4）：
//   - 新增敏感字段只需修改 SensitiveFieldKeys 切片
//   - 禁止在 service / handler / middleware 本地维护一份字段列表
//   - 匹配通过 IsSensitive 函数，大小写不敏感 + 去除下划线/连字符
package mask

import "strings"

// MaskPlaceholder 是敏感字段脱敏后的占位值。
// config Upsert 检测到此占位值时保留 DB 旧值，防止前端回传脱敏值覆盖真实密码。
const MaskPlaceholder = "****"

// SensitiveFieldKeys 是需要在审计日志/操作日志/配置响应中脱敏的字段名集合。
// 列表内的 key 全部小写，匹配时通过 IsSensitive 做归一化（去下划线/连字符 + 小写）。
//
// 同时覆盖 camelCase（JSON 请求体字段，如 appSecret）与 snake_case（DB 配置键，
// 如 secret_key）两种命名风格，确保任何来源的敏感字段都能被统一识别。
var SensitiveFieldKeys = []string{
	// 密码类
	"password",
	"oldpassword",
	"newpassword",
	"confirmpassword",
	// 密钥类（camelCase + snake_case 双覆盖）
	"secretkey",
	"secretid",
	"accesskeysecret",
	"apikey",
	"privatekey",
	// 令牌类
	"token",
	"refreshtoken",
	"accesstoken",
	// 应用密钥
	"appsecret",
	// 签名
	"signature",
	// 验证码
	"captchacode",
	"captchakey",
	"verifycode",
}

// normalizedSet 是 SensitiveFieldKeys 的归一化集合，供 IsSensitive O(1) 查找。
var normalizedSet = func() map[string]struct{} {
	m := make(map[string]struct{}, len(SensitiveFieldKeys))
	for _, k := range SensitiveFieldKeys {
		m[normalizeKey(k)] = struct{}{}
	}
	return m
}()

// normalizeKey 将字段名归一化：小写 + 去除下划线与连字符。
// 使得 "secret_key" / "SecretKey" / "secret-key" / "secretkey" 统一匹配为 "secretkey"。
func normalizeKey(key string) string {
	s := strings.ToLower(key)
	s = strings.ReplaceAll(s, "_", "")
	s = strings.ReplaceAll(s, "-", "")
	return s
}

// IsSensitive 判断给定字段名是否为敏感字段（大小写不敏感，忽略下划线/连字符差异）。
// 供 config service、operation_log 中间件等模块引用，替代本地硬编码列表。
func IsSensitive(key string) bool {
	_, ok := normalizedSet[normalizeKey(key)]
	return ok
}
