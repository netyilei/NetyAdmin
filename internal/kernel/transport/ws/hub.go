// hub.go：连接注册表骨架（TR-03）——conn_id 注册/注销/按身份定位 + 每连接
// 有界发送（满 → 关闭 1013 过载语义由 Hub 决策，本骨架只暴露 SendBlocked）。
// 频道/ACL/信封/心跳在 realtime 平台服务——本骨架是连接生命周期的
// 最小公共面（握手产物 → 注册 → 读写循环挂接点 → 注销）。
package ws

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/netyilei/netyadmin/internal/kernel/security/authn"
)

// Conn 是注册表中的连接条目。
type Conn struct {
	ID        string          // conn_id（ULID；握手方生成传入）
	Principal authn.Principal // ticket 绑定身份
	conn      *websocket.Conn
	send      chan []byte // 有界发送队列（§6 背压下限）
	closed    chan struct{}
	once      sync.Once
	// shutdown 段（Shutdown 请求的 close 帧——泵在队列排空后写出再关连，
	// 保证 close 帧不被立即关连冲掉——audit-m6 协-A5 排空语义）
	shutdownMu     sync.Mutex
	shutdownFrame  []byte
	shutdownCode   websocket.StatusCode
	shutdownReason string
}

// DefaultSendQueue 是单连接发送队列默认上限（ws-protocol §6 规范值 64；
// 配置化经 realtime 模块 Config.SendQueue 注入——「限制必须通过配置」）。
const DefaultSendQueue = 64

// ErrSendBlocked 是发送队列满（调用方决策：丢弃计数/慢速踢出—— Hub 语义）。
var ErrSendBlocked = errors.New("ws: send queue blocked")

// ErrConnClosed 是连接已关（幂等判定面）。
var ErrConnClosed = errors.New("ws: connection closed")

// Hub 是连接注册表（进程内；多节点广播经 Bus）。
type Hub struct {
	mu        sync.RWMutex
	conns     map[string]*Conn
	channels  *Channels // 频道订阅表（channels.go）
	sendQueue int       // 单连接发送队列上限（配置注入）
}

// NewHub 创建（queueSize ≤0 取 DefaultSendQueue——Conn 构造须用相同队列）。
func NewHub(queueSize int) *Hub {
	if queueSize <= 0 {
		queueSize = DefaultSendQueue
	}
	return &Hub{conns: map[string]*Conn{}, channels: NewChannels(), sendQueue: queueSize}
}

// SendQueueSize 返回连接发送队列上限（Conn 构造方读取）。
func (h *Hub) SendQueueSize() int { return h.sendQueue }

// Channels 访问器（诊断面——订阅计数）。
func (h *Hub) ChannelStats() (channels, subscriptions int) {
	return h.channels.Count()
}

// Register 注册连接（conn_id 重复 fail——ULID 冲突=装配错误）。
func (h *Hub) Register(c *Conn) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, dup := h.conns[c.ID]; dup {
		return fmt.Errorf("ws: duplicate conn %s", c.ID)
	}
	h.conns[c.ID] = c
	return nil
}

// Unregister 注销（幂等；频道订阅同步清理——防幽灵订阅）。
func (h *Hub) Unregister(connID string) {
	h.mu.Lock()
	delete(h.conns, connID)
	h.mu.Unlock()
	h.channels.DropConn(connID)
}

// ErrConnsExhausted 节点级全局连接上限已满（调用侧映射 1013）。
var ErrConnsExhausted = errors.New("ws: node conns exhausted")

// RegisterLimited 注册并在同一临界区内执行同身份上限决策（ws-protocol §6：
// 超出关闭最旧连接——ULID=注册序）。计数按身份类型分型（§6 两行独立上限）：
// app principal 用 maxPerApp（单 App 10,000），其余类型用 maxPerPrincipal
// （单用户 5）；桶键=Type+ID（不同类型同 ID 互不影响）。
// 计数→摘除→注册原子完成（A4：拆开两步在并发注册窗口会超限）；摘除连接移出
// 注册表（立即不再计数/参与投递），由调用方锁外关闭——网络 IO 不持 hub 锁；
// 其频道订阅保留至自身注销路径清理（广播 Unref 依赖注销前快照），注销幂等。
// max≤0=该型不启用上限（等价 Register）。
func (h *Hub) RegisterLimited(c *Conn, maxPerPrincipal, maxPerApp, maxTotal int) ([]*Conn, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, dup := h.conns[c.ID]; dup {
		return nil, fmt.Errorf("ws: duplicate conn %s", c.ID)
	}
	// 节点级全局上限（资源上限红线——防多 App 连接累加耗尽内存；<=0=显式不限）。
	// 满=拒绝新连接（调用侧映射 1013 语义）。
	if maxTotal > 0 && len(h.conns) >= maxTotal {
		return nil, ErrConnsExhausted
	}
	max := maxPerPrincipal
	if c.Principal.Type == authn.PrincipalApp {
		max = maxPerApp
	}
	h.conns[c.ID] = c
	if max <= 0 {
		return nil, nil
	}
	var kicked []*Conn
	for {
		count := 0
		var oldest *Conn
		for _, pc := range h.conns {
			if pc.Principal.Type != c.Principal.Type || pc.Principal.ID != c.Principal.ID {
				continue
			}
			count++
			if pc.ID != c.ID && (oldest == nil || pc.ID < oldest.ID) {
				oldest = pc
			}
		}
		if count <= max || oldest == nil {
			return kicked, nil
		}
		delete(h.conns, oldest.ID)
		kicked = append(kicked, oldest)
	}
}

// Get 按 conn_id 取连接。
func (h *Hub) Get(connID string) (*Conn, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	c, ok := h.conns[connID]
	return c, ok
}

// ByPrincipal 按身份取全部连接（§6 同身份连接数上限由 surface 决策）。
func (h *Hub) ByPrincipal(id string) []*Conn {
	h.mu.RLock()
	defer h.mu.RUnlock()
	var out []*Conn
	for _, c := range h.conns {
		if c.Principal.ID == id {
			out = append(out, c)
		}
	}
	return out
}

// Count 是当前连接数（诊断面）。
func (h *Hub) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.conns)
}

// All 是全部连接快照（surface 优雅关停逐一 Shutdown 用）。
func (h *Hub) All() []*Conn {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]*Conn, 0, len(h.conns))
	for _, c := range h.conns {
		out = append(out, c)
	}
	return out
}

// SubscribersConn 是频道订阅者连接快照（realtime session 投递用——锁外发送
// 防持锁写；竞态注销中的连接由调用方 Get 判定跳过）。
func (h *Hub) SubscribersConn(channel string) []*Conn {
	h.mu.RLock()
	defer h.mu.RUnlock()
	var out []*Conn
	for id := range h.channels.byChannel[channel] {
		if c, ok := h.conns[id]; ok {
			out = append(out, c)
		}
	}
	return out
}

// CloseAll 关停全部连接（bootstrap BeforeShutdown 排空；§5 1001 Going Away
// 语义——关闭码常量归 realtime，此处用 websocket 规范码）。
func (h *Hub) CloseAll(ctx context.Context) {
	h.mu.RLock()
	conns := make([]*Conn, 0, len(h.conns))
	for _, c := range h.conns {
		conns = append(conns, c)
	}
	h.mu.RUnlock()
	for _, c := range conns {
		c.Close(ctx, websocket.StatusGoingAway, "server shutting down")
	}
}

// Send 入队待发帧（非阻塞；满 → ErrSendBlocked——调用方决定丢弃计数/踢出）。
func (c *Conn) Send(frame []byte) error {
	select {
	case <-c.closed:
		return ErrConnClosed
	default:
	}
	select {
	case c.send <- frame:
		return nil
	default:
		return ErrSendBlocked
	}
}

// WriterLoop 是每连接的发送泵（goroutine 惯例：调用方 wg 托管 + panic 隔离；
// 从 send 队列顺序写帧，ctx 取消/连接断开退出）。写失败以错误返回——调用方
// （realtime session）决策踢出/计数，本层不静默吞（reliability §6）。
// 退出前排空队列剩余帧 + 写出 Shutdown close 帧（best-effort——audit-m6
// 协-A5 关停尾帧排空；对端已断时写失败即丢，at-most-once 语义不放大）。
func (c *Conn) WriterLoop(ctx context.Context) error {
	defer c.once.Do(func() { close(c.closed) })
	defer c.drain()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-c.closed:
			return nil
		case frame := <-c.send:
			wctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			err := c.conn.Write(wctx, websocket.MessageText, frame)
			cancel()
			if err != nil {
				return fmt.Errorf("ws: write frame conn %s: %w", c.ID, err)
			}
		}
	}
}

// DrainBudget 是排空总预算（超时即中止排空直接关连——慢消费者满队列时
// 防泵 goroutine 与 Stop().wg.Wait() 被逐帧 2s 拖到无界）。
const DrainBudget = 5 * time.Second

// drain 排空发送队列剩余帧，随后写出 Shutdown close 帧并关连（顺序保证：
// close 帧永远在既有队列帧之后——客户端先收业务帧尾再收 close；总预算
// DrainBudget 内完不成即中止——at-most-once 不放大）。
func (c *Conn) drain() {
	deadline := time.Now().Add(DrainBudget)
	writeFrame := func(frame []byte) bool {
		remain := time.Until(deadline)
		if remain <= 0 {
			return false
		}
		wctx, cancel := context.WithTimeout(context.Background(), remain)
		_ = c.conn.Write(wctx, websocket.MessageText, frame)
		cancel()
		return true
	}
	for {
		select {
		case frame := <-c.send:
			if !writeFrame(frame) {
				c.shutdownMu.Lock()
				code, reason := c.shutdownCode, c.shutdownReason
				c.shutdownMu.Unlock()
				_ = c.conn.Close(code, reason)
				return
			}
		default:
			c.shutdownMu.Lock()
			frame, code, reason := c.shutdownFrame, c.shutdownCode, c.shutdownReason
			c.shutdownMu.Unlock()
			if frame != nil && writeFrame(frame) {
				_ = c.conn.Close(code, reason)
			}
			return
		}
	}
}

// Shutdown 请求优雅关闭：close 帧由写泵在队列排空后写出，随后关连（幂等；
// frame=nil 时仅关连——异常路径）。与 Close（立即关连——慢消费者踢出/关停
// 兜底）分工：常规关闭走本方法保证 close 帧送达。
func (c *Conn) Shutdown(frame []byte, code websocket.StatusCode, reason string) {
	c.shutdownMu.Lock()
	if c.shutdownFrame == nil {
		c.shutdownFrame, c.shutdownCode, c.shutdownReason = frame, code, reason
	}
	c.shutdownMu.Unlock()
	c.once.Do(func() { close(c.closed) })
}

// Close 关闭连接（幂等；code/reason 由协议常量承载——本骨架收 lifecycle）。
func (c *Conn) Close(ctx context.Context, code websocket.StatusCode, reason string) {
	c.once.Do(func() { close(c.closed) })
	if c.conn != nil {
		_ = c.conn.Close(code, reason) // 对端已断时 Close 失败无补救路径
	}
}

// NewConn 构造连接条目（surface 会话用——queueSize ≤0 取默认；
// conn/send/closed 不导出，session 包经本构造器组装）。
func NewConn(id string, principal authn.Principal, conn *websocket.Conn, queueSize int) *Conn {
	if queueSize <= 0 {
		queueSize = DefaultSendQueue
	}
	return &Conn{ID: id, Principal: principal, conn: conn,
		send: make(chan []byte, queueSize), closed: make(chan struct{})}
}

// Closed 返回连接关闭信号（会话层 select 用——幂等关闭后即刻触发）。
func (c *Conn) Closed() <-chan struct{} { return c.closed }
