// session.go：Session 平面——token 吊销、登录会话存储、刷新轮换。
// 策略唯一源 contracts/state-policies.yaml session 段（Go 常量镜像）：
//
//	token_verification = local_jwt（本地 RS256 校验，不经本包—— AuthN 组合原语）；
//	token_revocation / session_store：Redis + DB 回退（single: db_fallback_or_fail，
//	multi: fail_closed）；refresh_rotation：fail_closed（双拓扑）。
package session

import (
	"context"
	"errors"
	"time"

	"github.com/netyilei/netyadmin/internal/kernel/observability"
)

// OnUnavailable 是 Redis 不可用时的行为声明（镜像 contracts session 段）。
type OnUnavailable string

const (
	Allow            OnUnavailable = "allow"               // token_verification：本地校验天然可用
	DBFallbackOrFail OnUnavailable = "db_fallback_or_fail" // DB 回调注入则回退，否则 fail
	FailClosed       OnUnavailable = "fail_closed"         // 拒绝操作
)

// Topology 是部署拓扑。
type Topology string

const (
	TopologySingle Topology = "single"
	TopologyMulti  Topology = "multi"
)

// Policy 是单个面的降级声明。
type Policy struct {
	OnSingle      OnUnavailable
	OnMulti       OnUnavailable
	HasDBFallback bool // revocation/session_store 的 read_fallback: db（回调注入后生效）
}

// 镜像 contracts session 段的策略表。
var policies = map[string]Policy{
	"revocation":       {OnSingle: DBFallbackOrFail, OnMulti: FailClosed, HasDBFallback: true},
	"refresh_rotation": {OnSingle: FailClosed, OnMulti: FailClosed},
	"session_store":    {OnSingle: DBFallbackOrFail, OnMulti: FailClosed, HasDBFallback: true},
}

// PolicyFor 按面名取策略（未知名 fail-closed）。
func PolicyFor(face string) Policy {
	if p, ok := policies[face]; ok {
		return p
	}
	return Policy{OnSingle: FailClosed, OnMulti: FailClosed}
}

// ErrUnavailable 是 Redis 不可用且无 DB 回退（fail-closed）的错误。
var ErrUnavailable = errors.New("session: redis unavailable and no db fallback (fail-closed)")

// ErrDenied 是占用失败/已失效（非系统错误）。
var ErrDenied = errors.New("session: denied")

// —— DB 回退回调（ID-01 接表；nil=无回退） ——

// RevocationFallback 从 DB 查询 token 是否已吊销（read_fallback: db）。
type RevocationFallback func(ctx context.Context, jti string) (revoked bool, err error)

// SessionFallback 从 DB 读取会话负载（read_fallback: db）。
type SessionFallback func(ctx context.Context, sessionID string) ([]byte, bool, error)

// Client 是依赖的最小 Redis 面（适配见 redis_client.go）。
type Client interface {
	SetNX(ctx context.Context, key string, value any, ttl time.Duration) (bool, error)
	Exists(ctx context.Context, key string) (bool, error)
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, val []byte, ttl time.Duration) error
	Del(ctx context.Context, keys ...string) error
	Eval(ctx context.Context, script string, keys []string, args ...any) (any, error)
	Ping(ctx context.Context) error
}

// Store 是 Session 平面入口（构造后不可变；拓扑运行期不重解释）。
type Store struct {
	client    Client
	topo      Topology
	ns        string // 键命名空间（<root>:<env>:sess——铁律 6 环境段，审计 B 组）
	revDB     RevocationFallback
	sessionDB SessionFallback
	metrics   *observability.Metrics // 可空（nil=降级路径不计数；）

	// 键前缀（New 时按 ns 展开一次；faces.go 直接拼 key）
	prefixRevoked string
	prefixStore   string
	prefixRefresh string
}

// New 创建（ns 键命名空间=<root>:<env>:sess——铁律 6 环境段，
// namespaces.yaml kernel_plane_keys 登记；DB 回退回调由注入—— ID-01
// 接表后传入，nil=无回退；metrics 为降级指标接线，可空）。
// 键含环境段后旧格式键（sess:*）不再读取——旧会话/刷新令牌一次性失效
// （本地开发可接受；多机滚动发布时需排空在线会话）。
func New(client Client, topo Topology, ns string, revDB RevocationFallback, sessionDB SessionFallback, metrics *observability.Metrics) *Store {
	if ns == "" {
		ns = "sess" // legacy 兼容（dev 单环境；生产必须注入——coordination.New 同惯例）
	}
	return &Store{
		client: client, topo: topo, ns: ns,
		prefixRevoked: ns + ":revoked:",
		prefixStore:   ns + ":store:",
		prefixRefresh: ns + ":refresh:",
		revDB:         revDB, sessionDB: sessionDB, metrics: metrics,
	}
}

// dbFallbackAllowed 判断某面在当前拓扑下是否允许 DB 回退
// （contracts：revocation/session_store 的 single=db_fallback_or_fail；multi=fail_closed）。
func (s *Store) dbFallbackAllowed(face string) bool {
	p := PolicyFor(face)
	if s.topo == TopologySingle {
		return p.OnSingle == DBFallbackOrFail && p.HasDBFallback
	}
	return false // multi fail_closed：DB 回调不改变声明
}
