// edge.go：网关 HTTP 入口鉴权编排（V1 OpenPlatformAuth 385 行模块化——
// §18 edge 上下文）。六步管线（V1 顺序对齐，安全顺序修正保留）：
//
//	① timestamp ±60s 窗（authn.AppHMACVerifier）→ ② App 快照（gw.Repo）→
//	③ 状态（disabled 拒）→ ④ IPAC fail-closed → ⑤ 签名验证（canonical
//	定稿）→ ⑥ nonce 防重放（签名后占用——防 nonce 槽位 DoS，V1 修正语义）→
//	⑦ quota（App token 桶——coordination）→ ⑧ scope→API 白名单 → Principal 注入。
//
// 销项：HMAC canonical 定稿（V1 对照）+ App Principal.Scopes 填充。
package adapters

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/netyilei/netyadmin/internal/kernel/errorx"
	"github.com/netyilei/netyadmin/internal/kernel/observability"
	"github.com/netyilei/netyadmin/internal/kernel/security/authn"
	"github.com/netyilei/netyadmin/internal/kernel/state/coordination"
	"github.com/netyilei/netyadmin/internal/modules/gw/application"
	"net/netip"

	"github.com/netyilei/netyadmin/internal/modules/gw/domain"
)

// EdgeDeps 是 edge 中间件依赖（module.go 装配注入）。
// 判定数据源=gw 装载型快照（application.AppSnapshot——Service 三方法内切换，
// 本文件控制流/细分码/顺序不变；快照未装载=既有 fail-closed 映射）。
type EdgeDeps struct {
	Service *application.Service       // 治理面（App 快照）
	Nonce   *coordination.Coordination // nonce 防重放
	IPAC    IPACChecker                // IP 访问控制（ipac 模块 Port——fail-closed）
	Metrics *observability.Metrics
}

// IPACChecker 是 edge IP 判定面（配置组模型——实现=gw application
// IPEngine.MatchGroups：应用启用组集严格合成，deny 跨组恒优先，组集空=不限）。
type IPACChecker interface {
	CheckGroups(ctx context.Context, ip netip.Addr, groupIDs []string) domain.IPDecision
}

// payloadFor 构造签名串第五段（定稿：GET=query 排序全值串/非 GET=body
// sha256 hex——V1 同构；同名参数全值参与防参数走私）。
func payloadFor(c *gin.Context, body []byte) string {
	if strings.ToUpper(c.Request.Method) == "GET" {
		query := c.Request.URL.Query()
		ks := make([]string, 0, len(query))
		for k := range query {
			ks = append(ks, k)
		}
		sort.Strings(ks)
		var sb strings.Builder
		first := true
		for _, k := range ks {
			for _, v := range query[k] {
				if !first {
					sb.WriteString("&")
				}
				first = false
				sb.WriteString(k)
				sb.WriteString("=")
				sb.WriteString(v)
			}
		}
		return sb.String()
	}
	if len(body) > 0 {
		h := sha256.Sum256(body)
		return hex.EncodeToString(h[:])
	}
	return ""
}

// buildCredential 组装 HMAC Credential（头/payload——canonical 输入）。
func buildCredential(c *gin.Context, payload string) authn.Credential {
	return authn.Credential{
		Token:   c.GetHeader("X-NA-Signature"),
		Method:  c.Request.Method,
		Path:    c.Request.URL.Path,
		Payload: payload,
		Headers: map[string]string{
			authn.HeaderAppID:     c.GetHeader("X-NA-App-Id"),
			authn.HeaderTimestamp: c.GetHeader("X-NA-Timestamp"),
			authn.HeaderNonce:     c.GetHeader("X-NA-Nonce"),
		},
	}
}

// EdgeAuthMiddleware 网关入口鉴权编排（/gateway/v1 组级）。
// verifier 为 AppHMACVerifier（module.go 装配——Secret 经 gw Repo 快照解密）。
func EdgeAuthMiddleware(deps EdgeDeps, verifier *authn.AppHMACVerifier) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 读 body（签名 payload + dispatch 透传两用；limit 由 mwBodyLimit 引擎级承担）
		var body []byte
		if c.Request.Body != nil {
			raw, err := io.ReadAll(c.Request.Body)
			if err != nil {
				transportFail(c, errorx.New(errorx.CodeInvalidParams, "reason", "read body"))
				return
			}
			body = raw
			c.Request.Body = io.NopCloser(strings.NewReader(string(raw)))
		}
		payload := payloadFor(c, body)

		// ①⑤ HMAC 验签链（timestamp 窗/app secret/canonical——authn provider）
		principal, err := verifier.Authenticate(c.Request.Context(), buildCredential(c, payload))
		if err != nil {
			transportFail(c, mapEdgeAuthnError(err))
			return
		}

		// ②③ App 快照 + 状态（gw 治理面）——细分码契约：18101102。
		app, scopes, err := deps.Service.AppAuthSnapshot(c.Request.Context(), principal.ID)
		if err != nil {
			transportFail(c, errorx.New(errorx.CodeGatewayAppNotFound, "reason", "app not found"))
			return
		}
		if app.Status != domain.StatusEnabled {
			transportFail(c, errorx.New(errorx.CodeGatewayAppDisabled, "reason", "app disabled"))
			return
		}

		// ④ IP 访问控制（配置组模型——严格合成「禁止 > 开放」）：
		// 应用启用组集内任一 deny 命中 → 拒（跨组恒优先）；allow 命中 → 放；
		// 组集非空无命中 → 拒（fail-closed 白名单语义）；组集空 → 不限（零配置不拒）。
		// 全局规则概念=默认组（gw_apps 行 ip_group_ids 引用默认组即全员生效）。
		if deps.IPAC == nil {
			transportFail(c, errorx.New(errorx.CodeUnavailable, "reason", "ipac not wired"))
			return
		}
		d := deps.IPAC.CheckGroups(c.Request.Context(), clientIPAddr(c), app.IPGroupIDs)
		if !d.Allowed {
			transportFail(c, errorx.New(errorx.CodeIPBlocked))
			return
		}

		// ⑥ nonce 防重放（签名通过后占用——V1 DoS 修正语义；TTL 2m 覆盖 ±60s 窗）。
		// 无 Redis（Nonce nil）=防重放面不可用 → fail-closed 拒绝（评审修：
		// 此前 nil 静默跳过=重放面裸奔）。
		if deps.Nonce == nil {
			transportFail(c, errorx.New(errorx.CodeUnavailable, "reason", "replay guard unavailable"))
			return
		}
		nonce := c.GetHeader("X-NA-Nonce")
		ok, err := deps.Nonce.CheckNonce(c.Request.Context(), app.AppKey+":"+nonce, 2*time.Minute)
		if err != nil {
			transportFail(c, errorx.New(errorx.CodeUnavailable)) // 存储故障=503（非 401——防重放面语义）
			return
		}
		if !ok {
			transportFail(c, errorx.New(errorx.CodeGatewayNonceReplayed, "reason", "nonce replay"))
			return
		}

		// ⑦ quota（coordination 滑动窗口按 app 维度；Rate=每秒请求数→窗口 1s。
		// 评审修：ErrDenied（超限）此前静默放行；窗口此前错用 1m）。
		// 契约语义（G3 定档）：RateLimitEnabled=true 时 Rate=0=不限流（显式无限，
		// 管理面可设 0 关闭配额）——0 非禁用；RateLimitEnabled=false 才是关闭面。
		// 不可用=503 fail-closed（state-policies gateway_app on_unavailable 定档）。
		if app.RateLimitEnabled && app.Quota.Rate > 0 {
			switch err := deps.Nonce.AllowByPolicy(c.Request.Context(),
				"gateway_app", app.ID, app.Quota.Rate, time.Second); {
			case err == nil:
			case errors.Is(err, coordination.ErrDenied):
				transportFail(c, errorx.New(errorx.CodeGatewayQuotaExceeded, "reason", "quota exceeded"))
				return
			case errors.Is(err, coordination.ErrUnavailable):
				transportFail(c, errorx.New(errorx.CodeUnavailable))
				return
			default: // 未知错误=拒绝面到此（非既知 Denied/Unavailable 不放行）
				transportFail(c, errorx.New(errorx.CodeUnavailable))
				return
			}
		}

		// Principal 注入（App HMAC——Scopes 填充=app 授权 scopes；§8.1 销项）；
		// scopes 同注入 ctx（APIGate ⑧ 步消费—— 起拆分：平台端点 /ws/*
		// 不参与 gw_apis 登记，仅业务分派分支过白名单）。
		principal.Scopes = scopes
		c.Set("principal", principal)
		c.Set("gw_app_scopes", scopes)
		c.Next()
	}
}

// APIGate 是 edge ⑧ 步 scope→API 白名单（method+path 匹配 gw_apis → 要求
// scopes ∩ app scopes；EdgeAuthMiddleware 之后挂载——仅业务 API 分派分支）。
func APIGate(deps EdgeDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		scopes, _ := c.MustGet("gw_app_scopes").([]string)
		api, requiredScopes, granted, err := deps.Service.MatchGatewayAPI(c.Request.Context(),
			c.Request.Method, c.Request.URL.Path)
		if err != nil {
			transportFail(c, errorx.New(errorx.CodeGatewayAPINotRegistered, "reason", "api not registered"))
			return
		}
		// 白名单存在但无启用组覆盖（组全禁用）=策略失效——拒绝（F8 根修）；
		// 未配置白名单（granted=false）=登记面自由——放行。两处拒绝同属 scope
		// 授权面 → 18201（细分码契约），reason 区分 grant-disabled/mismatch。
		if granted && len(requiredScopes) == 0 {
			transportFail(c, errorx.New(errorx.CodeGatewayScopeDenied, "reason", "scope grant disabled"))
			return
		}
		if !scopesCover(requiredScopes, scopes) {
			transportFail(c, errorx.New(errorx.CodeGatewayScopeDenied, "reason", "scope mismatch"))
			return
		}
		c.Set("gw_api_id", api.ID)
		c.Next()
	}
}

// scopesCover 判定 app 授权 scopes 与 API 要求 scopes 有交集。
func scopesCover(required, granted []string) bool {
	if len(required) == 0 {
		return true // 未声明要求 scope 的 API=登录即可（登记面自由度）
	}
	set := make(map[string]bool, len(granted))
	for _, s := range granted {
		set[s] = true
	}
	for _, s := range required {
		if set[s] {
			return true
		}
	}
	return false
}

// mapEdgeAuthnError HMAC 验签错误 → 18xxx 细分码（契约 401 段）：时间窗过期
// 18104（ErrExpiredToken 哨兵——authn ±60s skew 判定），签名错误本体 18103
// （含缺凭证/时间戳格式坏/secret 缺失/HMAC 不符——不外泄具体成因，reason
// 细分由 nonce 步 18105 承载）。
func mapEdgeAuthnError(err error) *errorx.Error {
	if errors.Is(err, authn.ErrExpiredToken) {
		return errorx.New(errorx.CodeGatewayTimestampExpired, "reason", "timestamp expired")
	}
	// Secret 回调透传的「应用不存在」（快照未命中）对齐契约 18101——此前落入
	// 通用 18103 签名错误（multi 顺带发现：契约 app_not_found 语义不可达）
	if errors.Is(err, domain.ErrNotFound) {
		return errorx.New(errorx.CodeGatewayAppNotFound, "reason", "app not found")
	}
	return errorx.New(errorx.CodeGatewaySignatureInvalid)
}

// transportFail 统一失败渲染（errorx 信封；X-Request-ID 传播）。
func transportFail(c *gin.Context, e *errorx.Error) {
	c.Header("X-Request-ID", c.GetHeader("X-Request-ID"))
	c.JSON(e.HTTPStatus(), errorx.NewErrorEnvelope(e,
		c.GetHeader("X-Request-ID"), observability.TraceIDFrom(c.Request.Context())))
	c.Abort()
}

// clientIPAddr 解析客户端 IP（IPAC 判定输入——解析失败返回零值 Addr 由引擎处理）。
func clientIPAddr(c *gin.Context) netip.Addr {
	addr, err := netip.ParseAddr(c.ClientIP())
	if err != nil {
		return netip.Addr{}
	}
	return addr
}
