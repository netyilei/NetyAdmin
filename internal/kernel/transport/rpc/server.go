// server.go：内部 RPC 服务端（TR-02；HTTP/JSON 起步）。
// 路由 /internal/v1/rpc/{module}/{port}/{method}；鉴权=Service JWT（
// ServiceVerifier，sub/scp/act/exp≤60s）；端口级 AuthZ=scope 必须覆盖
// {module}.{port}.{method}（接收方二次校验，不信任内网）；超时按方法元数据
// （Manifest §25.5）；N-1=契约主版本头校验（minor 向后兼容）。
// 响应：JSON data 或 errorx 统一信封（api-style §4 同口径）。
package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/netyilei/netyadmin/internal/kernel/errorx"
	"github.com/netyilei/netyadmin/internal/kernel/observability"
	"github.com/netyilei/netyadmin/internal/kernel/security/authn"
)

// 协议头（内部 RPC 专用；名具名——G1-08）。
const (
	// TraceHeader 是 trace 传播头（与 observability.TraceIDHeader 同值——
	// 本包独立常量避免 kernel 包级循环依赖，CI 字面量扫描锁定唯一写法）。
	TraceHeader = "trace_id"
	// ContractVersionHeader 是客户端声明端口契约主版本的头（N-1 校验）。
	ContractVersionHeader = "X-NA-Contract-Version"
	// AuthHeaderScheme 是 Authorization 头方案前缀。
	AuthHeaderScheme = "Bearer "
)

// Handler 是一个 RPC 方法实现（req 为原始 JSON；typed 由调用方泛型包装）。
type Handler func(ctx context.Context, req json.RawMessage) (any, error)

// MethodMeta 是方法元数据（Manifest ports.methods §25.5 镜像）。
type MethodMeta struct {
	Timeout    time.Duration // 单次执行上限（reliability.md §1；>0 强制）
	ContractV  int           // 契约主版本（N-1 校验基准）
	Idempotent bool          // 客户端可重试（reliability.md §2 只重试幂等）
}

// Server 是 RPC 服务端注册表（宿主面：白名单 + na_rpc_* 服务端指标）。
type Server struct {
	mu       sync.RWMutex
	handlers map[string]registered // key = module.port.method
	verifier ServiceVerifier       //  服务 JWT 验证
	allow    AllowList             // 调用方白名单
	metrics  *observability.Metrics
	onAccess func(c *gin.Context, caller string, module, port, method string, status int)
}

type registered struct {
	versions map[int]versioned // N-1：同方法多版本共存（新服务处理旧客户端窗口）
}

// versioned 是一个契约版本的实现（handler+该版本的元数据）。
type versioned struct {
	handler Handler
	meta    MethodMeta
}

// ServiceVerifier 是服务 JWT 验证的最小面（authn.ServiceVerifier 结构性满足）。
type ServiceVerifier interface {
	Authenticate(ctx context.Context, cred authn.Credential) (authn.Principal, error)
}

// ServerOptions 是服务端参数。
type ServerOptions struct {
	Verifier ServiceVerifier        // 必填——内部 RPC 无匿名
	Allow    AllowList              // 调用方白名单（必填语义：空清单=全拒；nil 同）
	Metrics  *observability.Metrics // 可空（测试旁路指标）
	// OnAccess 是访问审计回调（AccessRecord 产生方=transport RPC；
	// 「服务调用写审计 caller/callee/port/method/result」——装配方接
	// audit，kernel 不依赖审计模块；nil=不记）。
	OnAccess func(c *gin.Context, caller string, module, port, method string, status int)
}

// NewServer 创建。
func NewServer(opts ServerOptions) (*Server, error) {
	if opts.Verifier == nil {
		return nil, fmt.Errorf("rpc: nil service verifier")
	}
	return &Server{
		handlers: map[string]registered{},
		verifier: opts.Verifier,
		allow:    opts.Allow,
		metrics:  opts.Metrics,
		onAccess: opts.OnAccess,
	}, nil
}

// Register 注册单版本方法（module/port/method 全局唯一；Timeout>0 强制——禁无限）。
func (s *Server) Register(module, port, method string, h Handler, meta MethodMeta) error {
	return s.RegisterVersioned(module, port, method, []VersionedHandler{{ContractV: meta.ContractV, Handler: h, Meta: meta}})
}

// VersionedHandler 是一个契约版本的实现（N-1 双版本窗口——滚动升级期
// 新 server 同时注册旧/新版本 handler，按 client 版本头分派）。
type VersionedHandler struct {
	ContractV int
	Handler   Handler
	Meta      MethodMeta
}

// RegisterVersioned 注册多版本方法（「新服务能处理旧客户端」——同方法
// v1/v2 共存，按 client 版本精确分派；client 版本无对应 handler → 10603
// （server 窗口外——升级已完成或 client 太新）。
func (s *Server) RegisterVersioned(module, port, method string, vs []VersionedHandler) error {
	key := methodKey(module, port, method)
	if len(vs) == 0 {
		return fmt.Errorf("rpc: %s: no versions", key)
	}
	versions := make(map[int]versioned, len(vs))
	for _, v := range vs {
		if v.Handler == nil {
			return fmt.Errorf("rpc: nil handler %s v%d", key, v.ContractV)
		}
		if v.Meta.Timeout <= 0 {
			return fmt.Errorf("rpc: %s v%d: timeout must be explicit (reliability.md §1)", key, v.ContractV)
		}
		if v.ContractV <= 0 || v.Meta.ContractV <= 0 {
			return fmt.Errorf("rpc: %s v%d: contract version required (N-1)", key, v.ContractV)
		}
		if v.ContractV != v.Meta.ContractV {
			return fmt.Errorf("rpc: %s: ContractV %d != Meta.ContractV %d (双源漂移——单一语义)",
				key, v.ContractV, v.Meta.ContractV)
		}
		versions[v.ContractV] = versioned{handler: v.Handler, meta: v.Meta}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, dup := s.handlers[key]; dup {
		return fmt.Errorf("rpc: duplicate method %s", key)
	}
	s.handlers[key] = registered{versions: versions}
	return nil
}

// Mount 挂载到 gin 引擎（transport.Server Register 单 handler 入口的适配）。
func (s *Server) Mount(group gin.IRouter) {
	group.POST("/internal/v1/rpc/:module/:port/:method", s.handle)
}

func methodKey(module, port, method string) string {
	return module + "." + port + "." + method
}

// handle 单请求管线：JWT → scope → 契约版本 → 超时执行 → 信封。
func (s *Server) handle(c *gin.Context) {
	ctx := observability.ExtractTraceContext(c.Request.Context(),
		map[string]string{TraceHeader: c.GetHeader(TraceHeader)})

	start := time.Now()
	// ① Service JWT（码面按哨兵细分：10101102103104）
	principal, err := s.verifier.Authenticate(ctx, authn.Credential{
		Token: bearerToken(c.GetHeader("Authorization")),
	})
	if err != nil {
		status, rpcErr := authnErrorStatus(err)
		s.fireAccess(c, "", moduleOrParams(c), status)
		writeRPCError(c, status, rpcErr)
		return
	}

	module, port, method := c.Param("module"), c.Param("port"), c.Param("method")
	key := methodKey(module, port, method)

	// ①.5 宿主级校验（接收方白名单；fail-closed）
	defer func() {
		s.observeServer(module, port, method, start, c.Writer.Status())
		s.fireAccess(c, principal.ID, module+"."+port+"."+method, c.Writer.Status())
	}()
	if !s.applyHostChecks(c, principal, module, port, method) {
		return
	}

	// ② 端口级 scope 校验（接收方必须校验 scp 覆盖 port.method）
	if !scopeCovers(principal, module, port, method) {
		writeRPCError(c, http.StatusForbidden, errorx.New(errorx.CodePermissionDenied,
			"required_scope", key))
		return
	}

	// ③ 契约版本 N-1（按 client 版本精确分派——双版本共存窗口；
	// 无对应版本 → 10603，details 带 supported 供调用方排障）
	reg, ok := s.lookup(key)
	if !ok {
		writeRPCError(c, http.StatusNotFound, errorx.New(errorx.CodeNotFound, "method", key))
		return
	}
	clientV, err := contractVersion(c.GetHeader(ContractVersionHeader))
	if err != nil {
		writeRPCError(c, http.StatusBadRequest, errorx.New(errorx.CodeContractMismatch,
			"client_version", c.GetHeader(ContractVersionHeader)))
		return
	}
	ver, ok := reg.versions[clientV]
	if !ok {
		writeRPCError(c, http.StatusBadRequest, errorx.New(errorx.CodeContractMismatch,
			"client_version", c.GetHeader(ContractVersionHeader), "supported", supportedVersions(reg)))
		return
	}

	// ④ 方法超时执行（reliability.md §1：超时随 ctx 传播）
	mctx, cancel := context.WithTimeout(ctx, ver.meta.Timeout)
	defer cancel()
	var req json.RawMessage
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeRPCError(c, http.StatusBadRequest, errorx.New(errorx.CodeInvalidParams, "reason", "invalid json"))
		return
	}
	resp, err := ver.handler(mctx, req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			writeRPCError(c, http.StatusServiceUnavailable, errorx.New(errorx.CodeTimeout)) // 10008
			return
		}
		writeRPCError(c, http.StatusInternalServerError, err)
		return
	}
	c.Header(TraceHeader, observability.TraceIDFrom(ctx))
	c.JSON(http.StatusOK, resp)
}

// fireAccess 访问审计回调（caller 空=JWT 阶段失败未取到主体）。
func (s *Server) fireAccess(c *gin.Context, caller, portMethod string, status int) {
	if s.onAccess == nil {
		return
	}
	module, port, method := splitPortMethod(portMethod)
	s.onAccess(c, caller, module, port, method, status)
}

// moduleOrParams 是 JWT 失败路径的粗粒度标识（路由参数可能缺——client 侧构造
// 失败面；正常请求四段全）。
func moduleOrParams(c *gin.Context) string {
	return c.Param("module") + "." + c.Param("port") + "." + c.Param("method")
}

// splitPortMethod 拆回三段（fireAccess 的 module.port.method 复原；不足三段
// 尾段留空——JWT 失败路径的粗粒度标识面）。
func splitPortMethod(key string) (module, port, method string) {
	parts := strings.SplitN(key, ".", 3)
	for len(parts) < 3 {
		parts = append(parts, "")
	}
	return parts[0], parts[1], parts[2]
}

// supportedVersions 是方法已注册版本清单（10603 details——调用方排障面）。
func supportedVersions(reg registered) []int {
	out := make([]int, 0, len(reg.versions))
	for v := range reg.versions {
		out = append(out, v)
	}
	return out
}

func (s *Server) lookup(key string) (registered, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	reg, ok := s.handlers[key]
	return reg, ok
}

// scopeCovers 校验 Principal.Scopes 覆盖目标方法（精确项或同端口通配
// `module.port.*`——签发契约要求精确枚举，通配仅 port 级收窄）。
func scopeCovers(p authn.Principal, module, port, method string) bool {
	key := methodKey(module, port, method)
	portPrefix := module + "." + port + "."
	for _, sc := range p.Scopes {
		switch sc {
		case key:
			return true
		case portPrefix + "*":
			return true
		}
	}
	return false
}

func bearerToken(header string) string {
	if v, ok := strings.CutPrefix(header, AuthHeaderScheme); ok {
		return strings.TrimSpace(v)
	}
	return header
}

// authnErrorStatus 映射领域错误 → HTTP 传输语义（码面按哨兵细分，
// api-style §4 message_key 粒度不折叠）。
func authnErrorStatus(err error) (int, *errorx.Error) {
	switch {
	case errors.Is(err, authn.ErrNoCredential):
		return http.StatusUnauthorized, errorx.New(errorx.CodeTokenMissing)
	case errors.Is(err, authn.ErrExpiredToken):
		return http.StatusUnauthorized, errorx.New(errorx.CodeTokenExpired)
	case errors.Is(err, authn.ErrRevokedToken):
		return http.StatusUnauthorized, errorx.New(errorx.CodeTokenRevoked)
	case errors.Is(err, authn.ErrInvalidToken), errors.Is(err, authn.ErrUnknownScheme):
		return http.StatusUnauthorized, errorx.New(errorx.CodeTokenInvalid)
	default:
		// 系统错误（吊销面 Redis 不可用等）→ 10010（fail-closed 对外表述）
		return http.StatusServiceUnavailable, errorx.New(errorx.CodeUnavailable)
	}
}

// contractVersion 解析客户端契约主版本头（缺失/非法 → 错误，10603）。
func contractVersion(header string) (int, error) {
	if header == "" {
		return 0, errors.New("rpc: contract version header missing")
	}
	v, err := strconv.Atoi(header)
	if err != nil || v <= 0 {
		return 0, fmt.Errorf("rpc: bad contract version %q", header)
	}
	return v, nil
}

// writeRPCError 渲染 errorx 信封（api-style §4；details 结构化上下文）。
// status+err 双参或 (int, *errorx.Error) 组合由调用方解构后传入。
func writeRPCError(c *gin.Context, status int, err error) {
	env := errorx.NewErrorEnvelopeOf(err, c.GetHeader("X-Request-ID"), observability.TraceIDFrom(c.Request.Context()))
	c.Header("X-Request-ID", c.GetHeader("X-Request-ID"))
	c.JSON(status, env)
}
