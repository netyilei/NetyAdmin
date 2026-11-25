// Package realtime 是实时通信模块：Hub 实例持有 + 频道面
// （订阅注册在 kernel/ws Hub——协议常量/ACL/信封语义在本模块）+ ticket 存储
// 装配（Redis GETDEL 一次性）+ 关停排空。 endpoints（三端 upgrade）随
// 多节点广播Bus 面随。
package realtime

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/netyilei/netyadmin/internal/kernel/config"
	"github.com/netyilei/netyadmin/internal/kernel/module"
	"github.com/netyilei/netyadmin/internal/kernel/state/coordination"
	kernsession "github.com/netyilei/netyadmin/internal/kernel/state/session"
	"github.com/netyilei/netyadmin/internal/kernel/transport/ws"
	"github.com/netyilei/netyadmin/internal/modules/realtime/api/keys"
	"github.com/netyilei/netyadmin/internal/modules/realtime/session"
)

// Module 实现 kernel/module 契约。
type Module struct {
	logger *slog.Logger
	// cfg 是 kernel/config.Realtime（唯一配置源——defaults/env/yaml 三层在
	// config.Load 收敛；模块不再自建重复结构体，HV-1 根因：旧 ConfigBinder 只
	// 绑定 config.yaml 原始段，defaults/env 全部丢失→连接/限流上限=0 静默失效）。
	cfg          config.Realtime
	hub          *ws.Hub
	cli          *redis.Client
	coord        *coordination.Coordination // 协调面（nil=single 无 Redis——seq 进程内/App 限流直通）
	root         string                     // na:{env}（广播/presence 键段根——keys 唯一封装）
	topology     kernsession.Topology
	tickets      *ws.RedisTicketStore
	broadcasters []*RedisBroadcaster // per-surface 广播面（CloseAll 关停）
}

// New 构造（cfg=kernel/config.Realtime 唯一配置源——defaults/env 已由 config
// 层应用；redisAddr 空=无 ticket 面——WS 不可用，surface fail-closed 拒绝；
// namespaceRoot=na:{env} 键根（ticket/广播/presence 段经 keys 唯一封装派生）；
// topology=部署拓扑——multi 才挂广播/presence 面；coord=nil=single
// 无协调面，seq 进程内计数/App 维度限流直通）。
func New(logger *slog.Logger, coord *coordination.Coordination, redisAddr, namespaceRoot string, topology kernsession.Topology, cfg config.Realtime) *Module {
	m := &Module{logger: logger, cfg: cfg, hub: ws.NewHub(cfg.SendQueue), coord: coord,
		root: namespaceRoot, topology: topology}
	if redisAddr != "" {
		m.cli = redis.NewClient(&redis.Options{Addr: redisAddr})
		m.tickets = ws.NewRedisTicketStore(m.cli, namespaceRoot+":"+keys.TicketNS)
	}
	return m
}

// Cfg 访问器（kernel/config.Realtime——组合根开关判定消费）。
func (m *Module) Cfg() config.Realtime { return m.cfg }

// Name 实现 Module。
func (m *Module) Name() string { return "realtime" }

// DependsOn 实现 Module（无业务模块依赖）。
func (m *Module) DependsOn() []string { return nil }

// Init 纯装配：端口 Expose（Hub/TicketStore——三端 surface 消费）。配置在 New
// 注入（kernel/config.Realtime 单一源），此处不再绑定原始 YAML 段——旧
// ConfigBinder 绑定面只含 config.yaml 显式字段，defaults/env 全丢（HV-1 根因）。
func (m *Module) Init(_ context.Context, r *module.Registry) error {
	// Enabled=false：WS 面关闭（不 Expose Hub/TicketStore——surface 解析失败=
	// 零开销；评审 A1 根修：此前 Enabled 为死配置）
	if !m.cfg.Enabled {
		return nil
	}
	if err := r.Services.Expose("realtime.Hub", m.hub); err != nil {
		return fmt.Errorf("realtime: expose hub: %w", err)
	}
	if m.tickets != nil {
		if err := r.Services.Expose("realtime.TicketStore", m.tickets); err != nil {
			return fmt.Errorf("realtime: expose tickets: %w", err)
		}
	}
	return nil
}

// Hub 访问器。
func (m *Module) Hub() *ws.Hub { return m.hub }

// Surface 构造一个 WS 接入面（——三端共用机制；surface 特化经
// originPatterns/authorizer 注入）。Enabled=false 或无 ticket 面 → 错误
// （surface 装配失败=WS 不可用，fail-closed——调用方决定是否致命）。
func (m *Module) Surface(opts session.SurfaceOptions) (*session.Surface, error) {
	if !m.cfg.Enabled {
		return nil, fmt.Errorf("realtime: disabled (config.realtime.enabled=false)")
	}
	if m.tickets == nil {
		return nil, fmt.Errorf("realtime: no ticket store (redis addr empty)")
	}
	if m.topology == kernsession.TopologyMulti && m.cli != nil {
		opts.Presence = NewRedisPresence(m.cli, m.root,
			time.Duration(m.cfg.IdleTimeout)*time.Second+time.Minute) // §9 presence 心跳 TTL
	}
	limits := session.Limits{
		HeartbeatInterval:       time.Duration(m.cfg.HeartbeatInterval) * time.Second,
		IdleTimeout:             time.Duration(m.cfg.IdleTimeout) * time.Second,
		MaxChannelsPerConn:      m.cfg.MaxChannelsPerConn,
		MaxFrameBytes:           int64(m.cfg.MaxFrameBytes),
		MessageRatePerSec:       m.cfg.MessageRatePerSec,
		MaxMessageRatePerAppSec: m.cfg.MaxMessageRatePerAppSec,
		SendQueue:               m.cfg.SendQueue,
		MaxConnsPerPrincipal:    m.cfg.MaxConnsPerPrincipal,
		MaxConnsPerApp:          m.cfg.MaxConnsPerApp,
		MaxConnsTotal:           m.cfg.MaxConnsTotal,
	}
	opts.Coord = m.coord // nil=single 无协调面（seq 进程内/App 限流直通）
	surf, err := session.NewSurface(opts, m.tickets, m.hub, limits)
	if err != nil {
		return nil, err
	}
	if m.topology == kernsession.TopologyMulti && m.cli != nil {
		// 广播面（multi；deliver 回环=surface.DeliverLocal——统一投递路径）
		b := NewRedisBroadcaster(m.cli, m.root, opts.Name, surf.DeliverLocal, opts.Metrics, opts.Logger)
		surf.AttachBroadcaster(b)
		m.broadcasters = append(m.broadcasters, b)
	}
	return surf, nil
}

// Tickets 访问器（nil=无 Redis——surface 应拒绝 upgrade）。
func (m *Module) Tickets() *ws.RedisTicketStore { return m.tickets }

// CloseAll 关停排空（bootstrap BeforeShutdown——1001 Going Away 语义；nil 守卫；
// 广播订阅连接随 hub 排空后关闭）。
func (m *Module) CloseAll(ctx context.Context) {
	if m.hub != nil {
		m.hub.CloseAll(ctx)
	}
	for _, b := range m.broadcasters {
		b.Close()
	}
}
