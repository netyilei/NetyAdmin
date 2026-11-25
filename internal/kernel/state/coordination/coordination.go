package coordination

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/netyilei/netyadmin/internal/kernel/database"
	"github.com/netyilei/netyadmin/internal/kernel/observability"
)

// —— 策略模型（唯一源：contracts/state-policies.yaml coordination 段） ——
//
// Go 常量是该文件的镜像（运行时行为依据）；contracts↔代码一致性由 CI 对偶校验。
// 每个原语构造时显式接收 Policy——代码不得隐式回退（state-policies.yaml 头注）。

// OnUnavailable 是 Redis 不可用时的行为声明（contracts/state-policies.yaml）。
type OnUnavailable string

const (
	FailClosed OnUnavailable = "fail_closed" // 拒绝操作（正确性优先）
	FailOpen   OnUnavailable = "fail_open"   // 放行（可用性优先，如登录/API 限流）
	Local      OnUnavailable = "local"       // 单机本地回退（仅 topology=single 合法）
	LocalOrDB  OnUnavailable = "local_or_db" // 单机本地或 DB 回退
	// PGAdvisoryOrFail multi 下转 PG advisory lock（实现分支），
	// 当前分支未接入 → fail（见 defaultPolicies leader 登记）。
	PGAdvisoryOrFail OnUnavailable = "pg_advisory_lock_or_fail"
)

// Topology 是部署拓扑（AGENTS §3：本地回退仅 single 允许）。
type Topology string

const (
	TopologySingle Topology = "single"
	TopologyMulti  Topology = "multi"
)

// Policy 是单个原语/限流策略的降级声明。
type Policy struct {
	OnSingle OnUnavailable // topology=single 且 Redis 不可用
	OnMulti  OnUnavailable // topology=multi 且 Redis 不可用
}

// namedRateLimitPolicies 镜像 contracts/state-policies.yaml rate_limit.policies
// （登录/API/管理登录 fail-open；网关配额 fail-closed）。
var namedRateLimitPolicies = map[string]Policy{
	"login_ip":    {OnSingle: FailOpen, OnMulti: FailOpen},
	"api_rule":    {OnSingle: FailOpen, OnMulti: FailOpen},
	"admin_login": {OnSingle: FailOpen, OnMulti: FailOpen},
	"gateway_app": {OnSingle: FailClosed, OnMulti: FailClosed},
	"users_login": {OnSingle: FailOpen, OnMulti: FailOpen},
}

// RateLimitPolicy 按名取限流策略（未知名 fail-closed——未声明即拒绝）。
func RateLimitPolicy(name string) Policy {
	if p, ok := namedRateLimitPolicies[name]; ok {
		return p
	}
	return Policy{OnSingle: FailClosed, OnMulti: FailClosed}
}

// RateLimitPolicyRegistered 探针：名是否已登记 contracts（未知名与已登记
// fail-closed 策略取值相同，无法从取值区分—— 引擎装配校验用）。
func RateLimitPolicyRegistered(name string) bool {
	_, ok := namedRateLimitPolicies[name]
	return ok
}

// defaultPolicies 镜像 contracts coordination 段的 on_unavailable（single/multi）。
// 登记：leader 的 pg_advisory_lock_or_fail 的 PG 分支随（分布式锁/选主
// 跨服务）实现，当前按 fail 处理；idempotency 的 db 分支随幂等表接入。
var defaultPolicies = map[string]Policy{
	"lock":        {OnSingle: Local, OnMulti: FailClosed},
	"nonce":       {OnSingle: Local, OnMulti: FailClosed},
	"idempotency": {OnSingle: LocalOrDB, OnMulti: FailClosed},
	"leader":      {OnSingle: Local, OnMulti: PGAdvisoryOrFail},
	"counter":     {OnSingle: Local, OnMulti: FailClosed},
}

// PrimitivePolicy 按原语名取策略（未知名 fail-closed）。
func PrimitivePolicy(name string) Policy {
	if p, ok := defaultPolicies[name]; ok {
		return p
	}
	return Policy{OnSingle: FailClosed, OnMulti: FailClosed}
}

// decideUnavailable 决定 Redis 不可用时的行为；返回 (是否允许继续, 是否本地回退, error)。
// 拓扑合法性在此统一把守：multi 下的 Local 声明按 fail_closed 处理（AGENTS §3）。
func decideUnavailable(p Policy, topo Topology) (allow bool, local bool, err error) {
	on := p.OnMulti
	if topo == TopologySingle {
		on = p.OnSingle
	}
	switch on {
	case FailOpen:
		return true, false, nil
	case FailClosed, "":
		return false, false, ErrUnavailable
	case Local, LocalOrDB:
		if topo != TopologySingle {
			return false, false, ErrUnavailable // multi 本地回退禁止（AGENTS §3）
		}
		return true, true, nil
	default:
		return false, false, ErrUnavailable
	}
}

// —— 本地回退存储（仅 topology=single；TTL 过期语义与 Redis 对齐） ——

// localSweepEvery 是写路径摊销清扫周期（每 N 次写入清一次过期键——distinct
// 键的惰性过期会让长寿命进程内存无界累积，L48 无界面兜底）。
const localSweepEvery = 1024

type localEntry struct {
	value   any
	expires time.Time
}

type localStore struct {
	mu     sync.RWMutex
	data   map[string]*localEntry
	now    func() time.Time
	writes int // 自上次摊销清扫起的写次数
}

func newLocalStore() *localStore {
	return &localStore{data: map[string]*localEntry{}, now: time.Now}
}

func (l *localStore) setNX(key string, ttl time.Duration) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if e, ok := l.data[key]; ok && e.expires.After(l.now()) {
		return false
	}
	l.data[key] = &localEntry{expires: l.now().Add(ttl)}
	l.sweepAmortizedLocked()
	return true
}

func (l *localStore) incr(key string, ttl time.Duration) int64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.data[key]
	if !ok || e.expires.Before(l.now()) {
		// 与 Redis 路径语义对齐：仅首增设 TTL（n>1 不续期，窗口按到期重置）
		l.data[key] = &localEntry{value: int64(1), expires: l.now().Add(ttl)}
		l.sweepAmortizedLocked()
		return 1
	}
	n := e.value.(int64) + 1
	e.value = n
	return n
}

// incrSliding 递增并按滑动 TTL 续期（IncrSequence 的本地回退——每次递增刷新
// 到期时间，持续流量下不归零）。
func (l *localStore) incrSliding(key string, ttl time.Duration) int64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	n := int64(1)
	if e, ok := l.data[key]; ok && e.expires.After(l.now()) {
		n = e.value.(int64) + 1
	}
	l.data[key] = &localEntry{value: n, expires: l.now().Add(ttl)}
	l.sweepAmortizedLocked()
	return n
}

func (l *localStore) del(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.data, key)
}

func (l *localStore) setTTL(key, value string, ttl time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.data[key] = &localEntry{value: value, expires: l.now().Add(ttl)}
	l.sweepAmortizedLocked()
}

// sweepAmortizedLocked 每 localSweepEvery 次写清一次过期键（调用方必须持
// l.mu 写锁；过期键语义上已不存在——清扫只回收内存，不改任何读语义）。
func (l *localStore) sweepAmortizedLocked() {
	l.writes++
	if l.writes < localSweepEvery {
		return
	}
	l.writes = 0
	now := l.now()
	for k, e := range l.data {
		if e.expires.Before(now) {
			delete(l.data, k)
		}
	}
}

func (l *localStore) exists(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.data[key]
	return ok && e.expires.After(l.now())
}

// —— Redis 键前缀（AGENTS §6：key 前缀具名，禁止内联字面量） ——

// 前缀段（实例 ns 组装完整键 ns:<段>:<key>——namespaces 铁律 6 env 段；
// ns 由 bootstrap 注入 na:{env}:coord，审计 A12-6 根修）。
const (
	prefixLock    = "lock:"
	prefixNonce   = "nonce:"
	prefixIdem    = "idem:"
	prefixLeader  = "leader:"
	prefixCounter = "counter:"
	prefixRate    = "ratelimit:"
	prefixAlive   = "alive:"
)

// —— 依赖 ———

// Coordination 是协调平面入口：持 Redis 客户端 + 拓扑，各原语方法按策略执行。
// 构造后不可变（拓扑/策略运行期不重解释，热更边界）。
type Coordination struct {
	client  Client
	topo    Topology
	ns      string // 键命名空间（na:{env}:coord；空=legacy "coord"，仅 dev 兼容）
	local   *localStore
	metrics *observability.Metrics // 可空（nil=降级路径不计数）

	// PG advisory 分支（advisory.go——AttachPG 可选注入，nil=分支禁用）
	pgMu     sync.Mutex
	pgPool   *database.Pool
	pgHolder *pgAdvisoryCampaign
}

// New 创建协调平面（client 由 bootstrap 注入；topology 来自 config.Deployment；
// ns 键命名空间=na:{env}:coord——铁律 6 环境段，审计 A12-6；
// metrics 为降级指标接线，可空）。
func New(client Client, topo Topology, ns string, metrics *observability.Metrics) *Coordination {
	if ns == "" {
		ns = "coord" // legacy 兼容（dev 单环境；生产必须注入）
	}
	return &Coordination{client: client, topo: topo, ns: ns, local: newLocalStore(), metrics: metrics}
}

// decide 统一决策点：Redis 不可用时按策略裁决并计数（——降级静默=违规，
// reliability.md §5；coordination/session 降级指标项）。
func (c *Coordination) decide(name string, p Policy) (allow bool, local bool, err error) {
	allow, local, err = decideUnavailable(p, c.topo)
	if c.metrics != nil {
		switch {
		case err != nil:
			c.metrics.CoordFailClosed.WithLabelValues(name).Inc()
		case local:
			c.metrics.CoordFallbacks.WithLabelValues(name).Inc()
		default:
			c.metrics.CoordFailOpen.WithLabelValues(name).Inc()
		}
	}
	return allow, local, err
}

// key 组装完整键（ns:段:key）。
func (c *Coordination) key(prefix, key string) string { return c.ns + ":" + prefix + key }

// ErrUnavailable 是 Redis 不可用且策略 fail-closed 的错误（调用方转为对应状态码）。
var ErrUnavailable = errors.New("coordination: redis unavailable (fail-closed)")

// ErrDenied 是原子占用失败（锁被占/nonce 重放/幂等冲突）——非系统错误。
var ErrDenied = errors.New("coordination: denied")

// Ping 透传 Redis 探活（health 探针消费；运行期翻转 ready）。
func (c *Coordination) Ping(ctx context.Context) error { return c.client.Ping(ctx) }

//nop wrapper: 统一在原语内 try redis → err 时按策略。实现见各原语文件。
