// assembly.go：装配清单与接线面单一入口——模块实例注册/审计中间件/
// 各接入面路由构造入口。下游新增模块：实现 kernel/module 契约后在 registerModules
// 追加一行实例（两处装配语义见 docs/guide/tutorial-first-module.md Step 7）。
package main

import (
	"github.com/gin-gonic/gin"
	"github.com/netyilei/netyadmin/internal/kernel/binding"
	"github.com/netyilei/netyadmin/internal/kernel/bus"
	"github.com/netyilei/netyadmin/internal/kernel/config"
	"github.com/netyilei/netyadmin/internal/kernel/errorx"
	klog "github.com/netyilei/netyadmin/internal/kernel/log"
	"github.com/netyilei/netyadmin/internal/kernel/module"
	"github.com/netyilei/netyadmin/internal/kernel/observability"
	"github.com/netyilei/netyadmin/internal/kernel/security/authn"
	"github.com/netyilei/netyadmin/internal/kernel/security/authz"
	"github.com/netyilei/netyadmin/internal/kernel/security/keys"
	"github.com/netyilei/netyadmin/internal/kernel/security/primitives"
	"github.com/netyilei/netyadmin/internal/kernel/state/coordination"
	"github.com/netyilei/netyadmin/internal/kernel/state/session"
	"github.com/netyilei/netyadmin/internal/kernel/transport"
	"github.com/netyilei/netyadmin/internal/modules/audit"
	auditapp "github.com/netyilei/netyadmin/internal/modules/audit/application"
	auditdomain "github.com/netyilei/netyadmin/internal/modules/audit/domain"
	"github.com/netyilei/netyadmin/internal/modules/configcenter"
	"github.com/netyilei/netyadmin/internal/modules/gw"
	"github.com/netyilei/netyadmin/internal/modules/identity"
	"github.com/netyilei/netyadmin/internal/modules/message"
	"github.com/netyilei/netyadmin/internal/modules/message/application"
	"github.com/netyilei/netyadmin/internal/modules/rbac"
	"github.com/netyilei/netyadmin/internal/modules/realtime"
	"github.com/netyilei/netyadmin/internal/modules/storage"
	"github.com/netyilei/netyadmin/internal/modules/users"
	"log/slog"
)

// assembly.go：模块装配清单单一入口。
// 拆卸步骤 = 删模块目录 + 删下方对应 Register 行（两步，项目照常编译运行）。
// 自带模块：audit（审计）+ ipac（IP 访问控制）+
// configcenter（动态配置）+ gw（网关治理+入口）；users 演示模块
// 随 US-01 按此模式登记。

// registerModules 登记全部启用模块（Registry.Build 保证拓扑序与校验）。
// deps 为模块构造所需内核组件（bus reload 源/logger/keys/session Redis 地址/
// 协调面——realtime seq 计数与 App 维度限流消费/共享指标面——模块自注册指标）。
func registerModules(logger *slog.Logger, logMgr *klog.Manager, busInst *bus.Bus, registry *module.Registry,
	cfg *config.Config, signingKeys *keys.SigningKeys, redisAddr string, topology session.Topology,
	gwSecretKey []byte, namespaceRoot string, coord *coordination.Coordination,
	metrics *observability.Metrics) (*identity.Module, *rbac.Module, *audit.Module, *configcenter.Module, *gw.Module, *realtime.Module, *message.Module, *users.Module, *storage.Module, error) {

	// identity JWT 签发器（keys.SigningKeys → application.Signer 适配）。
	signer := keysSigner{keys: signingKeys}

	reg := func(dir string, m module.Module) error {
		mf, err := module.LoadManifest(dir)
		if err != nil {
			return err
		}
		return registry.Register(m, mf)
	}
	auditMod := audit.New(logger.With("module", "audit"))
	if err := reg("internal/modules/audit", auditMod); err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	configMod := configcenter.New(logger.With("module", "configcenter"), busInst)
	if err := reg("internal/modules/configcenter", configMod); err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	gwMod, err := gw.New(gwSecretKey, logMgr, busInst)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	if err := reg("internal/modules/gw", gwMod); err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	rbacMod := rbac.New(logger.With("module", "rbac"), busInst)
	if err := reg("internal/modules/rbac", rbacMod); err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	realtimeMod := realtime.New(logger.With("module", "realtime"), coord, redisAddr,
		namespaceRoot, topology, cfg.Realtime) // 配置唯一源：kernel/config.Realtime
	if err := reg("internal/modules/realtime", realtimeMod); err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	var messageMod *message.Module
	if cfg.Message.Enabled { // ——false=零注册零开销（迁移恒登记，users 先例）
		messageMod = message.New(logMgr, application.Config{
			EmailDriver: cfg.Message.EmailDriver,
			SmsDriver:   cfg.Message.SmsDriver,
			SMTP: application.SMTPConfig{
				Host: cfg.Message.SMTP.Host, Port: cfg.Message.SMTP.Port,
				Username: cfg.Message.SMTP.Username, From: cfg.Message.SMTP.From,
				Password: cfg.Message.SMTPPassword.Value(),
			},
		}, metrics)
		if err := reg("internal/modules/message", messageMod); err != nil {
			return nil, nil, nil, nil, nil, nil, nil, nil, nil, err
		}
	}
	var storageMod *storage.Module
	if cfg.Storage.Enabled { //
		storageMod = storage.New(cfg.Storage.HMACSalt.Value(), cfg.Storage.CredentialsTTLSeconds, logMgr)
		if err := reg("internal/modules/storage", storageMod); err != nil {
			return nil, nil, nil, nil, nil, nil, nil, nil, nil, err
		}
	}
	var usersMod *users.Module
	if cfg.Users.Enabled { // 模块开关演示（US-01——false=零注册零开销）
		usersMod, err = users.New(signingKeys.Signer(), signingKeys.Verifiers(), logMgr, cfg.Users.BcryptCost)
		if err != nil {
			return nil, nil, nil, nil, nil, nil, nil, nil, nil, err
		}
		if err := reg("internal/modules/users", usersMod); err != nil {
			return nil, nil, nil, nil, nil, nil, nil, nil, nil, err
		}
	}
	identityMod := identity.New(logger.With("module", "identity"), redisAddr, topology, sessionNamespace(cfg), signer, busInst, signingKeys, metrics)
	if err := reg("internal/modules/identity", identityMod); err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	return identityMod, rbacMod, auditMod, configMod, gwMod, realtimeMod, messageMod, usersMod, storageMod, nil
}

// keysSigner 把 keys.SigningKeys 适配为 identity/application.Signer。
type keysSigner struct{ keys *keys.SigningKeys }

func (s keysSigner) Sign(claims map[string]any) (string, error) {
	return primitives.SignJWT_RS256(s.keys.Signer(), claims)
}

// —— 审计中间件 ——
// adminapi 审计中间件装配。
// 访问流水面（HTTP/WS）2026-11-19全文件化不进 DB：HTTP=transport_access.log
// （mwAccessLog）、WS=realtime_access.log（wsAuditHooks 见 access.go）；本文件仅承载
// 审计合规面（操作/authz deny/限流/RPC 服务调用——G2 管理操作审计写 DB 红线）。
// ① operationAudit：管理端变更（POST/PUT/PATCH/DELETE 成功）落 Type=1 操作审计
//
//	——产生方=接入面（有 request_id/IP/UA； 登录审计同语义），
//	cmd 是组合根（assembly 同层），直调 audit Service 不违反模块互禁（§2）。
//
// ② onDenyAudit：authz deny 落 Type=3 SecurityRecord（security.md §2 越权必记）。
// Result 语义：操作审计只记成功（<400）——失败请求由 authz deny/限流/错误信封
// 各面覆盖，不双记；Detail 带方法/权限码/HTTP 状态，BizCode 不解析响应体（留空）。

// mutatingMethods 是产生操作审计的变更方法集合。
var mutatingMethods = map[string]bool{
	"POST": true, "PUT": true, "PATCH": true, "DELETE": true,
}

// baseRecord 填充接入面公共字段（产生方契约：IP/UA/request_id/trace_id
// 全链同源——各审计记录面共用，防口径漂移）。
func baseRecord(c *gin.Context) auditdomain.Record {
	return auditdomain.Record{
		IP:        c.ClientIP(),
		UserAgent: c.Request.UserAgent(),
		RequestID: c.GetString("request_id"),
		TraceID:   traceIDOf(c),
	}
}

// resultOfStatus HTTP 状态 → 审计 Result 唯一映射（>=500 error；>=400 deny；
// 其余 success）。
func resultOfStatus(status int) string {
	switch {
	case status >= 500:
		return auditdomain.ResultError
	case status >= 400:
		return auditdomain.ResultDeny
	default:
		return auditdomain.ResultSuccess
	}
}

// operationAuditMiddleware 管理端变更操作审计（组级挂载；幂等旁路：无主体
// （公开端点）或非变更方法或非 2xx 不记）。
func operationAuditMiddleware(svc *auditapp.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if svc == nil || !mutatingMethods[c.Request.Method] {
			return
		}
		if c.Writer.Status() >= 400 {
			return // 失败请求不双记（deny/错误信封各面覆盖）
		}
		p := transport.PrincipalFrom(c)
		if p.ID == "" {
			return // 公开端点（login/refresh）无主体——登录审计由 identity 事件面负责
		}
		rec := baseRecord(c)
		rec.Type = auditdomain.TypeOperation
		rec.ActorType = auditdomain.ActorAdmin
		rec.ActorID = p.ID
		rec.Action = operationAction(c)
		rec.Resource = c.FullPath()
		rec.Result = auditdomain.ResultSuccess
		rec.Detail = map[string]any{
			"method":    c.Request.Method,
			"status":    c.Writer.Status(),
			"perm":      c.GetString(transport.CtxPermKey),
			"target_id": c.Param("id"),
		}
		svc.Write(c.Request.Context(), rec)
	}
}

// operationAction 取操作名（路由声明的权限码优先——与 rbac_apis.act 同语义；
// 未声明路由回退 method+path）。
func operationAction(c *gin.Context) string {
	if perm := c.GetString(transport.CtxPermKey); perm != "" {
		return perm
	}
	return c.Request.Method + " " + c.FullPath()
}

// onDenyAudit 构造 authz deny 审计回调（10106 越权 SecurityRecord）。
func onDenyAudit(svc *auditapp.Service) func(*gin.Context, authn.Principal, string, string) {
	return func(c *gin.Context, p authn.Principal, perm, reason string) {
		if svc == nil {
			return
		}
		rec := baseRecord(c)
		rec.Type = auditdomain.TypeSecurity
		rec.ActorType = auditdomain.ActorAdmin
		rec.ActorID = p.ID
		rec.Action = "authz.denied"
		rec.Resource = c.FullPath()
		rec.Result = auditdomain.ResultDeny
		rec.BizCode = errorx.CodePermissionDenied // 10106——状态码单一源（禁止字面量）
		rec.Detail = map[string]any{"perm": perm, "reason": reason}
		svc.Write(c.Request.Context(), rec)
	}
}

// onAccessAudit RPC 访问审计回调（AccessRecord 产生方=transport RPC；
// caller/callee/port/method/result 全落 Type=2——svc 经 getter 晚绑定）。
// RPC 响应无信封业务码——BizCode 留空，HTTP 传输态入 Detail.status。
func onAccessAudit(getSvc func() *auditapp.Service) func(*gin.Context, string, string, string, string, int) {
	return func(c *gin.Context, caller, module, port, method string, status int) {
		svc := getSvc()
		if svc == nil {
			return
		}
		rec := baseRecord(c)
		rec.Type = auditdomain.TypeAccess
		rec.ActorType = auditdomain.ActorService
		rec.ActorID = caller
		rec.Action = module + "." + port + "." + method
		rec.Resource = "/internal/v1/rpc/" + module + "/" + port + "/" + method
		rec.Result = resultOfStatus(status)
		rec.Detail = map[string]any{"status": status}
		svc.Write(c.Request.Context(), rec)
	}
}

// bindAuthorizer 是 rbac.Authorizer 端口的绑定分派（/；§13.3
// local/remote 同接口——config.binding 段驱动；未声明端口=local）。
func bindAuthorizer(services *module.Services, entries []config.BindingEntry,
	factory func(baseURLs []string) (authz.Authorizer, error)) (authz.Authorizer, error) {

	target := binding.Target{Port: "rbac.Authorizer"}
	for _, e := range entries {
		if e.Port == target.Port {
			target.BaseURLs = e.BaseURLs
			break
		}
	}
	return binding.Bind(services, target, factory)
}

// rateLimitAuditCallback 限流触发审计回调（security.md §2 限流触发必记；
// 兑现——admin_login 为首个挂载消费方）。
// svc 经 getter 延迟解引用（构造早于 db-connect 赋值）；denied=false=不可用降级
// （fail-closed 拒绝面同记）。resource=策略名+维度键（非路由面——限流发生在路由前）。
func rateLimitAuditCallback(getSvc func() *auditapp.Service) func(*gin.Context, string, string, bool) {
	return func(c *gin.Context, policyName, dimKey string, denied bool) {
		svc := getSvc()
		if svc == nil {
			return
		}
		result := auditdomain.ResultDeny
		action := "ratelimit.denied"
		if !denied {
			action = "ratelimit.unavailable"
			result = auditdomain.ResultError
		}
		svc.Write(c.Request.Context(), auditdomain.Record{
			Type:      auditdomain.TypeSecurity,
			ActorType: auditdomain.ActorSystem,
			Action:    action,
			Resource:  policyName + ":" + dimKey,
			Result:    result,
			BizCode:   errorx.CodeTooManyRequests,
			IP:        c.ClientIP(),
			UserAgent: c.Request.UserAgent(),
			RequestID: c.GetString("request_id"),
			TraceID:   traceIDOf(c),
			Detail:    map[string]any{"policy": policyName, "dim_key": dimKey, "path": c.Request.URL.Path},
		})
	}
}

// traceIDOf 从请求 ctx 取 OTel trace（observability 注入面；空=未采样）。
func traceIDOf(c *gin.Context) string {
	return observability.TraceIDFrom(c.Request.Context())
}
