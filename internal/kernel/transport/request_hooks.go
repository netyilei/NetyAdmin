// request_hooks.go：request Hook 六段接线（TR-05）。
// 边界：传输层关注点仍是固定中间件链；Hook 只承载跨模块横切扩展
// （租户解析/审计补充/指标等）——可短路段失败 → 统一信封拒绝，log-only 段
// 失败仅告警；AfterHandler/ResponseSent 禁写业务 DB（审计走 Sink）。
// Hooks = nil 时全部直通（零开销：不建 payload、不进 stage 分发）。
//
// 链位：
//   - Received：引擎全局链（body limit 后、鉴权前——与组级 AuthN 位次无关）。
//   - Authenticated/Before/After/Sent/Error：组级 stage mw（mwHandlerStageHooks）
//     挂在组链尾——AuthN 组级中间件（identity/users AuthMiddleware）位在其前，
//     principal 已注入 ctx（§7.3「principal 注入后」兑现）。嵌套组经 gin Group
//     继承父组实例并各持自持实例，请求期由「链内最深实例」唯一触发（父组实例
//     让位，防双触发）。
//   - Register（引擎根注册）路径由 wrapHandlerStages 同体夹点（runHandlerStages
//     收拢，G1-06）。
package transport

import (
	"fmt"
	"net/http"
	"reflect"
	"runtime"

	"github.com/gin-gonic/gin"
	"github.com/netyilei/netyadmin/internal/kernel/hooks"
	hookstransport "github.com/netyilei/netyadmin/internal/kernel/hooks/transport"
	"github.com/netyilei/netyadmin/internal/kernel/observability"
	"github.com/netyilei/netyadmin/internal/kernel/security/authn"
)

// mwRequestReceivedHooks 触发 RequestReceived（§7.3：request_id/body limit 后、
// 鉴权前；可短路——链位在 mwBodyLimit 之后）。
func (s *Server) mwRequestReceivedHooks() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.hooks == nil {
			c.Next()
			return
		}
		if err := s.hooks.RunStage(c.Request.Context(), hooks.StageRequestReceived, s.requestPayload(c)); err != nil {
			FailOf(c, err)
			return
		}
		c.Next()
	}
}

// ctxStageDepthKey 是 stage mw 同类实例的请求内执行序号键（1-based；
// 嵌套组浅→深递增——链去重状态，请求隔离）。
const ctxStageDepthKey = "na_hook_stage_mw_seq"

// handlerStageMW 是组级 stage 触发中间件本体（具名方法——同类实例以运行时
// 函数名识别，gin v1.12 Context.handlers 未导出、HandlerNames 为唯一链内
// 视野）。执行语义见 runHandlerStages。
func (s *Server) handlerStageMW(c *gin.Context) {
	// 本实例非链内最深同类（嵌套组自持份在其后）→ 让位：唯一触发点=最深
	// 实例（其位在子组 AuthN/幂等等中间件之后——principal 已注入）。
	if s.hasDeeperStageMW(c) {
		c.Next()
		return
	}
	s.runHandlerStages(c, c.Next)
}

// mwHandlerStageHooks 构造组级 stage 中间件（Hooks=nil 不挂载——零开销由
// Server.Group/stagedGroup.Group 的挂载条件保证，本函数不做运行时分支）。
func (s *Server) mwHandlerStageHooks() gin.HandlerFunc {
	return s.handlerStageMW
}

// stageMWNameFunc 计算 stage mw 方法值的运行时名（gin HandlerNames 对齐——
// 方法值名带 -fm 后缀，与方法表达式名不同；New 期计算一次缓存 Server 字段）。
func stageMWNameFunc(fn gin.HandlerFunc) string {
	return runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name()
}

// hasDeeperStageMW 判定本实例是否为请求链内最深同类 stage mw：HandlerNames
// 给出链内全部 handler 名（含自身），同类总数 vs 本实例执行序号——非末位
// 让位。单实例链 seq==total==1 → 触发；嵌套组浅实例让位、深实例触发。
func (s *Server) hasDeeperStageMW(c *gin.Context) bool {
	total := 0
	for _, n := range c.HandlerNames() {
		if n == s.stageMWName {
			total++
		}
	}
	return stageMWSeq(c) < total
}

// stageMWSeq 返回本请求内同类实例的执行序号（1-based）。
func stageMWSeq(c *gin.Context) int {
	v, _ := c.Get(ctxStageDepthKey)
	seq, _ := v.(int)
	seq++
	c.Set(ctxStageDepthKey, seq)
	return seq
}

// runHandlerStages 以 next（业务 handler 或剩余链）为夹点执行五段（§7.3 时机
// 语义；Received 段在引擎链 mwRequestReceivedHooks——鉴权前位置固定）：
//
//	Authenticated（可短路，principal 注入后——AuthN 组级位在本 mw 前）→
//	Before（可短路）→ next → After（log-only）→ [5xx/panic → Error] → Sent。
//
// 短路语义：Authenticated/Before 拒绝即终结——After/Sent 不触发（handler 相位
// 未进入；与 Received 拒绝同口径）。
// 已知近似：gin 在 handler 内写出响应，AfterHandler
// 实际触发点为 handler 返回后——「响应写入前」不可严格满足（log-only 段不可短路）；
// Secured 的 AuthZ 链（RequirePerm→AuthzMiddleware）在 Before 之后、handler 之前
// 执行（业务管线内，非组级链位）。
func (s *Server) runHandlerStages(c *gin.Context, next func()) {
	payload := s.requestPayload(c)
	if err := s.hooks.RunStage(c.Request.Context(), hooks.StageRequestAuthenticated, payload); err != nil {
		FailOf(c, err) // 可短路
		return
	}
	if err := s.hooks.RunStage(c.Request.Context(), hooks.StageBeforeHandler, payload); err != nil {
		FailOf(c, err) // 可短路
		return
	}
	defer func() {
		if r := recover(); r != nil {
			payload.Error = fmt.Errorf("panic: %v", r)
			s.hooks.RunStageAsync(c.Request.Context(), hooks.StageRequestError, payload)
			// Recovery（链首）将写出 10009——ResponseSent 按该状态补触发
			//（code-audit P2 实证：panic 后 ResponseSent 缺席）
			payload.Status = http.StatusInternalServerError
			s.hooks.RunStageAsync(c.Request.Context(), hooks.StageResponseSent, payload)
			panic(r) // re-panic：Recovery 统一转 10009 信封
		}
		payload.Status = c.Writer.Status()
		if err := s.hooks.RunStage(c.Request.Context(), hooks.StageAfterHandler, payload); err != nil {
			s.logger.Warn("transport: after_handler hooks", "err", err) // log-only 段
		}
		if payload.Status >= http.StatusInternalServerError {
			s.hooks.RunStageAsync(c.Request.Context(), hooks.StageRequestError, payload)
		}
		s.hooks.RunStageAsync(c.Request.Context(), hooks.StageResponseSent, payload)
	}()
	next()
}

// wrapHandlerStages 在业务 handler 前后精确夹住 Authenticated/Before/After/
// ResponseSent/RequestError（引擎根 Register 路径；组级路径走 mwHandlerStageHooks
// ——同体 runHandlerStages）。健康/指标端点不经业务管线不触发。
func (s *Server) wrapHandlerStages(handler gin.HandlerFunc) gin.HandlerFunc {
	if s.hooks == nil {
		return handler
	}
	return func(c *gin.Context) {
		s.runHandlerStages(c, func() { handler(c) })
	}
}

// requestPayload 构造六段共享 payload（transport 层翻译：HTTP 请求 → 结构化字段）。
// Received 段独立快照（鉴权前 principal 必缺席）；Authenticated 起与 handler 段
// 共享实例（AuthN 已注入 ctx——principal 携带；Status 在 handler 相位填充）。
func (s *Server) requestPayload(c *gin.Context) *hookstransport.RequestContext {
	p := &hookstransport.RequestContext{
		Method:    c.Request.Method,
		Path:      c.Request.URL.Path,
		RequestID: c.GetString("request_id"),
		TraceID:   observability.TraceIDFrom(c.Request.Context()),
	}
	if pr, ok := authn.PrincipalFrom(c.Request.Context()); ok {
		p.Principal = &pr
	}
	return p
}
