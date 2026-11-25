// envelope.go：统一信封渲染（api-style §4；字段权威定义 docs/rules/status-codes.md §8）。
package transport

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/netyilei/netyadmin/internal/kernel/errorx"
	"github.com/netyilei/netyadmin/internal/kernel/observability"
)

// OK 渲染成功信封（code=10000；HTTP 200）。
func OK(c *gin.Context, data any) {
	c.Header("X-Request-ID", headerReqID(c))
	c.JSON(http.StatusOK, errorx.NewEnvelope(data))
}

// ctxBizCodeKey 是失败信封 5 位业务码的 gin ctx 键（access 审计面读取；
// BizCode 契约=5 位业务码，HTTP 状态不入该字段）。
const ctxBizCodeKey = "biz_code"

// Fail 渲染失败信封（按 errorx 映射 HTTP 传输语义）。
func Fail(c *gin.Context, e *errorx.Error) {
	reqID := headerReqID(c)
	traceID := observability.TraceIDFrom(c.Request.Context())
	c.Header("X-Request-ID", reqID)
	c.Set(ctxBizCodeKey, e.Code()) // 审计面取业务码（access 记录 BizCode 契约）
	c.JSON(e.HTTPStatus(), errorx.NewErrorEnvelope(e, reqID, traceID))
	c.Abort()
}

// FailOf 任意错误 → 信封（非 errorx 归一 10009 不外泄底层细节）。
func FailOf(c *gin.Context, err error) {
	env := errorx.NewErrorEnvelopeOf(err, headerReqID(c), observability.TraceIDFrom(c.Request.Context()))
	c.Header("X-Request-ID", headerReqID(c))
	c.Set(ctxBizCodeKey, env.Code)
	c.JSON(errorx.HTTPStatusOf(env.Code), env)
	c.Abort()
}

func headerReqID(c *gin.Context) string {
	if v, ok := c.Get("request_id"); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return c.GetHeader("X-Request-ID")
}
