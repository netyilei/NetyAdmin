// broadcast.go： 跨节点广播/在线状态（ws-protocol §9——Redis Pub/Sub）。
// 广播键 na:{env}:realtime:ch:{channel}（§9 字面）；presence 键
// na:{env}:realtime:presence:{surface}:{id}（SET conn 集合+心跳 TTL）。
// 节点只订阅本地有订阅者的频道（Ref/Unref 引用计数跟随 session 订阅面）：
// refs 判定锁内原子，Redis SUBSCRIBE/UNSUBSCRIBE 由回环 goroutine 串行对账
// 执行（终态=refs 终态——A1：并发退订+订阅交错不会撤销仍需要的订阅）。
// 回环 panic/失联自愈（A3）：异常退出后指数退避重连重订阅，连续失败封顶。
package realtime

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/netyilei/netyadmin/internal/kernel/observability"
	"github.com/netyilei/netyadmin/internal/modules/realtime/api/keys"
)

// 广播回环自愈参数（A3——指数退避，连续失败封顶；测试可注入覆写）。
const (
	loopHealthyWindow = 30 * time.Second       // 回环健康运行窗口（超过则失败计数清零防紧循环）
	loopReconnectBase = 100 * time.Millisecond // 重连退避基数
	loopReconnectMax  = 30 * time.Second       // 重连退避上限
)

// RedisBroadcaster 实现 session.Broadcaster（multi 拓扑）。
type RedisBroadcaster struct {
	cli     *redis.Client
	prefix  string // na:{env}:realtime:ch:
	surface string
	deliver func(channel string, frame []byte) // 广播回环本地投递（surface.DeliverLocal）
	metrics *observability.Metrics
	logger  *slog.Logger

	ps    *redis.PubSub
	msgCh <-chan *redis.Message // ps.Channel()——构造期一次性创建（回环重建复用同一 ps）

	mu         sync.Mutex
	refs       map[string]int
	subscribed map[string]struct{} // Redis 侧已订阅频道（仅回环 goroutine 读写——对账基准）

	wg         sync.WaitGroup
	stop       chan struct{}
	wake       chan struct{} // refs 变更信号（cap 1 幂等——终态由 refs 决定）
	rebuilds   atomic.Int64  // 回环异常重建次数（观测/测试面）
	healthyWin time.Duration
	reconnBase time.Duration
	reconnMax  time.Duration
}

// NewRedisBroadcaster 创建并启动订阅回环（deliver=本地面投递回调）。
func NewRedisBroadcaster(cli *redis.Client, namespaceRoot, surface string,
	deliver func(channel string, frame []byte), metrics *observability.Metrics, logger *slog.Logger) *RedisBroadcaster {

	b := &RedisBroadcaster{
		cli: cli, prefix: namespaceRoot + ":" + keys.BroadcastNS + ":", surface: surface,
		deliver: deliver, metrics: metrics, logger: logger,
		ps: cli.Subscribe(context.Background()), refs: map[string]int{},
		subscribed: map[string]struct{}{},
		stop:       make(chan struct{}), wake: make(chan struct{}, 1),
		healthyWin: loopHealthyWindow, reconnBase: loopReconnectBase, reconnMax: loopReconnectMax,
	}
	b.msgCh = b.ps.Channel()
	b.wg.Add(1)
	go b.loop()
	return b
}

// Publish 实现 session.Broadcaster（PUBLISH；本节点订阅连接回环收取）。
func (b *RedisBroadcaster) Publish(ctx context.Context, channel string, frame []byte) error {
	return b.cli.Publish(ctx, b.prefix+channel, frame).Err()
}

// RefChannel 实现 session.Broadcaster（refs 锁内原子 +1 → 唤醒对账）。
// A1：refs 判定与 Redis SUBSCRIBE 不再分两步无序执行——回环按 refs 终态对账，
// 并发退订+订阅交错后订阅态始终收敛=refs（原缺陷：交错后 refs=1 而订阅已撤销）。
func (b *RedisBroadcaster) RefChannel(channel string) {
	b.mu.Lock()
	b.refs[channel]++
	b.mu.Unlock()
	b.nudge()
}

// UnrefChannel 实现 session.Broadcaster（refs 锁内原子 -1 → 唤醒对账；钳位防负）。
func (b *RedisBroadcaster) UnrefChannel(channel string) {
	b.mu.Lock()
	n := b.refs[channel] - 1
	if n <= 0 {
		delete(b.refs, channel)
	} else {
		b.refs[channel] = n
	}
	b.mu.Unlock()
	b.nudge()
}

// nudge 唤醒回环对账（cap 1 幂等信号——合并丢失无害，终态由 refs 决定）。
func (b *RedisBroadcaster) nudge() {
	select {
	case b.wake <- struct{}{}:
	default:
	}
}

// desiredChannels refs 期望订阅态快照（锁内）。
func (b *RedisBroadcaster) desiredChannels() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]string, 0, len(b.refs))
	for ch := range b.refs {
		out = append(out, ch)
	}
	return out
}

// reconcile 将 Redis 订阅收敛到 refs 期望态（A1：refs 判定锁内原子、Redis 操作
// 由回环单 goroutine 串行执行——终态一致；幂等，重复调用无害）。仅回环 goroutine。
func (b *RedisBroadcaster) reconcile() error {
	ctx := context.Background()
	desired := b.desiredChannels()
	desiredSet := make(map[string]struct{}, len(desired))
	var toSub []string
	for _, ch := range desired {
		desiredSet[ch] = struct{}{}
		if _, ok := b.subscribed[ch]; !ok {
			toSub = append(toSub, b.prefix+ch)
		}
	}
	if len(toSub) > 0 {
		if err := b.ps.Subscribe(ctx, toSub...); err != nil {
			return fmt.Errorf("realtime: broadcast subscribe: %w", err)
		}
		for _, ch := range desired {
			if _, ok := b.subscribed[ch]; !ok {
				b.subscribed[ch] = struct{}{}
			}
		}
	}
	var toUnsub []string
	for ch := range b.subscribed {
		if _, ok := desiredSet[ch]; !ok {
			toUnsub = append(toUnsub, b.prefix+ch)
		}
	}
	if len(toUnsub) > 0 {
		if err := b.ps.Unsubscribe(ctx, toUnsub...); err != nil {
			return fmt.Errorf("realtime: broadcast unsubscribe: %w", err)
		}
		for _, full := range toUnsub {
			delete(b.subscribed, strings.TrimPrefix(full, b.prefix))
		}
	}
	return nil
}

// resubscribeAll 全量重订阅 refs 期望态（A3——panic/失联后重连重订阅：ps 断连
// 时 go-redis 在下一命令自动重连；重置对账基准并显式重发 SUBSCRIBE 保证服务端
// 订阅态收敛）。仅回环 goroutine。
func (b *RedisBroadcaster) resubscribeAll() error {
	desired := b.desiredChannels()
	b.subscribed = make(map[string]struct{}, len(desired))
	if len(desired) == 0 {
		return nil
	}
	full := make([]string, 0, len(desired))
	for _, ch := range desired {
		full = append(full, b.prefix+ch)
	}
	if err := b.ps.Subscribe(context.Background(), full...); err != nil {
		return fmt.Errorf("realtime: broadcast resubscribe: %w", err)
	}
	for _, ch := range desired {
		b.subscribed[ch] = struct{}{}
	}
	return nil
}

// loop 是订阅回环（A3：异常退出后指数退避重建——重连重订阅；连续失败封顶，
// 健康窗口运行后失败计数清零防紧循环）。
func (b *RedisBroadcaster) loop() {
	defer b.wg.Done()
	failures := 0
	for {
		ran := b.pump()
		if b.isStopped() {
			return
		}
		b.rebuilds.Add(1)
		if ran >= b.healthyWin {
			failures = 0
		} else {
			failures++
		}
		delay := b.backoffDelay(failures)
		b.logger.Warn("realtime: broadcast loop rebuild",
			"ran", ran.String(), "failures", failures, "delay", delay.String())
		select {
		case <-b.stop:
			return
		case <-time.After(delay):
		}
		if err := b.resubscribeAll(); err != nil {
			b.logger.Error("realtime: broadcast loop resubscribe", "err", err)
		}
	}
}

// pump 是回环单轮运行（stop/对账/投递分派）。panic 与 ps 关闭均视为异常退出
// ——recover 在此隔离并由 loop 重建（A3：不再仅记日志后永久死亡）；返回健康
// 运行时长供退避判定。
func (b *RedisBroadcaster) pump() time.Duration {
	start := time.Now()
	defer func() {
		if r := recover(); r != nil {
			b.logger.Error("realtime: broadcast loop panic", "panic", r)
		}
	}()
	for {
		select {
		case <-b.stop:
			return time.Since(start)
		case <-b.wake:
			if err := b.reconcile(); err != nil {
				b.logger.Warn("realtime: broadcast reconcile", "err", err)
			}
		case msg, ok := <-b.msgCh:
			if !ok {
				return time.Since(start) // ps 关闭（正常关停由 loop 判定退出）
			}
			channel := strings.TrimPrefix(msg.Channel, b.prefix)
			if channel == msg.Channel {
				continue // 非本命名空间键（防御）
			}
			b.deliver(channel, []byte(msg.Payload))
		}
	}
}

// backoffDelay 指数退避（failures=1 取基数；倍增至 reconnMax 封顶）。
func (b *RedisBroadcaster) backoffDelay(failures int) time.Duration {
	d := b.reconnBase
	for i := 1; i < failures && d < b.reconnMax; i++ {
		d *= 2
	}
	if d > b.reconnMax {
		d = b.reconnMax
	}
	return d
}

func (b *RedisBroadcaster) isStopped() bool {
	select {
	case <-b.stop:
		return true
	default:
		return false
	}
}

// Close 关停（bootstrap BeforeShutdown——ps 关闭终止回环）。
func (b *RedisBroadcaster) Close() {
	close(b.stop)
	_ = b.ps.Close()
	b.wg.Wait()
}

// RedisPresence 实现 session.Presence（multi 拓扑；§9 presence 心跳 TTL）。
type RedisPresence struct {
	cli    *redis.Client
	prefix string // na:{env}:realtime:presence:
	ttl    time.Duration
}

// NewRedisPresence 创建（ttl=空闲超时+宽限——断连后在线状态自然过期）。
func NewRedisPresence(cli *redis.Client, namespaceRoot string, ttl time.Duration) *RedisPresence {
	if ttl <= 0 {
		ttl = 2 * time.Minute
	}
	return &RedisPresence{cli: cli, prefix: namespaceRoot + ":" + keys.PresenceNS + ":", ttl: ttl}
}

func (p *RedisPresence) key(surface, principalID string) string {
	return p.prefix + surface + ":" + principalID
}

// Touch 实现 session.Presence（SADD conn + TTL 续期）。
func (p *RedisPresence) Touch(ctx context.Context, surface, principalID, connID string) error {
	key := p.key(surface, principalID)
	pipe := p.cli.Pipeline()
	pipe.SAdd(ctx, key, connID)
	pipe.Expire(ctx, key, p.ttl)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("realtime: presence touch: %w", err)
	}
	return nil
}

// Remove 实现 session.Presence（SREM；集合空即键自然过期）。
func (p *RedisPresence) Remove(ctx context.Context, surface, principalID, connID string) error {
	if err := p.cli.SRem(ctx, p.key(surface, principalID), connID).Err(); err != nil {
		return fmt.Errorf("realtime: presence remove: %w", err)
	}
	return nil
}
