// session.go：WS 接入面（ws-protocol §1 握手 + §4 帧处理 + §6 限制 +
// §7 心跳）。机制面三端共用（gateway/admin/client——surface 特化经
// ChannelAuthorizer/OriginPatterns 注入）；协议语义/信封/常量唯一源
// realtime/api/protocol（本包不定义帧类型字符串）。goroutine 纪律：每连接
// 读循环（HTTP handler goroutine）+ WriterLoop + 心跳器，全部 wg 托管。
// 超时豁免由 mwTimeout 承担（识别 Upgrade 跳过 30s——长连接生命周期归本面）。
package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/gin-gonic/gin"

	"github.com/netyilei/netyadmin/internal/kernel/errorx"
	"github.com/netyilei/netyadmin/internal/kernel/observability"
	"github.com/netyilei/netyadmin/internal/kernel/security/authn"
	"github.com/netyilei/netyadmin/internal/kernel/security/primitives"
	"github.com/netyilei/netyadmin/internal/kernel/state/coordination"
	"github.com/netyilei/netyadmin/internal/kernel/transport"
	"github.com/netyilei/netyadmin/internal/kernel/transport/ws"
	"github.com/netyilei/netyadmin/internal/modules/realtime/api/channels"
	"github.com/netyilei/netyadmin/internal/modules/realtime/api/keys"
	"github.com/netyilei/netyadmin/internal/modules/realtime/api/protocol"
)

// Limits 是连接与消息限制（ws-protocol §6——全部经配置注入，禁止硬编码）。
type Limits struct {
	HeartbeatInterval       time.Duration // ping 周期（<=0 取默认 30s）
	IdleTimeout             time.Duration // 空闲关闭 1001（<=0 取默认 90s）
	MaxChannelsPerConn      int           // 单连接频道数（<=0 取默认 100）
	MaxFrameBytes           int64         // 单帧上限（<=0 取默认 64KB）
	MessageRatePerSec       int           // 上行消息速率（<=0 取默认 10/s）
	SendQueue               int           // 发送队列（<=0 取默认 64）
	MaxConnsPerPrincipal    int           // 同身份连接上限（<=0 取默认 5——§6 4005）
	MaxConnsPerApp          int           // 单 App 连接上限（<=0 取默认 10000——§6 1013）
	MaxConnsTotal           int           // 节点级全局连接上限（0=取默认 100000；负=显式不限）
	MaxMessageRatePerAppSec int           // App 维度上行消息速率（<=0 取默认 1000/s——§6 应用级控制）
	BroadcastRatePerSec     int           // 每频道广播速率上限（0=10×MessageRatePerSec；§6 广播速率——超限丢弃并计数）
}

// Limits 零值兜底默认（与 kernel/config.defaults 同值——配置层已 fail-fast 拒
// 非正值，此处兜住不经 config 的直接装配路径；ws-protocol §6 规范基线）。
const (
	defaultHeartbeatInterval       = 30 * time.Second
	defaultIdleTimeout             = 90 * time.Second
	defaultMaxChannelsPerConn      = 100
	defaultMaxFrameBytes           = 64 * 1024
	defaultMessageRatePerSec       = 10
	defaultMaxConnsPerPrincipal    = 5
	defaultMaxConnsPerApp          = 10000
	defaultMaxConnsTotal           = 100000
	defaultMaxMessageRatePerAppSec = 1000
)

// Coordinator 是 WS 面消费的协调原语子集（*coordination.Coordination 实现；
// nil=single 无协调面——seq 进程内计数、App 维度限流直通）。
type Coordinator interface {
	// IncrSequence 多节点单调序号（Redis INC+续期；single Redis 故障回退本地）。
	IncrSequence(ctx context.Context, key string, ttl time.Duration) (int64, error)
	// AllowByPolicy 命名策略限流（contracts/state-policies.yaml rate_limit）。
	AllowByPolicy(ctx context.Context, policyName, dimensionKey string, limit int, window time.Duration) error
}

// TicketIssuer 是 ticket 签发+消费契约（*ws.RedisTicketStore 实现；测试桩可
// 替换——IssueBound/消费原子性由实现承担）。
type TicketIssuer interface {
	ws.TicketStore
	IssueBound(ctx context.Context, grant ws.TicketGrant) (string, error)
}

// SurfaceOptions 是接入面构造参数（Upgrader 由工厂内建——TicketStore/Origin
// 为参数面，调用方不触碰握手原语）。
type SurfaceOptions struct {
	Name           string   // gateway | admin | client
	OriginPatterns []string // 浏览器 Origin 白名单（空=拒所有带 Origin 请求——非浏览器客户端直连）
	RequireHTTPS   bool     // 代理场景 X-Forwarded-Proto 校验（本地明文测试豁免）
	Auth           ChannelAuthorizer
	Hooks          Hooks                  // nil=Noop
	Broadcaster    Broadcaster            // nil=single 拓扑本地直投
	Presence       Presence               // nil=不启用（multi 才注入——§9 presence 写入面）
	Coord          Coordinator            // nil=single 无协调面（seq 进程内、App 维度限流直通）
	Metrics        *observability.Metrics // nil=不出指标（测试面）
	Logger         *slog.Logger           // nil=slog.Default
}

// Surface 是一个 WS 接入面（ticket 签发 + upgrade + 帧会话）。
type Surface struct {
	name        string
	upgrader    *ws.Upgrader
	tickets     TicketIssuer
	hub         *ws.Hub
	auth        ChannelAuthorizer
	hooks       Hooks
	broadcaster Broadcaster
	presence    Presence
	coord       Coordinator
	limits      Limits
	metrics     *observability.Metrics
	logger      *slog.Logger

	seqMu  sync.Mutex
	seqs   map[string]int64 // 频道内序号
	rateMu sync.Mutex
	rate   map[string]*chanRate // 每频道广播速率窗口（§6 广播速率——超限丢弃并计数）

	wg    sync.WaitGroup
	ctx   context.Context // Start 注入的生命周期 ctx（Authorize/投递用——不随单请求终结）
	clock func() time.Time
}

// chanRate 是每频道广播固定窗口计数。
type chanRate struct {
	window int64
	count  int
}

// connState 是每连接会话状态（速率窗口 + 活跃标记）。
type connState struct {
	rateWindow int64
	rateCount  int
	lastPong   atomic.Int64
}

// NewSurface 创建接入面（Name/Tickets/Hub/Auth 必填——缺任一=装配错误）。
func NewSurface(opts SurfaceOptions, tickets TicketIssuer, hub *ws.Hub, limits Limits) (*Surface, error) {
	if opts.Name == "" || tickets == nil || hub == nil || opts.Auth == nil {
		return nil, errors.New("realtime: surface missing name/tickets/hub/auth")
	}
	upgrader, err := ws.NewUpgrader(ws.Options{
		SurfaceName:    opts.Name, // ticket 面匹配（跨面 ticket 无效——§1）
		TicketStore:    tickets,
		OriginPatterns: opts.OriginPatterns,
		RequireHTTPS:   opts.RequireHTTPS,
	})
	if err != nil {
		return nil, fmt.Errorf("realtime: upgrader: %w", err)
	}
	if opts.Hooks == nil {
		opts.Hooks = NoopHooks{}
	}
	if limits.HeartbeatInterval <= 0 {
		limits.HeartbeatInterval = defaultHeartbeatInterval
	}
	if limits.IdleTimeout <= 0 {
		limits.IdleTimeout = defaultIdleTimeout
	}
	if limits.MaxChannelsPerConn <= 0 {
		limits.MaxChannelsPerConn = defaultMaxChannelsPerConn
	}
	if limits.MaxFrameBytes <= 0 {
		limits.MaxFrameBytes = defaultMaxFrameBytes
	}
	if limits.MessageRatePerSec <= 0 {
		limits.MessageRatePerSec = defaultMessageRatePerSec
	}
	if limits.BroadcastRatePerSec <= 0 {
		limits.BroadcastRatePerSec = limits.MessageRatePerSec * 10
	}
	if limits.SendQueue <= 0 {
		limits.SendQueue = ws.DefaultSendQueue
	}
	// 三限制原为零值旁路点（HV-1 根因：模块双结构体丢 defaults→恒 0→单 App
	// 连接上限不启用+App 维度限流分支旁路）；配置层已 fail-fast 拒非正值，
	// 此处按 §6 规范基线兜底（直接装配路径同样不失效）。
	if limits.MaxConnsPerPrincipal <= 0 {
		limits.MaxConnsPerPrincipal = defaultMaxConnsPerPrincipal
	}
	if limits.MaxConnsPerApp <= 0 {
		limits.MaxConnsPerApp = defaultMaxConnsPerApp
	}
	if limits.MaxConnsTotal == 0 {
		limits.MaxConnsTotal = defaultMaxConnsTotal
	}
	if limits.MaxMessageRatePerAppSec <= 0 {
		limits.MaxMessageRatePerAppSec = defaultMaxMessageRatePerAppSec
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	return &Surface{
		name: opts.Name, upgrader: upgrader, tickets: tickets, hub: hub,
		auth: opts.Auth, hooks: opts.Hooks, broadcaster: opts.Broadcaster,
		presence: opts.Presence, coord: opts.Coord, limits: limits, metrics: opts.Metrics,
		logger: opts.Logger, seqs: map[string]int64{}, rate: map[string]*chanRate{},
		ctx: context.Background(), clock: time.Now,
	}, nil
}

// seqCounterTTL 是频道序号的 Redis 计数器 TTL（滑动续期——持续活跃频道不
// 归零；覆盖 §7 续传窗口 5 分钟并留余量）。空闲到期=序号重置，与 A2 本地
// 清理语义对齐（at-most-once 语义内）。
const seqCounterTTL = 10 * time.Minute

// broadcastAllow 每频道广播窗口判定（超限 false——丢弃并计数不关连：广播面
// 是性能面，发布方非恶意主体）。
func (s *Surface) broadcastAllow(channel string) bool {
	s.rateMu.Lock()
	defer s.rateMu.Unlock()
	nowSec := s.clock().Unix()
	cr, ok := s.rate[channel]
	if !ok || nowSec != cr.window {
		s.rate[channel] = &chanRate{window: nowSec, count: 1}
		return true
	}
	cr.count++
	return cr.count <= s.limits.BroadcastRatePerSec
}

// AttachBroadcaster 构造后注入广播面（multi 工厂路径——回环投递经实现方回调
// DeliverLocal）。
func (s *Surface) AttachBroadcaster(b Broadcaster) { s.broadcaster = b }

// Start 注入生命周期 ctx（升级会话与投递挂其下；Stop 关停）。
func (s *Surface) Start(ctx context.Context) { s.ctx = ctx }

// Stop 关停全部连接（1001 Going Away——bootstrap BeforeShutdown）并等待会话
// goroutine 退出（含读循环：conn 由 drain 关连后 Read 即返；不用 CloseAll——
// 立即关连会截断排空，修正）。
func (s *Surface) Stop() {
	for _, c := range s.hub.All() {
		s.closeConn(c, protocol.CloseGoingAway, "server shutting down")
	}
	s.wg.Wait() // 读循环在 drain 关连后 Read 即返——hooks/metrics 不滞后
}

// —— HTTP 端点（ws-protocol §1） ——

// TicketHandler 是 POST /ws/tickets（挂在各 surface 鉴权组内——admin=JWT 组、
// gateway=edge HMAC 管线、client= User JWT）。签发绑定身份+允许频道集合。
func (s *Surface) TicketHandler() gin.HandlerFunc {
	type ticketRequest struct {
		Channels []string `json:"channels"`
	}
	return func(c *gin.Context) {
		p := transport.PrincipalFrom(c)
		if p.ID == "" {
			transport.Fail(c, errorx.New(errorx.CodeUnauthorized))
			return
		}
		var req ticketRequest
		if c.Request.Body != nil && c.Request.ContentLength > 0 {
			if err := c.ShouldBindJSON(&req); err != nil {
				transport.Fail(c, errorx.New(errorx.CodeInvalidParams, "reason", "ticket body"))
				return
			}
		}
		if len(req.Channels) > s.limits.MaxChannelsPerConn {
			transport.Fail(c, errorx.New(errorx.CodeInvalidParams, "reason", "too many channels"))
			return
		}
		for _, ch := range req.Channels {
			if !channels.Valid(ch) {
				transport.Fail(c, errorx.New(errorx.CodeInvalidParams, "reason", "invalid channel"))
				return
			}
		}
		ticket, err := s.tickets.IssueBound(c.Request.Context(),
			ws.TicketGrant{Principal: p, Surface: s.name, Channels: req.Channels})
		if err != nil {
			s.logger.Error("realtime: ticket issue failed", "surface", s.name, "err", err)
			transport.Fail(c, errorx.New(errorx.CodeUnavailable))
			return
		}
		if s.metrics != nil {
			s.metrics.RealtimeTickets.WithLabelValues(s.name).Inc()
		}
		transport.OK(c, gin.H{"ticket": ticket, "expires_in": int(ws.TicketTTL.Seconds())})
	}
}

// UpgradeHandler 是 GET /ws（ticket 即一次性凭证——不挂 authn；公开组）。
func (s *Surface) UpgradeHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		conn, principal, bound, err := s.upgrader.Upgrade(c.Writer, c.Request)
		if err != nil {
			s.logger.Debug("realtime: upgrade failed", "surface", s.name, "err", err)
			return
		}
		s.serveConn(c.Request.Context(), conn, principal, bound, c.ClientIP())
	}
}

// MountTicket/MountUpgrade 组级便捷挂载（路径 /ws/tickets 与 /ws）。
func (s *Surface) MountTicket(g gin.IRouter)  { g.POST("/ws/tickets", s.TicketHandler()) }
func (s *Surface) MountUpgrade(g gin.IRouter) { g.GET("/ws", s.UpgradeHandler()) }

// —— 会话主循环 ——

// serveConn：注册→同身份上限→hello→发送泵+心跳器→读循环分派→清理。
// 本体在 handler goroutine（读循环）；wg 覆盖含清理段——Stop 等 drain 关连
// 后读循环即退，hooks/metrics 不滞后于 Stop 返回。
func (s *Surface) serveConn(reqCtx context.Context, conn *websocket.Conn, principal authn.Principal, bound []string, ip string) {
	s.wg.Add(1)
	defer s.wg.Done()
	connID := primitives.NewULID(primitives.SystemClock{})
	// 帧超限：coder/websocket 自动以 1009 关连（§6「或按策略关闭」的库内策略——
	// 4004 主动面留给显式路径）
	conn.SetReadLimit(s.limits.MaxFrameBytes)
	c := ws.NewConn(connID, principal, conn, s.limits.SendQueue)
	// 注册+同身份上限（ws-protocol §6：user 505、app 10,00013——超限关闭
	// 最旧同型连接；ULID=注册序）。A4：计数→摘除最旧→注册在 hub 锁内原子完成
	// ——并发注册窗口不再超限；摘除连接锁外关闭（网络 IO 不持 hub 锁），其会话
	// 清理走自身 defer 幂等收尾。
	kicked, err := s.hub.RegisterLimited(c, s.limits.MaxConnsPerPrincipal, s.limits.MaxConnsPerApp, s.limits.MaxConnsTotal)
	if err != nil {
		// 未注册无泵——Shutdown 无人消费=泄漏；直接关连（修正）
		c.Close(context.Background(), websocket.StatusCode(protocol.CloseServerErr), "register failed")
		return
	}
	for _, victim := range kicked {
		code, reason := protocol.CloseDuplicateConn, "duplicate connection limit"
		if victim.Principal.Type == authn.PrincipalApp {
			code, reason = protocol.CloseTryAgainLater, "app connection limit" // §6 单 App 超限 1013
		}
		s.closeConn(victim, code, reason)
	}
	if s.metrics != nil {
		s.metrics.RealtimeConns.WithLabelValues(s.name).Inc()
	}
	if s.presence != nil {
		if err := s.presence.Touch(s.ctx, s.name, principal.ID, connID); err != nil {
			s.logger.Warn("realtime: presence touch", "conn", connID, "err", err) // 在线面降级不断连
		}
	}
	s.hooks.OnConnect(context.WithoutCancel(reqCtx), s.name, principal, connID, ip)

	s.wg.Add(1)
	go func() { // 发送泵（panic 隔离——goroutine 惯例）
		defer s.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				s.logger.Error("realtime: writer panic", "surface", s.name, "conn", connID, "panic", r)
				c.Close(context.Background(), websocket.StatusCode(protocol.CloseServerErr), "internal error")
			}
		}()
		if err := c.WriterLoop(s.ctx); err != nil {
			s.logger.Warn("realtime: writer exit", "surface", s.name, "conn", connID, "err", err)
		}
	}()

	st := &connState{}
	st.lastPong.Store(s.clock().UnixMilli())
	s.wg.Add(1)
	go func() { // 心跳器：周期 ping + 空闲检测（无活跃 → 1001；panic 隔离）
		defer s.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				s.logger.Error("realtime: heartbeat panic", "surface", s.name, "conn", connID, "panic", r)
			}
		}()
		ticker := time.NewTicker(s.limits.HeartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-s.ctx.Done():
				return
			case <-c.Closed():
				return
			case <-ticker.C:
				if s.clock().UnixMilli()-st.lastPong.Load() > s.limits.IdleTimeout.Milliseconds() {
					s.closeConn(c, protocol.CloseGoingAway, "idle timeout")
					return
				}
				if s.presence != nil { // presence 心跳 TTL 续期（§9）
					if err := s.presence.Touch(s.ctx, s.name, c.Principal.ID, connID); err != nil {
						s.logger.Warn("realtime: presence heartbeat", "conn", connID, "err", err)
					}
				}
				ping := protocol.New(protocol.TypePing)
				ping.ID = primitives.NewULID(primitives.SystemClock{})
				_ = c.Send(mustFrame(ping))
			}
		}
	}()

	// hello（§1：protocol_version/conn_id/heartbeat_interval 毫秒）
	hello := protocol.New(protocol.TypeHello)
	hello.ID = primitives.NewULID(primitives.SystemClock{})
	hello.Payload = mustJSON(map[string]any{
		"conn_id":            connID,
		"protocol_version":   protocol.SchemaVersion,
		"heartbeat_interval": int(s.limits.HeartbeatInterval.Milliseconds()),
	})
	_ = c.Send(mustFrame(hello))

	closeCode, closeReason := protocol.CloseNormal, "client closed"
	defer func() {
		chans := s.hub.ConnChannels(connID) // Unregister 前快照（hooks 频道面）
		if s.broadcaster != nil {
			for _, ch := range chans {
				s.broadcaster.UnrefChannel(ch) // 广播订阅跟随本地订阅面
			}
		}
		if s.presence != nil {
			if err := s.presence.Remove(s.ctx, s.name, principal.ID, connID); err != nil {
				s.logger.Warn("realtime: presence remove", "conn", connID, "err", err)
			}
		}
		s.hub.Unregister(connID)
		s.dropChannelStateIfEmpty(chans) // A2：连接关闭路径同步清理每频道条目
		if s.metrics != nil {
			s.metrics.RealtimeConns.WithLabelValues(s.name).Dec()
		}
		s.hooks.OnDisconnect(context.WithoutCancel(reqCtx), s.name, principal, connID, ip, closeCode, closeReason, chans)
		if s.metrics != nil {
			s.metrics.RealtimeClose.WithLabelValues(s.name, strconv.Itoa(closeCode)).Inc()
		}
	}()

	for {
		_, raw, err := conn.Read(reqCtx)
		if err != nil {
			closeCode, closeReason = readErrCode(err)
			break
		}
		var env protocol.Envelope
		perr := json.Unmarshal(raw, &env)
		typ := env.Type
		if perr != nil || typ == "" {
			typ = "invalid"
		}
		if s.metrics != nil {
			s.metrics.RealtimeMessages.WithLabelValues(s.name, "in", typ).Inc()
		}
		if perr != nil || typ == "invalid" {
			s.sendError(c, "realtime.protocol_violation", "")
			closeCode, closeReason = protocol.CloseProtocolErr, "malformed frame"
			break
		}
		code, reason, stop := s.handleFrame(c, principal, bound, env, st)
		if stop {
			closeCode, closeReason = code, reason
			break
		}
	}
	s.closeConn(c, closeCode, closeReason)
}

// handleFrame 分派一帧（stop=true 表示按码关闭连接）。
func (s *Surface) handleFrame(c *ws.Conn, principal authn.Principal, bound []string,
	env protocol.Envelope, st *connState) (closeCode int, reason string, stop bool) {

	switch env.Type {
	case protocol.TypePing:
		st.lastPong.Store(s.clock().UnixMilli()) // ping/pong 同为对端存活证据（空闲语义=连接死）
		pong := protocol.New(protocol.TypePong)
		pong.ID = primitives.NewULID(primitives.SystemClock{})
		pong.TS = env.TS
		_ = c.Send(mustFrame(pong))
		return 0, "", false
	case protocol.TypePong:
		st.lastPong.Store(s.clock().UnixMilli())
		return 0, "", false
	case protocol.TypeSubscribe:
		return s.handleSubscribe(c, principal, bound, env)
	case protocol.TypeUnsubscribe:
		return s.handleUnsubscribe(c, env)
	case protocol.TypeMessage:
		return s.handleMessage(c, principal, bound, env, st)
	case protocol.TypeAck:
		return 0, "", false // at-most-once：ack 仅客户端幂等记账（V1 不追投）
	case protocol.TypeResume:
		// 续传是可选协议扩展，本版本不提供消息窗口——resume 返回
		// resume_unavailable 错误帧（客户端重订阅；强一致业务走 HTTP 拉取）。
		s.sendError(c, "realtime.resume_unavailable", env.Channel)
		return 0, "", false
	default:
		s.sendError(c, "realtime.protocol_violation", "")
		return protocol.CloseProtocolErr, "unknown frame type", true
	}
}

// handleSubscribe：格式→ticket 频道绑定→ACL（第一级）→数量上限→注册。
func (s *Surface) handleSubscribe(c *ws.Conn, principal authn.Principal, bound []string, env protocol.Envelope) (int, string, bool) {
	if !channels.Valid(env.Channel) {
		s.sendError(c, "realtime.invalid_channel", env.Channel)
		return 0, "", false
	}
	before := s.hub.ConnChannels(c.ID) // 幂等订阅判定（已订阅不重复 Ref 广播键）
	if len(bound) > 0 && !contains(bound, env.Channel) {
		s.denySubscribe(c, env.Channel)
		return protocol.CloseChannelDenied, "channel not in ticket grant", true
	}
	if err := s.auth.Authorize(s.ctx, principal, env.Channel, ActionSubscribe); err != nil {
		s.denySubscribe(c, env.Channel)
		return protocol.CloseChannelDenied, "channel denied", true
	}
	if chans := s.hub.ConnChannels(c.ID); len(chans) >= s.limits.MaxChannelsPerConn {
		s.sendError(c, "realtime.channel_limit", env.Channel)
		return 0, "", false // §6 超限 error+拒绝订阅（不关连）
	}
	if err := s.hub.Subscribe(c.ID, env.Channel); err != nil {
		s.logger.Warn("realtime: subscribe failed", "conn", c.ID, "err", err)
		return protocol.CloseServerErr, "subscribe failed", true
	}
	if s.broadcaster != nil && !contains(before, env.Channel) {
		s.broadcaster.RefChannel(env.Channel) // 本地新订阅者 → 订阅广播键
	}
	sub := protocol.New(protocol.TypeSubscribed)
	sub.ID = primitives.NewULID(primitives.SystemClock{})
	sub.Channel = env.Channel
	sub.Seq = s.channelSeq(env.Channel)
	_ = c.Send(mustFrame(sub))
	return 0, "", false
}

// handleUnsubscribe：不存在 → error 帧；成功 → unsubscribed。
func (s *Surface) handleUnsubscribe(c *ws.Conn, env protocol.Envelope) (int, string, bool) {
	if err := s.hub.Unsubscribe(c.ID, env.Channel); err != nil {
		s.sendError(c, "realtime.not_subscribed", env.Channel)
		return 0, "", false
	}
	if s.broadcaster != nil {
		s.broadcaster.UnrefChannel(env.Channel)
	}
	s.dropChannelStateIfEmpty([]string{env.Channel}) // A2：零订阅者即清理每频道条目
	un := protocol.New(protocol.TypeUnsubscribed)
	un.ID = primitives.NewULID(primitives.SystemClock{})
	un.Channel = env.Channel
	_ = c.Send(mustFrame(un))
	return 0, "", false
}

// handleMessage：上行业务消息（速率→格式→ticket 绑定→publish ACL→频道广播）。
func (s *Surface) handleMessage(c *ws.Conn, principal authn.Principal, bound []string,
	env protocol.Envelope, st *connState) (int, string, bool) {

	// 速率（§6 每连接 N/s；固定窗口计数——超限 4003 关闭）
	nowSec := s.clock().Unix()
	if nowSec != st.rateWindow {
		st.rateWindow = nowSec
		st.rateCount = 0
	}
	st.rateCount++
	if st.rateCount > s.limits.MessageRatePerSec {
		return protocol.CloseRateLimited, "message rate exceeded", true
	}
	// App 维度限流（§6 开放平台应用级控制）：gateway surface 的 app principal
	// 在连接级之上追加 per-app 滑窗——策略名与 HTTP 面一致（gateway_app，语义
	// 统一）；维度键加 ws: 前缀区分「WS 上行消息速率」与「HTTP 请求配额」。
	// Coord=nil（single 无 Redis）=直通（single 本地回退语义）；策略
	// fail_closed（contracts）——协调面不可用=拒绝（1013 可重试）。
	if principal.Type == authn.PrincipalApp && s.coord != nil && s.limits.MaxMessageRatePerAppSec > 0 {
		err := s.coord.AllowByPolicy(s.ctx, "gateway_app", "ws:"+principal.ID,
			s.limits.MaxMessageRatePerAppSec, time.Second)
		switch {
		case err == nil:
		case errors.Is(err, coordination.ErrDenied):
			return protocol.CloseRateLimited, "app message rate exceeded", true
		default:
			s.logger.Warn("realtime: app rate limit unavailable", "app", principal.ID, "err", err)
			return protocol.CloseTryAgainLater, "rate limiter unavailable", true
		}
	}
	if !channels.Valid(env.Channel) {
		s.sendError(c, "realtime.invalid_channel", env.Channel)
		return 0, "", false
	}
	if len(bound) > 0 && !contains(bound, env.Channel) {
		return protocol.CloseChannelDenied, "channel not in ticket grant", true
	}
	if err := s.auth.Authorize(s.ctx, principal, env.Channel, ActionPublish); err != nil {
		s.sendError(c, "realtime.channel_denied", env.Channel) // 与 subscribe 拒绝对称（error 帧先行）
		return protocol.CloseChannelDenied, "channel denied", true
	}
	seq, serr := s.nextSeq(s.ctx, env.Channel)
	if serr != nil {
		// multi counter fail_closed（contracts）：无法保证跨节点序号单调=不发
		// 错误序号；4008 关闭（服务端依赖不可用，非客户端违规）。
		s.logger.Warn("realtime: seq counter unavailable", "channel", env.Channel, "err", serr)
		return protocol.CloseServerErr, "seq unavailable", true
	}
	msg := protocol.New(protocol.TypeMessage)
	msg.ID = primitives.NewULID(primitives.SystemClock{})
	msg.Channel = env.Channel
	msg.Payload = env.Payload
	msg.Seq = seq
	s.publish(env.Channel, mustFrame(msg))
	return 0, "", false
}

// publish 是频道投递统一出口：multi=仅广播面（本节点经订阅回环收取
// ——统一路径防双投）；single=nil broadcaster 本地直投。广播失败=性能面
// fail-open：本地兜底+计数（§8 at-most-once 不放大）。
func (s *Surface) publish(channel string, frame []byte) {
	if !s.broadcastAllow(channel) {
		if s.metrics != nil {
			s.metrics.RealtimeDropped.WithLabelValues(s.name, "broadcast_rate").Inc()
		}
		return // §6 广播速率超限=按策略丢弃并计数
	}
	if s.broadcaster == nil {
		s.deliver(channel, frame)
		return
	}
	if err := s.broadcaster.Publish(s.ctx, channel, frame); err != nil {
		if s.metrics != nil {
			s.metrics.RealtimeDropped.WithLabelValues(s.name, "broadcast_failed").Inc()
		}
		s.logger.Warn("realtime: broadcast publish failed, fallback local", "channel", channel, "err", err)
		s.deliver(channel, frame)
	}
}

// DeliverLocal 是广播回环的本地投递出口（RedisBroadcaster onMessage 回调——
// module 注入；两级 ACL 在 deliver 内复用）。
func (s *Surface) DeliverLocal(channel string, frame []byte) { s.deliver(channel, frame) }

// deliver 是频道广播出口（单节点本地投递； 多节点在此挂 Redis 广播）。
// 两级 ACL 之第二级：每订阅者发送前再校验（§2——权限变更后不可继续收）。
func (s *Surface) deliver(channel string, frame []byte) {
	for _, sub := range s.hub.SubscribersConn(channel) {
		if err := s.auth.Authorize(s.ctx, sub.Principal, channel, ActionDeliver); err != nil {
			if s.metrics != nil {
				s.metrics.RealtimeSubscribeDeny.WithLabelValues(s.name).Inc()
			}
			continue // 拒绝该订阅者（不踢——订阅面失效，等其自行退订/重订阅）
		}
		if err := sub.Send(frame); err != nil {
			if s.metrics != nil {
				s.metrics.RealtimeDropped.WithLabelValues(s.name, "send_blocked").Inc()
			}
			if errors.Is(err, ws.ErrSendBlocked) {
				s.kick(sub, protocol.CloseTryAgainLater, "slow consumer") // §6 满则断开 1013
			}
			continue
		}
		if s.metrics != nil {
			s.metrics.RealtimeMessages.WithLabelValues(s.name, "out", protocol.TypeMessage).Inc()
		}
	}
}

// kick 慢消费者断开（1013——经 Shutdown 排空路径：队列可能已有空位写 close 帧；
// 写不出则 drain 预算内中止直接关连。总耗时受 DrainBudget 约束）。
func (s *Surface) kick(c *ws.Conn, code int, reason string) {
	if s.metrics != nil {
		s.metrics.RealtimeDropped.WithLabelValues(s.name, "slow_consumer").Inc()
	}
	s.closeConn(c, code, reason)
}

// closeConn 优雅关闭：close 帧由写泵在队列排空后写出（顺序保证——先收业务
// 帧尾再收 close），随后关连（幂等）。kick 慢消费者走 Conn.Close（队列满，
// close 帧不可达——1013 语义以关连承载）。
func (s *Surface) closeConn(c *ws.Conn, code int, reason string) {
	closeEnv := protocol.New(protocol.TypeClose)
	closeEnv.ID = primitives.NewULID(primitives.SystemClock{})
	closeEnv.CloseCode = code
	closeEnv.Reason = reason
	c.Shutdown(mustFrame(closeEnv), websocket.StatusCode(code), reason)
}

func (s *Surface) denySubscribe(c *ws.Conn, channel string) {
	if s.metrics != nil {
		s.metrics.RealtimeSubscribeDeny.WithLabelValues(s.name).Inc()
	}
	s.sendError(c, "realtime.channel_denied", channel)
}

// sendError 是 error 帧出口（code 由 surface+语义键经 protocol.ErrorFrameCode
// 映射登记码——ws-protocol §4 各面段，禁止字面量）。
func (s *Surface) sendError(c *ws.Conn, messageKey, channel string) {
	env := protocol.New(protocol.TypeError)
	env.ID = primitives.NewULID(primitives.SystemClock{})
	env.Code = protocol.ErrorFrameCode(s.name, messageKey)
	env.MessageKey = messageKey
	env.Channel = channel
	_ = c.Send(mustFrame(env))
}

// channelSeq 读当前频道序号（subscribe 响应；不推进）。读面=本节点本地镜像
// （每次 nextSeq 生成时同步——订阅回执 informational；权威计数在
// coordination.IncrSequence，多节点各自的回执只反映本节点已生成值）。
func (s *Surface) channelSeq(channel string) int64 {
	s.seqMu.Lock()
	defer s.seqMu.Unlock()
	return s.seqs[channel]
}

// nextSeq 推进频道序号（单频道 FIFO——§8）。Coord 非 nil=coordination 滑动
// 计数器（Redis INCR+续期——multi 跨节点单调；single Redis 故障由 counter
// 策略回退本地）；Coord=nil（single 无 Redis）=进程内计数。本地 map 始终记录
// 本节点最近生成值（channelSeq 读面 + A2 清理跟踪）。counter 策略 multi
// fail_closed：返回错误由调用方关闭连接（序号不能失真）。
func (s *Surface) nextSeq(ctx context.Context, channel string) (int64, error) {
	if s.coord != nil {
		n, err := s.coord.IncrSequence(ctx, keys.Seq(channel), seqCounterTTL)
		if err != nil {
			return 0, err
		}
		s.seqMu.Lock()
		s.seqs[channel] = n
		s.seqMu.Unlock()
		return n, nil
	}
	s.seqMu.Lock()
	defer s.seqMu.Unlock()
	s.seqs[channel]++
	return s.seqs[channel], nil
}

// dropChannelStateIfEmpty 频道零本地订阅者时清理每频道状态条目（A2：seqs/rate
// 原先只增不删=内存泄漏；清理时机对照 hub.byChannel——最后一个订阅者离开）。
// map delete 幂等；并发重订阅下最坏 seq 归零重计——at-most-once 语义内。
func (s *Surface) dropChannelStateIfEmpty(channels []string) {
	for _, ch := range channels {
		if len(s.hub.SubscribersConn(ch)) > 0 {
			continue
		}
		s.seqMu.Lock()
		delete(s.seqs, ch)
		s.seqMu.Unlock()
		s.rateMu.Lock()
		delete(s.rate, ch)
		s.rateMu.Unlock()
	}
}

// —— 小帮手 ——

func mustJSON(v any) json.RawMessage {
	raw, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage("{}")
	}
	return raw
}

func mustFrame(env protocol.Envelope) []byte {
	raw, err := env.Marshal()
	if err != nil { // marshal 失败回退：error 帧（码走登记常量——禁字面量）
		fb := protocol.New(protocol.TypeError)
		fb.Code = errorx.CodeInternal
		fb.MessageKey = "realtime.marshal"
		raw, _ = fb.Marshal()
		return raw
	}
	return raw
}

// readErrCode 读错误 → 关闭码归类（EOF/取消=对端断/服务停——不归 4008 污染分布）。
func readErrCode(err error) (int, string) {
	var ce websocket.CloseError
	if errors.As(err, &ce) {
		return int(ce.Code), ce.Reason
	}
	if errors.Is(err, context.Canceled) {
		return protocol.CloseGoingAway, "server shutting down"
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return protocol.CloseNormal, "client closed"
	}
	return protocol.CloseServerErr, "read error"
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
