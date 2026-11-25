// upgrader.go：WS 握手原语（TR-03；coder/websocket；ws-protocol.md §1）。
// 升级校验：一次性 ticket（TicketStore 消费式占用——单次语义由实现保证）+ Origin
// 白名单（浏览器面；非浏览器客户端 Origin 为空放行）+ X-Forwarded-Proto https
// （代理场景，可豁免）。握手失败=HTTP 响应（非 WS 关闭码——连接未建立）：
// ticket 拒绝走 18501502 失败信封（HTTP 401=契约码表 http 映射——过期细分
// 靠存储层 ErrTicketExpired）；Origin/proto 等接入面拒绝保持 403 纯文本。
// 升级失败响应体格式对客户端 WS 语义不可见（客户端只看非 101 状态与连接关闭），
// 信封不影响 WS 协议行为。
// 超时豁免：Upgrade 路径绕过 http.TimeoutHandler（§1）——由接入面把 WS 路由
// 注册在超时中间件之外（transport.Server 链外挂载），本包提供校验与升级封装。
// 协议常量唯一源 modules/realtime/api/protocol——本包不定义消息类型/
// 关闭码（Hub 接入时登记）。
package ws

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/coder/websocket"

	"github.com/netyilei/netyadmin/internal/kernel/errorx"
	"github.com/netyilei/netyadmin/internal/kernel/observability"
	"github.com/netyilei/netyadmin/internal/kernel/security/authn"
)

// Options 是握手参数。
type Options struct {
	// SurfaceName 是本升级面名（admin/gateway/client——ticket 载荷的面匹配，
	// 跨面 ticket 即无效；ws-protocol §1「ticket 绑定 surface」）。
	SurfaceName string
	// TicketStore 是一次性 ticket 存储（消费式：Take 成功即占用并返回绑定身份；
	// 提供 Redis 实现——kernel 只定义契约）。
	TicketStore TicketStore
	// OriginPatterns 是允许的浏览器 Origin host 列表（如 "https://admin.example.com"；
	// 空 = 拒绝所有带 Origin 的请求——非浏览器客户端无 Origin 可直连）。
	OriginPatterns []string
	// HandshakeTimeout 是 ticket 校验+升级总预算（§1 ticket 60s 过期之外的
	// 处理时限；<=0 取默认 5s）。
	HandshakeTimeout time.Duration
	// RequireHTTPS 代理场景校验 X-Forwarded-Proto（默认 true；本地明文测试可豁免）。
	RequireHTTPS bool
}

// TicketStore 是一次性 ticket 存储契约（实现注册）。
type TicketStore interface {
	// TakeBound 消费 ticket：有效（存在且未过期）→ 返回绑定身份/频道绑定/签发面
	// 并删除（单次语义）；不存在/已用/超保留期 → (false, nil)；逻辑过期但
	// 存储层仍可辨 → (false, ErrTicketExpired)。实现必须原子（Redis GETDEL）。
	TakeBound(ctx context.Context, ticket string) (authn.Principal, []string, string, bool, error)
}

// ErrTicketExpired 是 ticket 逻辑过期（存储层仍可辨——值内 expires_at 判定；
// upgrader 据此映射 18502，与「无效/已用」（18501）区分）。
var ErrTicketExpired = errors.New("ws: ticket expired")

// 握手拒绝原因（HTTP 403 响应体；错误串英文）。
var (
	ErrTicketInvalid  = errors.New("ws: ticket invalid or already used")
	ErrOriginDenied   = errors.New("ws: origin not allowed")
	ErrInsecureProto  = errors.New("ws: insecure proto behind proxy")
	ErrHandshakeIssue = errors.New("ws: handshake issue")
)

// Upgrader 是握手校验+升级封装（每 surface 一个实例——OriginPatterns 各面不同）。
type Upgrader struct {
	opts Options
}

// NewUpgrader 创建（TicketStore 必填——无 ticket 的 WS 拒绝装配）。
func NewUpgrader(opts Options) (*Upgrader, error) {
	if opts.TicketStore == nil {
		return nil, fmt.Errorf("ws: nil ticket store")
	}
	if opts.SurfaceName == "" {
		return nil, fmt.Errorf("ws: empty surface name (ticket surface binding)")
	}
	if opts.HandshakeTimeout <= 0 {
		opts.HandshakeTimeout = 5 * time.Second
	}
	return &Upgrader{opts: opts}, nil
}

// Upgrade 校验并升级连接。返回 conn、绑定身份与频道绑定（session 订阅校验
// 用）；校验失败返回错误并已写出 403。
func (u *Upgrader) Upgrade(w http.ResponseWriter, r *http.Request) (*websocket.Conn, authn.Principal, []string, error) {
	ctx, cancel := context.WithTimeout(r.Context(), u.opts.HandshakeTimeout)
	defer cancel()

	if u.opts.RequireHTTPS && r.Header.Get("X-Forwarded-Proto") == "http" {
		reject(w, ErrInsecureProto)
		return nil, authn.Principal{}, nil, ErrInsecureProto
	}
	if err := u.checkOrigin(r); err != nil {
		reject(w, err)
		return nil, authn.Principal{}, nil, err
	}
	principal, bound, surface, ok, err := u.opts.TicketStore.TakeBound(ctx, r.URL.Query().Get("ticket"))
	if err != nil {
		if errors.Is(err, ErrTicketExpired) {
			rejectTicket(w, r, errorx.CodeGatewayWSTicketExpired, err) // 18502（过期桶内可辨）
			return nil, authn.Principal{}, nil, ErrTicketExpired
		}
		reject(w, ErrHandshakeIssue) // 存储故障不外泄细节
		return nil, authn.Principal{}, nil, fmt.Errorf("ws: ticket take: %w", err)
	}
	if !ok || surface != u.opts.SurfaceName {
		rejectTicket(w, r, errorx.CodeGatewayWSTicketInvalid, ErrTicketInvalid) // 无效/已用/跨面（§1 surface 绑定）
		return nil, authn.Principal{}, nil, ErrTicketInvalid
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns:     u.opts.OriginPatterns,
		CompressionMode:    websocket.CompressionDisabled, // 首版关闭压缩（按面开启）
		InsecureSkipVerify: false,
	})
	if err != nil {
		return nil, principal, bound, fmt.Errorf("ws: accept: %w", err)
	}
	return conn, principal, bound, nil
}

// checkOrigin 浏览器 Origin 白名单（无 Origin = 非浏览器客户端，放行——
// ticket 本身已绑定身份；带 Origin 必须命中白名单）。
func (u *Upgrader) checkOrigin(r *http.Request) error {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return nil
	}
	for _, p := range u.opts.OriginPatterns {
		if origin == p {
			return nil
		}
	}
	return ErrOriginDenied
}

func reject(w http.ResponseWriter, cause error) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Connection", "close")
	w.WriteHeader(http.StatusForbidden)
	_, _ = w.Write([]byte(cause.Error())) // 原因面=枚举哨兵，无敏感细节
}

// rejectTicket ticket 拒绝 → 18501502 失败信封（http 401=契约码表 codeHTTP
// 映射；渲染对照 transport.Fail 惯例：X-Request-ID 回传 + errorx 信封）。reason
// 双轨保留（details.reason=原因哨兵串——兼容既有消费方）。细分为两档：
//   - 18501 无效/已用/跨面（TicketStore 返回 ok=false）；
//   - 18502 逻辑过期（TicketStore 返回 ErrTicketExpired——Redis 键保留期为
//     2×逻辑 TTL，过期桶内可辨；超桶统一 18501）。
//
// WS upgrade 失败响应走 JSON 信封不影响客户端 WS 语义——握手未达 101 即失败，
// 响应体对 WS 协议不可见。
func rejectTicket(w http.ResponseWriter, r *http.Request, code string, cause error) {
	e := errorx.New(code, "reason", cause.Error())
	reqID := r.Header.Get("X-Request-ID")
	w.Header().Set("X-Request-ID", reqID)
	w.Header().Set("Connection", "close")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(e.HTTPStatus())
	_ = json.NewEncoder(w).Encode(errorx.NewErrorEnvelope(e,
		reqID, observability.TraceIDFrom(r.Context())))
}
