// redis.go：Bus 依赖的最小 Redis 面（Pub/Sub + Streams）——接口保留假实现
// 注入面（同 coordination.Client 惯例）。go-redis 适配快速失败（MaxRetries=0），
// 不可用时的行为由调用方决策（Publish 返回错误 / 本地回退）。
package bus

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// Redis 键片段（组合在 bus.keySet：na:{env}:bus:ch|q:...，namespaces.md §4
// Pub/Sub 行 na:{env}:{module}:ch:{name} 与队列/Stream 行 na:{env}:{module}:q:{name}，
// 消费组 <module>:<name>:cg）。
const (
	segBus           = "bus"
	segCh            = "ch"
	segQ             = "q"
	segInbox         = "inbox"
	groupName        = "default"
	groupSuf         = "cg"
	streamField      = "m"         // 流条目信封字段
	busyGroupMark    = "BUSYGROUP" // 组已存在的可幂等忽略错误标记
	pendingScanLimit = 100         // 单轮 PEL 回收扫描条数上限
	relayChanBuf     = 64          // Broadcast relay 通道缓冲（与丢弃计数策略配合）
)

// redisFace 最小 Redis 面（Pub/Sub + Streams + Inbox 的 SetNX）。
type redisFace interface {
	Ping(ctx context.Context) error
	Publish(ctx context.Context, channel string, payload []byte) error
	Subscribe(ctx context.Context, channel string) (pubsubFace, error)
	SetNX(ctx context.Context, key string, ttl time.Duration) (bool, error)
	XGroupCreateMkStream(ctx context.Context, stream, group string) error
	XAdd(ctx context.Context, stream string, maxLen int64, fields map[string]any) (string, error)
	XReadGroup(ctx context.Context, group, consumer string, streams []string) ([]streamEntry, error)
	XAck(ctx context.Context, stream, group string, ids ...string) error
	XPending(ctx context.Context, stream, group string, minIdle time.Duration, count int64) ([]pendingEntry, error)
	XClaim(ctx context.Context, stream, group, consumer string, minIdle time.Duration, ids []string) ([]streamEntry, error)
	XRange(ctx context.Context, stream, start, end string, count int64) ([]streamEntry, error)
	Close() error
}

// pubsubFace 订阅连接面。Channel 返回的 payload 通道随 Close() 关闭
// （go-redis Channel 内部 goroutine 由 PubSub.Close 触发退出并关闭源通道）。
// 不用 ReceiveMessage：其阻塞读无读期限，ctx 取消不中断（Close 死锁隐患）。
type pubsubFace interface {
	Channel() <-chan string
	Close() error
}

// streamEntry 流条目（XREADGROUP/XCLAIM 归一形状）。
type streamEntry struct {
	Stream string
	ID     string
	Fields map[string]string
}

// pendingEntry PEL 条目（回收判定用）。
type pendingEntry struct {
	ID       string
	Consumer string
	Idle     time.Duration
	Attempts int64 // 累计投递次数（首投=1）
}

// NewRedisFace 创建适配 redisFace 的 go-redis 客户端。
// MaxRetries=0：快速失败，与 coordination 适配器同口径；读超时 1s（消费循环
// 用非阻塞 XREADGROUP 轮询，不依赖阻塞命令的读超时放宽）。
func NewRedisFace(addr string) redisFace {
	return redisFaceImpl{cli: redis.NewClient(&redis.Options{
		Addr:         addr,
		DialTimeout:  1 * time.Second,
		ReadTimeout:  1 * time.Second,
		WriteTimeout: 1 * time.Second,
		MaxRetries:   0,
	})}
}

// keySet 运行期键组合（root+env 来自配置；铁律 6 禁止省略环境段）。
type keySet struct {
	broadcastChannel string // <root>:<env>:bus:ch:broadcast
	streamPrefix     string // <root>:<env>:bus:q:
	inboxPrefix      string // <root>:<env>:bus:inbox:
	consumerGroup    string // bus:default:cg
}

func newKeySet(root, env string) keySet {
	return keySet{
		broadcastChannel: fmt.Sprintf("%s:%s:%s:%s:broadcast", root, env, segBus, segCh),
		streamPrefix:     fmt.Sprintf("%s:%s:%s:%s:", root, env, segBus, segQ),
		inboxPrefix:      fmt.Sprintf("%s:%s:%s:%s:", root, env, segBus, segInbox),
		consumerGroup:    fmt.Sprintf("%s:%s:%s", segBus, groupName, groupSuf),
	}
}

// streamKey topic → Cluster 流键。
func (k keySet) streamKey(topic string) string { return k.streamPrefix + topic }

// inboxKey 业务键 → Inbox 键（Inbox：去重窗口存 Redis，TTL 由调用方给）。
func (k keySet) inboxKey(key string) string { return k.inboxPrefix + key }

// streamTopic 流键 → topic（前缀剥离；非本前缀流返回原键用于日志）。
func (k keySet) streamTopic(stream string) string {
	if len(stream) > len(k.streamPrefix) && stream[:len(k.streamPrefix)] == k.streamPrefix {
		return stream[len(k.streamPrefix):]
	}
	return stream
}

type redisFaceImpl struct {
	cli *redis.Client
}

func (f redisFaceImpl) Ping(ctx context.Context) error {
	return f.cli.Ping(ctx).Err()
}

func (f redisFaceImpl) Publish(ctx context.Context, channel string, payload []byte) error {
	return f.cli.Publish(ctx, channel, payload).Err()
}

func (f redisFaceImpl) Subscribe(ctx context.Context, channel string) (pubsubFace, error) {
	ps := f.cli.Subscribe(ctx, channel)
	// 等订阅确认，避免 relay 消息在确认前到达被丢弃
	if _, err := ps.Receive(ctx); err != nil {
		_ = ps.Close()
		return nil, fmt.Errorf("subscribe %s: %w", channel, err)
	}
	return pubsubImpl{ps: ps}, nil
}

func (f redisFaceImpl) SetNX(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	return f.cli.SetNX(ctx, key, 1, ttl).Result()
}

func (f redisFaceImpl) XGroupCreateMkStream(ctx context.Context, stream, group string) error {
	err := f.cli.XGroupCreateMkStream(ctx, stream, group, "0").Err()
	if err != nil && strings.Contains(err.Error(), busyGroupMark) {
		return nil // 幂等：组已存在
	}
	return err
}

func (f redisFaceImpl) XAdd(ctx context.Context, stream string, maxLen int64, fields map[string]any) (string, error) {
	return f.cli.XAdd(ctx, &redis.XAddArgs{
		Stream: stream,
		MaxLen: maxLen,
		Approx: true, // 近似裁剪（节点间预算）
		Values: fields,
	}).Result()
}

func (f redisFaceImpl) XReadGroup(ctx context.Context, group, consumer string, streams []string) ([]streamEntry, error) {
	// Streams 形状 = [stream1..streamN, id1..idN]——Redis 要求每 stream 各配一个
	// ID（审核修：此前 N 流只补 1 个 ">" = Unbalanced 'xreadgroup' 错误，
	// cluster 多订阅消费全失效）；">" 取未投递新消息。
	// Block=-1：go-redis 对 Block>=0 拼 BLOCK 参数（0=永久阻塞会被读超时打断），
	// 负值跳过该参数 → 真非阻塞，由调用方轮询。
	args := &redis.XReadGroupArgs{
		Group:    group,
		Consumer: consumer,
		Block:    -1,
		Streams:  append(append([]string{}, streams...), repeatID(">", len(streams))...),
	}
	res, err := f.cli.XReadGroup(ctx, args).Result()
	if err == redis.Nil {
		return nil, nil // 无新消息
	}
	if err != nil {
		return nil, err
	}
	entries := make([]streamEntry, 0)
	for _, s := range res {
		for _, m := range s.Messages {
			entries = append(entries, streamEntry{Stream: s.Stream, ID: m.ID, Fields: toStringMap(m.Values)})
		}
	}
	return entries, nil
}

func (f redisFaceImpl) XAck(ctx context.Context, stream, group string, ids ...string) error {
	return f.cli.XAck(ctx, stream, group, ids...).Err()
}

func (f redisFaceImpl) XPending(ctx context.Context, stream, group string, minIdle time.Duration, count int64) ([]pendingEntry, error) {
	res, err := f.cli.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: stream,
		Group:  group,
		Idle:   minIdle,
		Start:  "-",
		End:    "+",
		Count:  count,
	}).Result()
	if err != nil {
		return nil, err
	}
	out := make([]pendingEntry, len(res))
	for i, p := range res {
		out[i] = pendingEntry{ID: p.ID, Consumer: p.Consumer, Idle: p.Idle, Attempts: p.RetryCount}
	}
	return out, nil
}

func (f redisFaceImpl) XClaim(ctx context.Context, stream, group, consumer string, minIdle time.Duration, ids []string) ([]streamEntry, error) {
	msgs, err := f.cli.XClaim(ctx, &redis.XClaimArgs{
		Stream:   stream,
		Group:    group,
		Consumer: consumer,
		MinIdle:  minIdle,
		Messages: ids,
	}).Result()
	if err != nil {
		return nil, err
	}
	entries := make([]streamEntry, len(msgs))
	for i, m := range msgs {
		entries[i] = streamEntry{Stream: stream, ID: m.ID, Fields: toStringMap(m.Values)}
	}
	return entries, nil
}

func (f redisFaceImpl) XRange(ctx context.Context, stream, start, end string, count int64) ([]streamEntry, error) {
	msgs, err := f.cli.XRangeN(ctx, stream, start, end, count).Result()
	if err == redis.Nil {
		return nil, nil // 区间无条目
	}
	if err != nil {
		return nil, err
	}
	entries := make([]streamEntry, len(msgs))
	for i, m := range msgs {
		entries[i] = streamEntry{Stream: stream, ID: m.ID, Fields: toStringMap(m.Values)}
	}
	return entries, nil
}

func (f redisFaceImpl) Close() error {
	return f.cli.Close()
}

type pubsubImpl struct {
	ps *redis.PubSub
}

// Channel 转发 go-redis Channel 的 payload（内部 goroutine 随 PubSub.Close 退出
// 并关闭源通道 → out 随之关闭；重连/健康检查由 go-redis Channel 内建）。
// 消费慢导致缓冲满时非阻塞丢弃（Broadcast at-most-once 语义；避免转发
// goroutine 永久阻塞泄漏——审计 B2）。
func (p pubsubImpl) Channel() <-chan string {
	out := make(chan string, relayChanBuf)
	go func() {
		defer close(out)
		for m := range p.ps.Channel(redis.WithChannelSize(relayChanBuf)) {
			select {
			case out <- m.Payload:
			default:
				// 消费侧（run 循环）正常远快于 pubsub；满=run 循环卡死（异常路径）
				// 丢弃优于泄漏，消息丢失属 broadcast 既有语义
			}
		}
	}()
	return out
}

func (p pubsubImpl) Close() error {
	return p.ps.Close()
}

// toStringMap XADD 值（string）→ 归一 map（信封只用 string 字段）。
func toStringMap(values map[string]any) map[string]string {
	out := make(map[string]string, len(values))
	for k, v := range values {
		if s, ok := v.(string); ok {
			out[k] = s
		}
	}
	return out
}

// repeatID 生成 n 个相同 stream ID（XREADGROUP 多流语法面：每 stream 各配一 ID）。
func repeatID(id string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = id
	}
	return out
}
