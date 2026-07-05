// Package middleware 提供 HTTP 中间件。
//
// security_headers.go 设置一组安全响应头，缓解常见的 Web 攻击向量：
//   - X-Content-Type-Options: nosniff      —— 防 MIME 嗅探
//   - X-Frame-Options: DENY                —— 防点击劫持（同源 deny）
//   - Referrer-Policy: strict-origin-when-cross-origin
//   - Strict-Transport-Security            —— HSTS（仅 HTTPS 时下发，强制 HTTPS）
//   - Content-Security-Policy              —— CSP（可配置，默认 'self'）
//   - Permissions-Policy                   —— 禁用敏感浏览器能力
//
// 已移除已废弃的 X-XSS-Protection 头：现代浏览器（Chrome 78+ / Edge / Firefox）
// 已弃用并移除该过滤器，启用反而可能引入 XSS 攻击面（参见
// https://developer.mozilla.org/docs/Web/HTTP/Headers/X-XSS-Protection）。
// 防御 XSS 应使用 Content-Security-Policy。
package middleware

import "github.com/gin-gonic/gin"

// 安全响应头常量（集中可审查）。
const (
	// hstsHeaderValue 在 HTTPS 下下发的 HSTS 头，强制浏览器 1 年内使用 HTTPS。
	// includeSubDomains 表示覆盖所有子域名（生产环境务必确认证书覆盖范围）。
	hstsHeaderValue = "max-age=31536000; includeSubDomains"

	// permissionsPolicyHeaderValue 禁用敏感浏览器能力：
	//   - camera / microphone：禁用摄像头与麦克风
	//   - geolocation：禁用地理位置
	permissionsPolicyHeaderValue = "camera=(), microphone=(), geolocation=()"
)

// SecurityHeaders 安全响应头中间件。
//
// csp 为 Content-Security-Policy 头的内容。为空字符串时不设置该头
// （保持兼容性，允许运维按需关闭 CSP）。
//
// Strict-Transport-Security 仅在 HTTPS 请求（c.Request.TLS != nil）时下发，
// 避免在 HTTP 降级场景下被中间人拦截后变成不可逆的 HSTS 锁定。
// 若部署在反向代理后由代理终止 TLS，需确保代理正确转发 X-Forwarded-Proto
// 并由 Gin 信任该代理（参见 [server].trusted_proxies）。
func SecurityHeaders(csp string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

		// HSTS 仅在 HTTPS 场景下发：
		// - c.Request.TLS != nil：直接 TLS 监听（cfg.TLS.Enable=true）
		// - X-Forwarded-Proto == https：Nginx/反向代理终止 TLS 时透传
		// 注意：X-Forwarded-Proto 可信前提是 cfg.Server.TrustedProxies 配置了真实代理 CIDR，
		// 否则攻击者可伪造该头绕过判断。TrustedProxies 默认为空（fail-closed）。
		if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
			c.Header("Strict-Transport-Security", hstsHeaderValue)
		}

		// CSP：可配置，为空时不设置（不强制）
		if csp != "" {
			c.Header("Content-Security-Policy", csp)
		}

		c.Header("Permissions-Policy", permissionsPolicyHeaderValue)
		c.Next()
	}
}
