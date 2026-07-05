// Package mask 提供敏感字段统一脱敏列表，供 operation_log、open_platform_auth 等中间件引用。
package mask

// SensitiveFieldKeys 是需要在审计日志/操作日志中脱敏的字段名集合（小写）。
// 这些字段对应的值在记录到 operation_log 等审计位置时必须被移除。
var SensitiveFieldKeys = []string{
	"password",
	"oldpassword",
	"newpassword",
	"confirmpassword",
	"secretkey",
	"accesskeysecret",
	"token",
	"refreshtoken",
	"accesstoken",
	"appsecret",
	"signature",
	"captchacode",
	"captchakey",
	"verifycode",
}
