package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// DefaultMaxRequestBodyBytes 是全局 HTTP 请求体上限（10 MiB）。
//
// 在中间件层统一限制，避免各 handler 在 ShouldBindJSON / io.ReadAll 时
// 无界读取请求体造成内存耗尽。业务文件上传走预签名直传（不经过 API），
// JSON 业务请求（含存储测试上传的文本内容）远小于该值。
const DefaultMaxRequestBodyBytes int64 = 10 << 20

// BodyLimit 对每个请求的 Body 施加读取上限（http.MaxBytesReader）。
//
// 超限时后续读取（ShouldBindJSON / io.ReadAll）返回 *http.MaxBytesError，
// 由各入口按自身响应语义处理（如开放平台验签返回 413）。
// maxBytes <= 0 时不启用限制（与项目其他"零值不启用"约定一致）。
func BodyLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if maxBytes > 0 && c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		}
		c.Next()
	}
}
