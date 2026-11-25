// client.go：内部 RPC 客户端（TR-02；reliability.md 全约束落地）——
// 熔断 sony/gobreaker/v2 按目标 BaseURL 隔离（§3 半开 ≥3 成功恢复）；重试
// cenkalti/backoff/v5 仅幂等方法（§2 ≤3 次且计入调用方超时预算；4xx/熔断不
// 叠加）；多端点轮询 LB（retryable 换端重试 + 可选主动探活恢复）；
// 签名= primitives SignJWT_RS256（sub/scp/exp≤60s，签发契约）；
// trace_id 头传播。
package rpc

import (
	"bytes"
	"context"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/cenkalti/backoff/v5"
	"github.com/sony/gobreaker/v2"

	"github.com/netyilei/netyadmin/internal/kernel/observability"
	"github.com/netyilei/netyadmin/internal/kernel/security/primitives"
)

// 客户端默认值（reliability.md 基线；显式具名——禁依赖默认值）。
const (
	defaultCallTimeout   = 5 * time.Second  // §1 外部调用显式超时
	defaultRetryAttempts = 2                // 首调 + ≤2 重试（§2 单调用链重试 ≤3 次）
	defaultServiceJWTTTL = 60 * time.Second // exp ≤ 60s 签发契约
	circuitHalfOpenMax   = 3                // §3 半开探测并发数（恢复语义见下 IsSuccessful 注）
	circuitTripThreshold = 5                // 连续失败 5 次熔断（初始基线）
	circuitOpenTimeout   = 30 * time.Second // 熔断打开持续时间

	healthProbePath    = "/health/ready"       // 主动探活路径（进程就绪约定面，smoke/waitReady 同源）
	probeTimeout       = 2 * time.Second       // 单次探活预算（独立于调用超时）
	probeIntervalFloor = 10 * time.Millisecond // 探活周期下限（防误配热循环）
)

// 客户端错误哨兵。
var (
	// ErrCircuitOpen：目标熔断打开（调用方不得重试——不放大故障）。
	ErrCircuitOpen = errors.New("rpc: circuit open")
	// ErrRetryExhausted：幂等方法重试穷尽（末次错误在链上）。
	ErrRetryExhausted = errors.New("rpc: retry exhausted")
	// ErrRejected：服务端业务拒绝信封（4xx 语义；重试无意义）。
	ErrRejected = errors.New("rpc: rejected by server")
)

// ClientOptions 是客户端参数。
type ClientOptions struct {
	Signer    *rsa.PrivateKey  // service JWT 签发钥（与验证侧配对）
	ServiceID string           // JWT sub（调用方服务名）
	Audience  string           // JWT aud（空=不写）
	JWTTTL    time.Duration    // ≤0 取默认 60s；>60s 拒绝
	Clock     primitives.Clock // 时间注入（nil=SystemClock）
	// Endpoints 是目标实例清单（Discovery/static endpoints——轮询 LB；
	// 单端点兼容写 BaseURL）。熔断按 endpoint 隔离（一台熔断不影响其余）。
	BaseURL   string                 // 单端点便捷写法（Endpoints 空时生效）
	Endpoints []string               // 多端点清单（轮询；优先于 BaseURL）
	HTTP      *http.Client           // 可注入（默认带 per-call 超时的客户端）
	Metrics   *observability.Metrics // 可空
	// ProbeInterval 是主动探活周期（R3「RPC 主动探活」；≤0=关——默认）。
	// 启用后后台 goroutine 周期探测熔断 open 的端点（GET /health/ready），
	// 成功即重建熔断器恢复入环——恢复感知从被动 open-timeout（30s）缩到单探测周期。
	// 用毕调 Close 停止（未启用为 no-op）。下限 probeIntervalFloor 防误配热循环。
	ProbeInterval time.Duration
}

// Client 是内部 RPC 调用端（每 Client 一个目标 BaseURL）。
//
// 半开恢复语义：gobreaker MaxRequests=3 限半开并发探测数，任一探测成功即恢复
// closed——与 reliability §3「半开探测成功数 ≥3 才全开」存在口径差（gobreaker
// v2 无连续成功恢复原语），已登记（触发点：首个熔断敏感消费方）。
// 主动探活启用时，open 端点恢复由探活驱动（探测成功=恢复证据，直接重建 closed
// 入环——半开窗口由探活承担）；未启用时退化为被动 open-timeout 恢复。
type Client struct {
	opts      ClientOptions
	http      *http.Client
	endpoints []*endpointClient //：静态发现清单（轮询 LB + 按 endpoint 熔断）
	next      atomic.Uint64     // round-robin 游标

	probeCancel   chan struct{} // 探活停止信号（nil=未启用）
	probeDone     chan struct{} // 探活 goroutine 退出确认（Close 等待——防泄漏）
	probeCancelFn func()        // context 取消（探活阻塞拨号快速退出）
}

// endpointClient 是单实例子客户端（熔断按 endpoint 隔离——一台过载不拖垮其余）。
// circuit 走 atomic.Pointer：主动探活恢复会重建熔断器（Invoke 并发读——锁换原子）。
type endpointClient struct {
	baseURL    string
	circuit    atomic.Pointer[gobreaker.CircuitBreaker[json.RawMessage]]
	newBreaker func() *gobreaker.CircuitBreaker[json.RawMessage] // 探活恢复重建（同设置）
}

// breaker 取当前熔断器（探活重建后指向新实例）。
func (e *endpointClient) breaker() *gobreaker.CircuitBreaker[json.RawMessage] {
	return e.circuit.Load()
}

// state 是熔断态的 gauge 映射（0=closed 1=half-open 2=open）。
func (e *endpointClient) state() float64 {
	switch e.breaker().State() {
	case gobreaker.StateHalfOpen:
		return 1
	case gobreaker.StateOpen:
		return 2
	}
	return 0
}

// NewClient 创建。
func NewClient(opts ClientOptions) (*Client, error) {
	if opts.Signer == nil || opts.ServiceID == "" {
		return nil, fmt.Errorf("rpc: client requires signer and service id")
	}
	if opts.BaseURL == "" && len(opts.Endpoints) == 0 {
		return nil, fmt.Errorf("rpc: client requires base url or endpoints")
	}
	if opts.BaseURL != "" && len(opts.Endpoints) == 0 {
		opts.Endpoints = []string{opts.BaseURL}
	}
	if opts.JWTTTL <= 0 {
		opts.JWTTTL = defaultServiceJWTTTL
	}
	if opts.JWTTTL > defaultServiceJWTTTL {
		return nil, fmt.Errorf("rpc: jwt ttl %s exceeds cap 60s", opts.JWTTTL)
	}
	if opts.Clock == nil {
		opts.Clock = primitives.SystemClock{}
	}
	if opts.HTTP == nil {
		opts.HTTP = &http.Client{Timeout: defaultCallTimeout}
	}
	eps := make([]*endpointClient, 0, len(opts.Endpoints))
	for _, base := range opts.Endpoints {
		base := base
		eps = append(eps, &endpointClient{
			baseURL:    base,
			newBreaker: func() *gobreaker.CircuitBreaker[json.RawMessage] { return newBreaker(base) },
		})
		eps[len(eps)-1].circuit.Store(eps[len(eps)-1].newBreaker())
	}
	c := &Client{opts: opts, http: opts.HTTP, endpoints: eps}
	if opts.ProbeInterval > 0 {
		if opts.ProbeInterval < probeIntervalFloor {
			opts.ProbeInterval = probeIntervalFloor
		}
		ctx, cancel := context.WithCancel(context.Background())
		c.probeCancel = make(chan struct{})
		c.probeDone = make(chan struct{})
		c.probeCancelFn = cancel
		go func() {
			defer close(c.probeDone)
			c.probeLoop(ctx)
		}()
	}
	return c, nil
}

// newBreaker 构造熔断器（reliability.md §3 基线；探活恢复重建同设置）。
func newBreaker(base string) *gobreaker.CircuitBreaker[json.RawMessage] {
	return gobreaker.NewCircuitBreaker[json.RawMessage](gobreaker.Settings{
		Name:        base,
		MaxRequests: circuitHalfOpenMax,
		Timeout:     circuitOpenTimeout,
		ReadyToTrip: func(c gobreaker.Counts) bool { return c.TotalFailures >= circuitTripThreshold },
		// IsSuccessful=true 表示本次调用算成功（不计失败）——4xx 业务拒绝
		// 当成功处理（目标活着，只是请求被拒；reliability.md §3 熔断语义=
		// 保护对端过载）；5xx/网络错误=false 计失败。
		IsSuccessful: func(err error) bool {
			if err == nil {
				return true
			}
			var httpErr *httpError
			if errors.As(err, &httpErr) {
				return httpErr.status < 500 // 4xx 算成功
			}
			return false // 网络错误计失败
		},
	})
}

// Close 停止主动探活 goroutine（未启用探活为 no-op；进程优雅关闭面）。
func (c *Client) Close() {
	if c.probeCancel == nil {
		return
	}
	select {
	case <-c.probeCancel:
	default:
		close(c.probeCancel)
		c.probeCancelFn()
		<-c.probeDone
	}
}

// probeLoop 主动探活主循环（默认关——ProbeInterval>0 启用）：对熔断 open 的
// 端点周期 GET /health/ready，成功即重建熔断器（closed 重回轮询）。顺带刷新
// na_rpc_circuit_state——open 端点不被调用时 gauge 否则冻结在旧值。
func (c *Client) probeLoop(ctx context.Context) {
	ticker := time.NewTicker(c.opts.ProbeInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for _, ep := range c.endpoints {
				if c.opts.Metrics != nil {
					c.opts.Metrics.RPCCircuitState.WithLabelValues(ep.baseURL).Set(ep.state())
				}
				if ep.breaker().State() != gobreaker.StateOpen {
					continue
				}
				if c.probeOnce(ep) {
					ep.circuit.Store(ep.newBreaker()) // 探测成功=恢复证据：重建 closed 入环
				}
			}
		}
	}
}

// probeOnce 单端点轻量健康探测（GET /health/ready——进程就绪约定面；
// <400=存活；失败/超时不动作——下一周期再探）。
func (c *Client) probeOnce(ep *endpointClient) bool {
	req, err := http.NewRequest(http.MethodGet, strings.TrimSuffix(ep.baseURL, "/")+healthProbePath, nil)
	if err != nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	resp, err := c.http.Do(req.WithContext(ctx))
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }() // 探活响应体不消费——排空防连接池悬挂
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	return resp.StatusCode < 400
}

// Invoke 调用远端方法：req 编码 JSON → 2xx 响应解码进 resp（nil 跳过）。
// retryable=true 按指数退避重试（仅网络/超时/5xx；4xx 业务拒绝与熔断态不叠加），
// 且每次重试换端点（轮询 LB——不再 hammer 首端点）。
func (c *Client) Invoke(ctx context.Context, module, port, method string,
	contractV int, retryable bool, req, resp any) error {

	// 轮询 LB：round-robin 选端点，跳过熔断 open 的实例（被动摘除；
	// 主动探活启用时 open 端点由探活提前恢复入环）。
	ep := c.pickEndpoint()
	if ep == nil {
		return fmt.Errorf("%w: all endpoints open (%d)", ErrCircuitOpen, len(c.endpoints))
	}
	start := time.Now()
	var raw json.RawMessage
	var err error
	if retryable {
		raw, ep, err = c.invokeWithRetry(ctx, ep, module, port, method, contractV, req)
	} else {
		raw, err = ep.breaker().Execute(func() (json.RawMessage, error) {
			return c.invokeOnce(ctx, ep.baseURL, module, port, method, contractV, req)
		})
	}
	if c.opts.Metrics != nil {
		c.opts.Metrics.RPCCallsTotal.WithLabelValues(module, port, method, "client", codeGroup(err)).Inc()
		c.opts.Metrics.RPCDuration.WithLabelValues(module, port, method, "client").Observe(time.Since(start).Seconds())
		c.opts.Metrics.RPCCircuitState.WithLabelValues(ep.baseURL).Set(ep.state())
	}
	if err != nil {
		// 熔断态统一 ErrCircuitOpen 表述（retryable 路径已在 invokeWithRetry
		// 按末次端点包装——此处只补非重试路径的 gobreaker 哨兵）。
		if errors.Is(err, gobreaker.ErrOpenState) || errors.Is(err, gobreaker.ErrTooManyRequests) {
			return fmt.Errorf("%w: %s", ErrCircuitOpen, ep.baseURL)
		}
		return err
	}
	if resp != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, resp); err != nil {
			return fmt.Errorf("rpc: decode %s.%s.%s response: %w", module, port, method, err)
		}
	}
	return nil
}

// pickEndpoint round-robin 选端点（跳过 open 熔断——最多绕一圈；全 open 返 nil）。
func (c *Client) pickEndpoint() *endpointClient {
	n := len(c.endpoints)
	if n == 0 {
		return nil
	}
	for i := 0; i < n; i++ {
		ep := c.endpoints[(c.next.Add(1)-1)%uint64(n)]
		if ep.breaker().State() != gobreaker.StateOpen {
			return ep
		}
	}
	return nil // 全部 open——调用方收 ErrCircuitOpen（不盲试）
}

// invokeWithRetry 幂等方法重试（backoff/v5 指数退避；ctx 预算统一约束）。
// 重试换端（互-A1）：每次重试重新轮询选端点（游标推进=下一端点），
// 失败按尝试所在端点的熔断器各自计数；全 open 时留在当前端点（其熔断拒绝=
// 快速失败，熔断态不重试——不放大故障）。返回末次尝试端点（指标归因面）。
func (c *Client) invokeWithRetry(ctx context.Context, first *endpointClient, module, port, method string,
	contractV int, req any) (json.RawMessage, *endpointClient, error) {

	ep := first
	attempt := 0
	raw, err := backoff.Retry(ctx, func() (json.RawMessage, error) {
		if attempt > 0 {
			if next := c.pickEndpoint(); next != nil {
				ep = next // 换端重试
			}
		}
		attempt++
		out, callErr := ep.breaker().Execute(func() (json.RawMessage, error) {
			return c.invokeOnce(ctx, ep.baseURL, module, port, method, contractV, req)
		})
		if callErr != nil {
			if errors.Is(callErr, gobreaker.ErrOpenState) || errors.Is(callErr, gobreaker.ErrTooManyRequests) {
				return out, backoff.Permanent(fmt.Errorf("%w: %s", ErrCircuitOpen, ep.baseURL))
			}
			if !retryWorthy(callErr) {
				return out, backoff.Permanent(callErr)
			}
		}
		return out, callErr
	}, backoff.WithMaxTries(defaultRetryAttempts+1), backoff.WithBackOff(backoff.NewExponentialBackOff()))
	return raw, ep, err
}

// retryWorthy 判定可重试错误类：网络/超时/5xx 可重试；4xx 业务拒绝不重试（不放大）。
func retryWorthy(err error) bool {
	var httpErr *httpError
	if errors.As(err, &httpErr) {
		return httpErr.status >= 500
	}
	return true // 网络层错误可重试
}

// httpError 是非 2xx 响应（信封体原文随错误链进日志）。
type httpError struct {
	status int
	body   []byte
}

func (e *httpError) Error() string {
	return fmt.Sprintf("rpc: http status %d: %s", e.status, string(e.body))
}

// invokeOnce 单次调用（签名/头/超时全链路）。
func (c *Client) invokeOnce(ctx context.Context, baseURL, module, port, method string,
	contractV int, req any) (json.RawMessage, error) {

	body := []byte("null")
	if req != nil {
		raw, err := json.Marshal(req)
		if err != nil {
			return nil, fmt.Errorf("rpc: encode request: %w", err)
		}
		body = raw
	}
	token, err := c.signToken(module, port, method)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		baseURL+"/internal/v1/rpc/"+module+"/"+port+"/"+method, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("rpc: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", AuthHeaderScheme+token)
	httpReq.Header.Set(ContractVersionHeader, fmt.Sprintf("%d", contractV))
	if tid := observability.TraceIDFrom(ctx); tid != "" {
		httpReq.Header.Set(TraceHeader, tid)
	}
	httpResp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("rpc: call %s.%s.%s: %w", module, port, method, err)
	}
	defer func() { _ = httpResp.Body.Close() }() // 读取后关闭失败无补救路径
	raw, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("rpc: read response: %w", err)
	}
	if httpResp.StatusCode != http.StatusOK {
		return nil, &httpError{status: httpResp.StatusCode, body: raw}
	}
	return raw, nil
}

// codeGroup 是客户端结果低基数分组（2xx/4xx/5xx/net——网络层错误独立组）。
func codeGroup(err error) string {
	switch {
	case err == nil:
		return "2xx"
	case errors.Is(err, ErrCircuitOpen), errors.Is(err, ErrRetryExhausted):
		return "circuit"
	}
	var httpErr *httpError
	if errors.As(err, &httpErr) {
		if httpErr.status >= 500 {
			return "5xx"
		}
		return "4xx"
	}
	return "net"
}

// signToken 签发单次调用 service JWT（exp≤60s；scp 精确 port.method；服务间
// 直调无代行主体故无 act；trace_id 走头传播不进 claim）。
func (c *Client) signToken(module, port, method string) (string, error) {
	now := c.opts.Clock.Now()
	token, err := primitives.SignJWT_RS256(c.opts.Signer, map[string]any{
		"sub": c.opts.ServiceID,
		"scp": []string{module + "." + port + "." + method},
		"iat": now.Unix(),
		"exp": now.Add(c.opts.JWTTTL).Unix(),
	})
	if err != nil {
		return "", fmt.Errorf("rpc: sign service jwt: %w", err)
	}
	return token, nil
}
