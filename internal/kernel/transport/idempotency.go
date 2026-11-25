// idempotency.go：Idempotency-Key 中间件（TR-01；api-style §5）。
// 语义：客户端对非幂等写带 Idempotency-Key 头 → 首见占位放行、重放 10011 冲突；
// 响应 5xx 视为未提交 → 释放键允许重试（4xx/2xx 保持占用至 TTL）。
// 键作用域 = 路由模板 + 客户端键（防跨路由碰撞）。Coordination=nil 时直通
// （无 Redis 的部署不启用；登记触发点）。幂等键 fail-closed 语义由
// coordination 策略保证（contracts/state-policies.yaml idempotency 段）。
package transport

import (
	"context"
	"errors"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/netyilei/netyadmin/internal/kernel/errorx"
	"github.com/netyilei/netyadmin/internal/kernel/state/coordination"
)

// HeaderIdempotencyKey 是幂等键头（api-style §5）。
const HeaderIdempotencyKey = "Idempotency-Key"

// idempotencyTTL 是键占用窗口基线（覆盖重试窗口：reliability.md 重试 ≤3 次 +
// 客户端退避，5 分钟足够；显式声明不依赖默认值）。
const idempotencyTTL = 5 * time.Minute

// Idempotency 返回幂等键占位中间件（**组级挂载，authn 之后**——未认证请求
// 不造键占内存；L48 根修/方案 A 2026-11-19）。作用面 = 创建/
// 提交类 POST（api-style §5 字面；PUT/DELETE 天然幂等不占位）。
// Coordination=nil 直通（无 Redis 的部署不启用）。
func (s *Server) Idempotency() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.idempotency == nil || c.Request.Method != "POST" {
			c.Next()
			return
		}
		key := c.GetHeader(HeaderIdempotencyKey)
		scope := c.FullPath()
		if key == "" || scope == "" { // 无键 / 未匹配路由不进占位
			c.Next()
			return
		}
		scoped := scope + ":" + key
		if err := s.idempotency.BeginIdempotency(c.Request.Context(), scoped, idempotencyTTL); err != nil {
			switch {
			case errors.Is(err, coordination.ErrDenied):
				Fail(c, errorx.New(errorx.CodeIdempotencyConflict))
			case errors.Is(err, coordination.ErrUnavailable):
				Fail(c, errorx.New(errorx.CodeUnavailable)) // multi fail-closed → 10010
			default:
				FailOf(c, err)
			}
			return
		}
		// 5xx/panic = 未提交：释放允许重试（reliability.md 只重试幂等操作——键已
		// 释放即客户端可安全重放）；4xx/2xx 保持占用。WithoutCancel：处理链收尾
		// 阶段请求 ctx 可能已取消，释放动作必须完成。
		defer func() {
			release := c.Writer.Status() >= 500
			if r := recover(); r != nil {
				release = true
				defer func() { panic(r) }() // re-panic：Recovery 统一兜底
			}
			if !release {
				return
			}
			if err := s.idempotency.ReleaseIdempotency(context.WithoutCancel(c.Request.Context()), scoped); err != nil {
				s.logger.Warn("transport: idempotency key release failed", "key", scoped, "err", err)
			}
		}()
		c.Next()
	}
}
