// ticket_redis.go：一次性 WS ticket 存储（TicketStore 契约实现）——
// GETDEL 原子消费（单次语义）+ 60s TTL（验收：单次s/绑定 Principal）。
// 签发侧（edges）经 Issue 生成 ticket；Upgrader 握手经 Take 消费。
package ws

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/netyilei/netyadmin/internal/kernel/security/authn"
)

// TicketTTL 是 ticket 有效期（验收基线 60s）。
const TicketTTL = 60 * time.Second

// ticketStoreTTL 是 Redis 键保留期（逻辑 TTL 的 2 倍）：逻辑过期后键再存续
// 一个桶周期，消费时按 payload.expires_at 区分「过期（18502）」与「已用/
// 未发（18501）」。超过保留期两者不可辨——统一 18501（ws-protocol §1/§10）。
// N-1 注记：滚动窗口内旧节点无 expires_at 判定（忽略未知字段），其可见过期
// 上界临时放宽到键 TTL（2×）——单次使用/身份绑定不变，一个发布周期后消失。
const ticketStoreTTL = 2 * TicketTTL

// RedisTicketStore 实现 TicketStore（GETDEL 原子消费）。
type RedisTicketStore struct {
	cli   *redis.Client
	ns    string // 键命名空间（na:{env}:realtime:ticket——keys.TicketNS）
	clock func() time.Time
}

// NewRedisTicketStore 创建（ns 由装配注入——realtime 模块统一前缀）。
func NewRedisTicketStore(cli *redis.Client, ns string) *RedisTicketStore {
	return &RedisTicketStore{cli: cli, ns: ns, clock: time.Now}
}

func (s *RedisTicketStore) key(ticket string) string { return s.ns + ":" + ticket }

// ticketPayload 是 ticket 绑定负载（Principal 关键字段——WS 会话身份；
// Surface=签发面（跨面消费即无效——ws-protocol §1「ticket 绑定 surface」）；
// Channels=允许频道集合，空=不限定（订阅时仍过 surface ACL）；
// ExpiresAt=逻辑过期毫秒时间戳（Issue 写入——消费面 18501502 细分依据；
// 滚动升级窗口内旧节点签发值缺省=0，按键 TTL 语义视为有效，N-1 兼容）。
type ticketPayload struct {
	Type      string   `json:"type"`
	ID        string   `json:"id"`
	Surface   string   `json:"surface,omitempty"`
	Scopes    []string `json:"scopes,omitempty"`
	Channels  []string `json:"channels,omitempty"`
	ExpiresAt int64    `json:"expires_at,omitempty"`
}

// TicketGrant 是签发绑定（身份 + 签发面 + 允许频道集合）。
type TicketGrant struct {
	Principal authn.Principal
	// Surface 是签发面名（admin/gateway/client——Take 校验面匹配，跨面消费即无效）。
	Surface string
	// Channels 是允许订阅的频道清单（空=不限定——订阅时仍过 surface ACL）。
	Channels []string
}

// Issue 签发 ticket（调用方=接入面握手前——绑定 Principal；返回 32 字节随机
// base64url 凭证）。
func (s *RedisTicketStore) Issue(ctx context.Context, p authn.Principal) (string, error) {
	return s.IssueBound(ctx, TicketGrant{Principal: p})
}

// IssueBound 签发带频道绑定的 ticket（ws-protocol §1 允许频道集合；
// audit-m6 协-A3 surface 绑定面补全——绑定面=身份+频道，surface 由键命名空间
// 隔离（每 surface 独立 ns 装配））。
func (s *RedisTicketStore) IssueBound(ctx context.Context, g TicketGrant) (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("ws: ticket rand: %w", err)
	}
	ticket := base64.RawURLEncoding.EncodeToString(buf)
	raw, err := json.Marshal(ticketPayload{
		Type: string(g.Principal.Type), ID: g.Principal.ID, Surface: g.Surface,
		Scopes: g.Principal.Scopes, Channels: g.Channels,
		ExpiresAt: s.clock().Add(TicketTTL).UnixMilli(),
	})
	if err != nil {
		return "", fmt.Errorf("ws: ticket marshal: %w", err)
	}
	// SET NX EX：撞键（概率≈0）必须拒绝而非把既有 ticket 当自己的返回。
	// 键保留期=2×逻辑 TTL（过期桶供 18502 判定；见 ticketStoreTTL 注释）。
	ok, err := s.cli.SetNX(ctx, s.key(ticket), raw, ticketStoreTTL).Result()
	if err != nil {
		return "", fmt.Errorf("ws: ticket store: %w", err)
	}
	if !ok {
		return "", fmt.Errorf("ws: ticket collision (retry)")
	}
	return ticket, nil
}

// Take 实现 TicketStore：GETDEL 原子消费（单次语义）→ 反序列化绑定负载。
func (s *RedisTicketStore) Take(ctx context.Context, ticket string) (authn.Principal, bool, error) {
	p, _, _, ok, err := s.TakeBound(ctx, ticket)
	return p, ok, err
}

// TakeBound 消费 ticket 并返回频道/面绑定（session 订阅校验+面匹配用）。
// 过期细分（18502）：GETDEL 取到值后按 payload.expires_at 本地判定——按键
// 保留期 2×逻辑 TTL，过期票在桶内仍可取值；「不存在」=已用/未发/超保留期
// （统一 18501，存储层已无法区分）。
func (s *RedisTicketStore) TakeBound(ctx context.Context, ticket string) (authn.Principal, []string, string, bool, error) {
	raw, err := s.cli.GetDel(ctx, s.key(ticket)).Bytes()
	if err == redis.Nil {
		return authn.Principal{}, nil, "", false, nil // 不存在/已用/过期桶已清
	}
	if err != nil {
		return authn.Principal{}, nil, "", false, fmt.Errorf("ws: ticket take: %w", err)
	}
	var p ticketPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return authn.Principal{}, nil, "", false, fmt.Errorf("ws: ticket decode: %w", err)
	}
	if p.ExpiresAt > 0 && !s.clock().Before(time.UnixMilli(p.ExpiresAt)) {
		return authn.Principal{}, nil, "", false, ErrTicketExpired // 键已由 GETDEL 删除
	}
	return authn.Principal{Type: authn.PrincipalType(p.Type), ID: p.ID, Scopes: p.Scopes},
		p.Channels, p.Surface, true, nil
}
