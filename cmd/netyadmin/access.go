// access.go：接入面接线（cmd 组合根，下游一般不改）——ops 面（pprof）/clientapi
// 门面（RouteRegistrar）/网关边缘分派+真转发/WS 三面 + configcenter 热更消费。
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/netyilei/netyadmin/internal/kernel/config"
	"github.com/netyilei/netyadmin/internal/kernel/errorx"
	"github.com/netyilei/netyadmin/internal/kernel/log"
	"github.com/netyilei/netyadmin/internal/kernel/module"
	"github.com/netyilei/netyadmin/internal/kernel/observability"
	"github.com/netyilei/netyadmin/internal/kernel/security/authn"
	"github.com/netyilei/netyadmin/internal/kernel/security/authz"
	"github.com/netyilei/netyadmin/internal/kernel/transport"
	ckeys "github.com/netyilei/netyadmin/internal/modules/configcenter/api/keys"
	ccapplication "github.com/netyilei/netyadmin/internal/modules/configcenter/application"
	"github.com/netyilei/netyadmin/internal/modules/gw/application"
	"github.com/netyilei/netyadmin/internal/modules/message"
	"github.com/netyilei/netyadmin/internal/modules/realtime"
	"github.com/netyilei/netyadmin/internal/modules/realtime/api/channels"
	"github.com/netyilei/netyadmin/internal/modules/realtime/api/protocol"
	"github.com/netyilei/netyadmin/internal/modules/realtime/session"
	"github.com/netyilei/netyadmin/internal/modules/storage"
	"github.com/netyilei/netyadmin/internal/modules/users"
	"net"
	"net/http"
	"net/http/httputil"
	"net/http/pprof"
	"net/netip"
	"net/url"
	"path"
	"strings"
	"time"
)

// —— ops 面 + clientapi 门面 + configcenter 热更消费 ——
// ops 接入面 + clientapi 门面 + configcenter 热更消费接线。
// ① OPS-01：/debug/pprof 挂载（config.server.ops_pprof 开关——默认关，生产
// fail-closed；/health 与 /metrics 不受开关影响——LB 探针面已交付）。
// ②：clientapi 门面（/client/v1 版本组 + echo typed 演示端点）+
// RouteRegistrar 扩展点（users 经 Registry Resolve 拿组自注册——：
// 路由注册权由业务模块自持；本文件是 cmd 组合根，import 业务模块仅用于组合
// 接线（门函数不依赖模块内部实现，不违反模块互禁 §2））。
// ③ 热更消费：configcenter Watcher OnChange → log 模块/类型级别映射
// （group=log 键 log:module.<name>/log:type.<typ>，值=级别或 "reset"——
// log.Manager 动态 API 已有；全局级别热更需 kernel 新 API，随消费方）。

// mountHTTPGates 是 role→HTTP 门判定（main.go ⑧.55 与
// role_test 共用——G1-06 单点）。
func mountHTTPGates(role string) bool {
	return role == "all" || role == "admin"
}

// mountClientGate 是 role→client 门判定（仅 all——C 端面随 admin 角色承载
// 与否在定；Spec：role_test 同义反复根修=判定单点化）。
func mountClientGate(role string) bool {
	return role == "all"
}

// registerOpsRoutes 挂载 ops 面（健康面已有；此处补 pprof 开关面）。
func registerOpsRoutes(srv *transport.Server, cfg *config.Config) error {
	if !cfg.Server.OpsPprof {
		return nil
	}
	dbg := srv.Group("/debug/pprof")
	dbg.GET("/", gin.WrapF(pprof.Index))
	dbg.GET("/cmdline", gin.WrapF(pprof.Cmdline))
	dbg.GET("/profile", gin.WrapF(pprof.Profile))
	dbg.GET("/symbol", gin.WrapF(pprof.Symbol))
	dbg.POST("/trace", gin.WrapF(pprof.Trace))
	// allocs/block/goroutine/heap/mutex/threadcreate 经标准 handler 前缀路由
	dbg.GET("/:name", gin.WrapH(http.HandlerFunc(pprof.Index)))
	return nil
}

// registerClientAPI 组装 clientapi 门面（RouteRegistrar 扩展点；
// client WS surface——ticket 签发面 fail-closed：client AuthN=User JWT
// Provider 随 US-01 接线（移交），接线前 POST /ws/tickets 恒 401、
// upgrade 无可用 ticket 恒 403——面存在而凭证面缺席=fail-closed 语义）。
// echo 是 typed 端点演示（链：request_id/tracing/信封全走 transport 管线）；
// users 自注册：Resolve("clientapi.RouteRegistrar") 拿 /client/v1 组开子组——
// clientapi 门不 import 业务模块。
// accessCtl=面级访问控制（——frontend-app 应用行 IP 组集判定+应用 quota
// 滑窗，authn 之前 IP 层最先判定； 根修：挂 root 组覆盖全部 client
// 路由——此前在 main 另建同路径组被丢弃，配置对流量零效力）；nil=不挂。
// 返回 surface 供生命周期管理（Start/Stop——nil=realtime disabled）。
func registerClientAPI(srv *transport.Server, registry *module.Registry,
	realtimeMod *realtime.Module, logMgr *log.Manager, metrics *observability.Metrics,
	accessCtl gin.HandlerFunc) (*session.Surface, error) {

	root := srv.Group("/client/v1")
	if accessCtl != nil {
		root.Use(accessCtl)
	}
	root.GET("/echo", echoHandler)
	// RouteRegistrar 先 Expose（users 自注册依赖——disabled 路径也要有）
	if err := registry.Services.Expose("clientapi.RouteRegistrar", root); err != nil {
		return nil, err
	}
	if realtimeMod != nil && realtimeMod.Cfg().Enabled && realtimeMod.Tickets() != nil {
		wsClient, err := realtimeMod.Surface(session.SurfaceOptions{
			Name: "client",
			// 浏览器形态 C 端客户端（H5/webview）Origin 白名单（realtime.client_origins；
			// 空=拒带 Origin 请求——App/SDK 等非浏览器客户端无 Origin 可直连）
			OriginPatterns: realtimeMod.Cfg().ClientOrigins,
			Auth:           clientChannelAuthorizer{},
			Hooks:          wsAuditHooks{log: logMgr},
			Metrics:        metrics,
			Logger:         logMgr.Logger("realtime", "app"),
			// X-Forwarded-Proto 兜底：realtime.require_https（默认 false；prod 由
			// config.normalize 强制 true——边缘终结语义：prod 明文 upgrade
			// 携 token 暴露，进程内拒绝；dev/staging 明文联调不拦且可显式开启）
			RequireHTTPS: realtimeMod.Cfg().RequireHTTPS,
		})
		if err != nil {
			return nil, fmt.Errorf("clientapi: ws surface: %w", err)
		}
		wsClient.Start(context.WithoutCancel(context.Background()))
		wsClient.MountUpgrade(root)
		// ticket 签发面由 users authed 组挂载（激活——main 接线）
		return wsClient, nil
	}
	return nil, nil
}

// echoHandler 回显请求元数据（演示端点； 管线全链验证面）。
func echoHandler(c *gin.Context) {
	transport.OK(c, gin.H{
		"method":     c.Request.Method,
		"path":       c.FullPath(),
		"request_id": c.GetString("request_id"),
		"route":      "clientapi.echo",
	})
}

// decodeGWKey 解码 gw App secret 加密钥（Secret base64 → 32 字节； fail-closed——
// 缺失/非法=启动失败，secret 加密面不可用不得静默降级明文存储）。
func decodeGWKey(cfg *config.Config) ([]byte, error) {
	if !cfg.GW.SecretKey.IsSet() {
		return nil, errors.New("gw: secret key not set (NETYADMIN_GW_SECRET_KEY)")
	}
	raw, err := base64.StdEncoding.DecodeString(cfg.GW.SecretKey.Value())
	if err != nil {
		return nil, fmt.Errorf("gw: secret key base64: %w", err)
	}
	if len(raw) != 32 {
		return nil, fmt.Errorf("gw: secret key must decode to 32 bytes, got %d", len(raw))
	}
	return raw, nil
}

// wireConfigHotReload 接线 configcenter 热更消费（首个真实消费者——log 级别）。
// 键契约：group=log；key=log:module.<name> / log:type.<typ>；值=debug|info|warn|error|reset。
// Watcher reload（Bus 广播/启动装载）后回调映射到 log.Manager 动态 API。
func wireConfigHotReload(w *ccapplication.Watcher, logMgr *log.Manager) {
	w.OnChange(func() {
		for skey, cfg := range w.Snapshot() {
			if cfg.GroupName != ckeys.GroupLog {
				continue
			}
			// 组内 key 契约：module:<name> / type:<typ>（keys.LogModuleKey/LogTypeKey）
			var err error
			switch {
			case strings.HasPrefix(skey[len(ckeys.GroupLog)+1:], "module:"):
				err = logMgr.SetModuleLevel(skey[len(ckeys.GroupLog)+1+len("module:"):], cfg.ConfigValue)
			case strings.HasPrefix(skey[len(ckeys.GroupLog)+1:], "type:"):
				err = logMgr.SetTypeLevel(skey[len(ckeys.GroupLog)+1+len("type:"):], cfg.ConfigValue)
			default:
				continue
			}
			if err != nil {
				logMgr.Logger("configcenter", "app").Warn("config hot reload skip", "key", skey, "err", err)
			}
		}
	})
}

// wireClientModuleRoutes 挂 client 业务模块路由（US-01 开关门单点；
// 「US-01 开关 e2e 断言」的可测面）。
//
// usersMod==nil（users.enabled=false）→ 零注册：/client/v1/users/* 与依赖 users
// 鉴权面的站内信/存储 client 路由整体缺席（404 统一信封由 transport NoRoute
// 承载——D9 组合死区防呆）。users 启用时：users 自注册 authed 子组，ws ticket/
// 站内信/存储挂同一组（realtime 关闭仅跳过 ticket 面，业务路由不受连带）。
func wireClientModuleRoutes(registrar gin.IRouter, usersMod *users.Module,
	messageMod *message.Module, storageMod *storage.Module,
	wsClient *session.Surface, rlUsers gin.HandlerFunc) error {
	if usersMod == nil {
		return nil
	}
	authed, err := usersMod.RegisterClientRoutes(registrar, rlUsers)
	if err != nil {
		return fmt.Errorf("users client routes: %w", err)
	}
	if wsClient != nil {
		wsClient.MountTicket(authed)
	}
	if messageMod != nil {
		messageMod.RegisterClientInboxRoutes(authed)
	}
	if storageMod != nil {
		storageMod.RegisterClientRoutes(authed)
	}
	return nil
}

// —— WS 三面接线 ——
// WS 面接线（cmd 组合根——与审计中间件同惯例：组合根
// 直调模块 Service/端口不违反模块互禁 §2）。三端 surface 机制在 realtime/session；
// surface 特化（ChannelAuthorizer）与连接级审计 hooks 在此桥接。

// adminChannelAuthorizer 桥 rbac.Authorizer（ws-protocol §2 admin 行）：
// admin:<id> 私有频道仅本人；topic:<name> 权限码 ws:channel:<name>（rbac 判定
// 链 Casbin role_apis×apis——策略变更 reload 即生效）；user:/app: 非本面拒绝。
type adminChannelAuthorizer struct{ authz authz.Authorizer }

// Authorize 实现 session.ChannelAuthorizer（订阅/投递/发布同权限码——V1 不细分
// 频道读写权限；系统错误 fail-closed 拒绝——authz 中间件同语义）。
func (a adminChannelAuthorizer) Authorize(ctx context.Context, p authn.Principal, channel string, action session.Action) error {
	kind, subject := channels.Classify(channel)
	switch kind {
	case channels.KindAdmin:
		if subject == p.ID {
			return nil
		}
		return session.ErrChannelDenied
	case channels.KindTopic:
		d, err := a.authz.Authorize(ctx, p, "ws:channel:"+subject, string(action))
		if err != nil {
			return session.ErrChannelDenied // Authorize 故障=拒绝（fail-closed）
		}
		if d.Allowed {
			return nil
		}
		return session.ErrChannelDenied
	default:
		return session.ErrChannelDenied
	}
}

// gwChannelAuthorizer 桥 App Scopes（ws-protocol §2 gateway 行：Scope→频道模式
// 匹配）：仅 app:<appID>:<name> 且 appID=Principal 本体；name 匹配 scope
// ws.channel.<pattern>（glob，* 通配段——path.Match 语义）。
type gwChannelAuthorizer struct{}

// Authorize 实现 session.ChannelAuthorizer。
func (gwChannelAuthorizer) Authorize(_ context.Context, p authn.Principal, channel string, _ session.Action) error {
	kind, subject := channels.Classify(channel)
	if kind != channels.KindApp || subject != p.ID {
		return session.ErrChannelDenied // user:/admin:/他人 app 频道一律拒绝
	}
	name := strings.TrimPrefix(channel, channels.App(p.ID, ""))
	for _, sc := range p.Scopes {
		if pattern, ok := strings.CutPrefix(sc, "ws.channel."); ok {
			if matched, err := path.Match(pattern, name); err == nil && matched {
				return nil
			}
		}
	}
	return session.ErrChannelDenied
}

// wsAuditHooks 桥 log.Manager（2026-11-19：WS 访问流水全文件化
// 不进 DB——logs/<date>/realtime_access.log（connect/disconnect 流水）+
// realtime_security.log（异常关闭——ws-protocol §10 安全记录面））。
type wsAuditHooks struct {
	log *log.Manager
}

// OnConnect 实现 session.Hooks。
func (h wsAuditHooks) OnConnect(_ context.Context, surface string, p authn.Principal, connID, ip string) {
	h.log.Logger("realtime", "access").Info("ws.connect",
		"surface", surface, "actor_type", string(p.Type), "actor_id", p.ID,
		"conn_id", connID, "ip", ip)
}

// OnDisconnect 实现 session.Hooks（code 非正常关闭（≠100001）→ security
// 文件——ws-protocol §10「异常关闭计入安全记录」）。
func (h wsAuditHooks) OnDisconnect(_ context.Context, surface string, p authn.Principal,
	connID, ip string, code int, _ string, chans []string) {
	attrs := []any{
		"surface", surface, "actor_type", string(p.Type), "actor_id", p.ID,
		"conn_id", connID, "ip", ip, "close_code", code,
	}
	if len(chans) > 0 {
		attrs = append(attrs, "channels", chans)
	}
	if code != protocol.CloseNormal && code != protocol.CloseGoingAway {
		h.log.Logger("realtime", "security").Warn("ws.abnormal_close", attrs...)
		return
	}
	h.log.Logger("realtime", "access").Info("ws.disconnect", attrs...)
}

// clientChannelAuthorizer 桥 client 面 ACL（ws-protocol §2 client 行）：
// user:<id> 私有频道仅本人；topic 公开白名单/app:/admin: 非本面拒绝——
// 公开频道白名单随 users 消费者（G1-15 无消费者不建）。
type clientChannelAuthorizer struct{}

// Authorize 实现 session.ChannelAuthorizer。
func (clientChannelAuthorizer) Authorize(_ context.Context, p authn.Principal, channel string, _ session.Action) error {
	kind, subject := channels.Classify(channel)
	if kind == channels.KindUser && subject == p.ID && p.ID != "" {
		return nil
	}
	return session.ErrChannelDenied
}

// forwardTarget 从监听地址推导自环转发目标（P2-H：--addr 绑具体网卡时
// 127.0.0.1 未必监听——转发全 503）：
//   - 通配监听（空 host / 0.0.0.0 /）= 全接口含环回 → 127.0.0.1:port；
//   - 具体 IP/主机名（--addr 绑定自身）→ 原样使用（本机网卡可达，环回未必）；
//   - 解析失败（无端口）→ 回退 127.0.0.1:httpPort。
func forwardTarget(bind string, httpPort int) string {
	host, port, err := net.SplitHostPort(bind)
	if err != nil || port == "" {
		return fmt.Sprintf("127.0.0.1:%d", httpPort)
	}
	if host == "" {
		return net.JoinHostPort("127.0.0.1", port)
	}
	if ip, perr := netip.ParseAddr(host); perr == nil && ip.IsUnspecified() {
		return net.JoinHostPort("127.0.0.1", port)
	}
	return net.JoinHostPort(host, port)
}

// gatewayForwarder 是业务转发面：
// 进程内反代（httputil.ReverseProxy → forwardTarget(bind)，自环同进程监听）。
// 选型=反代而非 gin engine 重入（ServeHTTP 直调），理由：
//  1. engine 重入=全引擎中间件链二次执行：request_id 重生成（内外两层追溯
//     断裂）、访问日志/指标/TR-05 hooks 各记两次、嵌套 timeout——审计与可
//     观测语义失真（优先级序：正确性 > 性能）。反代内层=真实 HTTP 请求，
//     clientapi 全管线（authn/幂等/审计/访问日志）恰好执行一次。
//  2. 单一 request_id 闭环：分派器注入 X-Request-ID → 内层 mwRequestID
//     「上游已生成→保留」→ 业务信封 request_id=网关层 ID 全链一致。
//  3. 零跨模块 import（转发走 HTTP 面——模块互禁 §2；禁 import users/message
//     内部）。
//
// 自环开销=一次同机 HTTP 往返（默认环回；--addr 绑具体网卡时走该网卡——edge
// 每请求已有 nonce/quota Redis IO，量级不敏感）；防自环：分派器拒绝 inner path
// /gateway/ 前缀（fail-closed）。
type gatewayForwarder struct {
	proxy *httputil.ReverseProxy
}

// newGatewayForwarder 构造自环反代（bind=实际监听地址——addrOf 产物；
// httpPort=cfg.Server.HTTPPort 回退位；目标推导见 forwardTarget）。
func newGatewayForwarder(bind string, httpPort int) *gatewayForwarder {
	rp := httputil.NewSingleHostReverseProxy(
		&url.URL{Scheme: "http", Host: forwardTarget(bind, httpPort)})
	// 自环失败（进程关闭/重启窗口）→ 503 信封（edge fail-closed 同惯例；
	// transport.Fail 面向 gin.Context——ErrorHandler 只有 http.ResponseWriter，
	// 信封手工渲染与 errorx 序列化同构）
	rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		e := errorx.New(errorx.CodeUnavailable, "reason", "dispatch upstream unavailable")
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(e.HTTPStatus())
		_ = json.NewEncoder(w).Encode(
			errorx.NewErrorEnvelope(e, r.Header.Get("X-Request-ID"), ""))
	}
	return &gatewayForwarder{proxy: rp}
}

// gatewayDispatcher 是 /gateway/v1 单一分派器（安-F9 根修：catch-all 收进
// 分派器，组内不再被 gin 通配冲突阻塞；WS/平台端点/业务 API 同器路由——
// tickets 走 HMAC 管线（AppKey+HMAC——ws-protocol §1），upgrade 免 HMAC
// （ticket 即一次性凭证），业务 API 走全管线+scope 白名单+真转发）。
func gatewayDispatcher(edgeAuth, apiGate gin.HandlerFunc, gwWS *session.Surface,
	gwStats *application.StatsService, fwd *gatewayForwarder) gin.HandlerFunc {
	return func(c *gin.Context) {
		pathName, method := c.Param("path"), c.Request.Method
		switch {
		case method == http.MethodGet && pathName == "/ws":
			if gwWS == nil {
				transport.Fail(c, errorx.New(errorx.CodeUnavailable)) // realtime disabled=WS fail-closed
				return
			}
			gwWS.UpgradeHandler()(c)
		case method == http.MethodPost && pathName == "/ws/tickets":
			if gwWS == nil {
				transport.Fail(c, errorx.New(errorx.CodeUnavailable))
				return
			}
			edgeAuth(c)
			if c.IsAborted() {
				return
			}
			gwWS.TicketHandler()(c)
		default:
			edgeAuth(c)
			if c.IsAborted() {
				return
			}
			// 真转发：HMAC 契约=客户端所见
			// 完整网关 path（edge.go buildCredential 读 URL.Path——重写必须在
			// edgeAuth 之后、apiGate 之前）；gw_apis 登记与转发目标=inner path
			// 原样（/gateway/v1/client/v1/echo → /client/v1/echo——12753 种子
			// 登记形态）。path.Clean 归一（含 ../ 穿越防御）。
			inner := path.Clean("/" + pathName)
			if inner == "/gateway" || strings.HasPrefix(inner, "/gateway/") {
				// 防自环 fail-closed：/gateway/* 不得再入网关（反代目标=本进程）
				transport.Fail(c, errorx.New(errorx.CodeGatewayAPINotRegistered,
					"reason", "gateway prefix not dispatchable"))
				return
			}
			c.Request.URL.Path = inner
			c.Request.URL.RawPath = ""
			apiGate(c)
			if c.IsAborted() {
				return
			}
			// App 身份注入（下游 client 路由可感知调用方应用——覆盖客户端自报
			// 值=edge 验证过的 appKey 本体）+ request_id 传播（内层 mwRequestID
			// 保留上游值——全链单一追溯 ID）。body 保留（edge 已重挂 NopCloser）、
			// method/query/其余头原样（ReverseProxy 语义）。注：外层访问流水按
			// 重写后 path 记录（mwAccessLog 完成时读 URL.Path；kernel/transport
			// 不可触碰——网关入口 path 以 gw_stats 流水 path 字段承载）。
			if p := transport.PrincipalFrom(c); p.ID != "" {
				c.Request.Header.Set("X-NA-App-Id", p.ID)
			}
			if rid := c.GetString("request_id"); rid != "" {
				c.Request.Header.Set("X-Request-ID", rid)
			}
			if fwd == nil {
				transport.Fail(c, errorx.New(errorx.CodeUnavailable, "reason", "dispatch not wired")) // 组合根必构造——防御面
				return
			}
			// 内部标记（P1-A）：内层 /client/v1 accessCtl 跳过面级重复判定——跳过
			// 条件含对端环回，外部同名头不生效（application.EdgePassedHeader 注释）。
			c.Request.Header.Set(application.EdgePassedHeader, application.EdgePassedValue)
			start := time.Now()
			fwd.proxy.ServeHTTP(c.Writer, c.Request)
			// 记录面（流水文件+内存聚合——2026-11-19全文件化零 DB；
			// 转发后记录——判定/转发同一链路，stats 有无均转发）
			if gwStats != nil {
				p := transport.PrincipalFrom(c)
				gwStats.Record(application.StatsEntry{
					AppID: p.ID, APIID: c.GetString("gw_api_id"),
					Method: method, Path: pathName,
					StatusCode: c.Writer.Status(), LatencyMS: time.Since(start).Milliseconds(),
					ClientIP: c.ClientIP(), RequestID: c.GetString("request_id"),
				})
			}
		}
	}
}
