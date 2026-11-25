// cache.go：Cache 平面——L1（BigCache，常开）/ L2（Redis，可选）/ L3（DB 真相源）。
// class 准入（hot_local 白名单）、generation 失效、负缓存、TTL jitter、自动降级统计。
// 规范：docs/rules/state-and-cache.md；策略值镜像 contracts/state-policies.yaml cache 段。
package cache

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/allegro/bigcache/v3"
	"github.com/netyilei/netyadmin/internal/kernel/observability"
)

// —— 类别与一致性（AGENTS §5；contracts/state-policies.yaml cache 段镜像） ——

// Class 是缓存层级声明（只声明 class，禁止手工选层）。
type Class string

const (
	ClassHotLocal Class = "hot_local" // L1+L2+L3（白名单 + 门槛）
	ClassShared   Class = "shared"    // L2+L3（默认）
	ClassDurable  Class = "durable"   // L3（不经过本平面）
)

// Consistency 是一致性模式。
type Consistency string

const (
	ConsistencyEvent   Consistency = "event"   // L1+L2，generation 失效，错时 bypass DB
	ConsistencyStrong  Consistency = "strong"  // L2 only，exact 失效，错时 fail
	ConsistencyBounded Consistency = "bounded" // TTL only
	ConsistencyBypass  Consistency = "bypass"  // 不过缓存
)

// DataClass 是实体数据分级（contracts cache.pii；state-and-cache §5.7）。
// classification=pii 的实体写 L2 前加密（encrypt_l2: true），L1 保持明文
// （encrypt_l1: false——进程内存面）。
type DataClass string

const (
	DataClassPublic       DataClass = "public"
	DataClassInternal     DataClass = "internal" // 默认
	DataClassConfidential DataClass = "confidential"
	DataClassPII          DataClass = "pii" // L2 值 AES-256-GCM 加密
)

// 默认值镜像 contracts cache.defaults。
const (
	defaultTTL         = 10 * time.Minute
	defaultNegativeTTL = 30 * time.Second
	defaultMaxValueKB  = 512
	defaultJitter      = 0.1
	hotLocalMaxEntry   = 64 * 1024 // admission.max_value_size
	l1TotalMB          = 128       // l1.max_mb
)

// hotLocalWhitelist 镜像 contracts cache.hot_local_whitelist（新增需评审）。
var hotLocalWhitelist = map[string]bool{
	"rbac.permissions":     true,
	"gateway.app_scope":    true,
	"config.feature_flags": true,
	"route.metadata":       true,
	"auth.public_keys":     true,
	"message.template":     true,
	"storage.config":       true,
}

// ErrNotFound 是未命中哨兵（业务侧映射状态码）。
var ErrNotFound = errors.New("cache: not found")

// L2Client 是 L2（Redis）依赖的最小面（适配见 l2_redis.go）。
type L2Client interface {
	Get(ctx context.Context, key string) ([]byte, bool, error) // bool=命中
	Set(ctx context.Context, key string, val []byte, ttl time.Duration) error
	Del(ctx context.Context, keys ...string) error
	Incr(ctx context.Context, key string) (int64, error)
}

// —— Store ——

// Store 是 Cache 平面（L1 常开；L2 可选——multi 必开由/ 校验）。
type Store struct {
	ns       string // 键命名空间（<root>:<env>:cache——铁律 6 环境段；仅 L2/Redis 键使用，L1 进程内无需）
	l1       *bigcache.BigCache
	l2       L2Client // 可为 nil（single 无 L2）
	now      func() time.Time
	metrics  *observability.Metrics // 可空（nil=命中/降级不计数；）
	inflight inflightGroup

	l2EncryptKey []byte       // PII 实体 L2 值 AES-256-GCM 密钥（WithL2EncryptKey 注入；nil=pii 注册拒绝）
	demote       demotePolicy // auto_demote 门槛（contracts cache.admission.auto_demote）

	mu      sync.RWMutex
	entries map[string]*entityCore
	genSnap map[string]genSnapshot
}

type entityCore struct {
	name       string
	class      atomic.Pointer[Class] // 有效 class（hot_local 可降级为 shared）
	generation atomic.Int64
	hits       atomic.Int64
	misses     atomic.Int64
	qps        qpsWindow // 滚动请求计数（auto_demote.min_qps 窗口化观测面）
}

// EntityConfig 是注册缓存实体的声明。
type EntityConfig[T any] struct {
	Name           string                                                 // 领域名（hot_local 必须在白名单）
	Class          Class                                                  // hot_local | shared | durable
	Consistency    Consistency                                            // event | strong | bounded | bypass
	Classification DataClass                                              // public | internal | confidential | pii（空=internal）
	TTL            time.Duration                                          // 0=默认 10m
	NegativeTTL    time.Duration                                          // 0=默认 30s
	MaxValueSize   int                                                    // 字节（0=默认 512KB）
	Jitter         float64                                                // TTL 抖动（0=默认 ±10%）
	Loader         func(ctx context.Context, key string) (T, bool, error) // L3 加载（bool=存在）
}

// Entity 是类型化缓存句柄（Register 返回；GetOrLoad 的类型安全入口）。
type Entity[T any] struct {
	store          *Store
	name           string
	class          Class
	consistency    Consistency
	classification DataClass
	ttl            time.Duration
	negativeTTL    time.Duration
	maxSize        int
	jitter         float64
	loader         func(ctx context.Context, key string) (T, bool, error)
}

// New 创建 Cache 平面（ns 键命名空间=<root>:<env>:cache——铁律 6 环境段，
// namespaces.yaml kernel_plane_keys 登记；L2 键唯一构造点在本 Store（session/
// coordination 同惯例：适配器不拼键）。ns 由装配注入（cmd/netyadmin
// cacheNamespace），禁硬编码 root/env；空 ns=legacy 兼容（旧格式键 cache:<name>:<key>，
// coordination.New 同惯例——dev 单环境，生产必须注入）。l2 传 nil 表示无 Redis——
// single 拓扑合法（contracts cache.l2.single_optional：L1+loader 直读 DB）；multi 必开
// 由 db-connect 段 F4 fail-fast 兜底；metrics 为命中/降级指标接线，可空。
// 键格式样例：na:dev:cache:<name>:<key> / na:dev:cache:gen:<name>。
func New(ns string, l2 L2Client, now func() time.Time, metrics *observability.Metrics, opts ...Option) (*Store, error) {
	if ns == "" {
		ns = "cache" // legacy 兼容（dev 单环境；生产必须注入——coordination.New 同惯例）
	}
	if now == nil {
		now = time.Now
	}
	o := options{}
	for _, opt := range opts {
		opt(&o)
	}
	if len(o.l2EncryptKey) != 0 && len(o.l2EncryptKey) != l2EncryptKeySize {
		return nil, fmt.Errorf("cache: L2 encrypt key must be %d bytes (got %d)", l2EncryptKeySize, len(o.l2EncryptKey))
	}
	// L1 过期由信封 Exp 字段管理（generation 同源），BigCache 内部过期关闭
	l1, err := bigcache.New(context.Background(), bigcache.Config{
		Shards:             64,
		LifeWindow:         0, // 条目永不过期（读取时校验信封 Exp）
		CleanWindow:        0,
		MaxEntriesInWindow: 1000 * 60 * 10,
		MaxEntrySize:       512,
		HardMaxCacheSize:   l1TotalMB, // l1.max_mb 总预算
		Verbose:            false,
	})
	if err != nil {
		return nil, fmt.Errorf("cache: init L1: %w", err)
	}
	return &Store{
		ns: ns, l1: l1, l2: l2, now: now, metrics: metrics, inflight: newInflight(),
		l2EncryptKey: o.l2EncryptKey, demote: defaultDemotePolicy(),
		entries: map[string]*entityCore{}, genSnap: map[string]genSnapshot{},
	}, nil
}

// Option 是 Store 构造可选项（只增不改——G1-15：唯一消费方=装配注入密钥）。
type Option func(*options)

type options struct {
	l2EncryptKey []byte
}

// WithL2EncryptKey 注入 PII 实体 L2 值加密钥（AES-256-GCM，必须 32 字节；
// 装配从 env-only NETYADMIN_CACHE_L2_KEY 解码注入——契约 cache.pii.encrypt_l2）。
// 未注入时 classification=pii 实体注册失败（fail-closed）。
func WithL2EncryptKey(key []byte) Option {
	return func(o *options) { o.l2EncryptKey = key }
}

// observeHit/observeMiss 同步 prometheus 命中/未命中计数（
// metrics 可空——与 entityCore 原子计数并行，仅暴露面）。
func (s *Store) observeHit(name string) {
	if s.metrics != nil {
		s.metrics.CacheHits.WithLabelValues(name).Inc()
	}
}

func (s *Store) observeMiss(name string) {
	if s.metrics != nil {
		s.metrics.CacheMisses.WithLabelValues(name).Inc()
	}
}

// Close 关闭 L1（优雅关闭， 接入）。
func (s *Store) Close() error { return s.l1.Close() }

// —— 注册与准入 ——

// Register 注册缓存实体并校验准入，返回类型化句柄。
// 规则（AGENTS §5 + contracts cache.classes/admission）：
//   - 默认 shared；hot_local 必须在白名单且值 ≤64KB；
//   - durable 不经过本平面（直接 L3/DB）；
//   - hot_local 且无 L2（single）保留 L1-only 面。
func Register[T any](s *Store, ec EntityConfig[T]) (*Entity[T], error) {
	if err := sanitizeName(ec.Name); err != nil {
		return nil, err
	}
	if ec.Class == "" {
		ec.Class = ClassShared // admission.default_class
	}
	switch ec.Class {
	case ClassHotLocal, ClassShared, ClassDurable:
	default:
		return nil, fmt.Errorf("cache: %s invalid class %q", ec.Name, ec.Class)
	}
	switch ec.Consistency {
	case "", ConsistencyEvent, ConsistencyStrong, ConsistencyBounded, ConsistencyBypass:
	default:
		return nil, fmt.Errorf("cache: %s invalid consistency %q", ec.Name, ec.Consistency)
	}
	switch ec.Classification {
	case "":
		ec.Classification = DataClassInternal // 默认 internal（contracts cache.pii 只对 pii 施加加密）
	case DataClassPublic, DataClassInternal, DataClassConfidential, DataClassPII:
	default:
		return nil, fmt.Errorf("cache: %s invalid classification %q", ec.Name, ec.Classification)
	}
	if ec.Class == ClassDurable {
		return nil, fmt.Errorf("cache: %s durable class bypasses cache plane (L3 only, §5)", ec.Name)
	}
	if ec.Class == ClassHotLocal && !hotLocalWhitelist[ec.Name] {
		return nil, fmt.Errorf("cache: %s not in hot_local whitelist (contracts, review required)", ec.Name)
	}
	if ec.Class == ClassHotLocal && ec.Classification == DataClassPII {
		// admission.allow_pii=false：PII 永不进 L1（state-and-cache §5.1.1）
		return nil, fmt.Errorf("cache: %s hot_local cannot carry pii classification (admission.allow_pii=false)", ec.Name)
	}
	if ec.Classification == DataClassPII && len(s.l2EncryptKey) == 0 {
		// cache.pii.encrypt_l2 fail-closed：密钥缺失时 pii 实体拒绝注册（禁明文落共享 Redis）
		return nil, fmt.Errorf("cache: %s classification=pii requires L2 encrypt key (NETYADMIN_CACHE_L2_KEY)", ec.Name)
	}
	if ec.Loader == nil {
		return nil, fmt.Errorf("cache: %s missing loader", ec.Name)
	}
	if ec.TTL <= 0 {
		ec.TTL = defaultTTL
	}
	if ec.NegativeTTL <= 0 {
		ec.NegativeTTL = defaultNegativeTTL
	}
	// hot_local 值上限（admission.max_value_size 64KB）先于通用默认：
	// 未声明=钳到上限；显式超限=拒绝
	if ec.Class == ClassHotLocal {
		if ec.MaxValueSize == 0 {
			ec.MaxValueSize = hotLocalMaxEntry
		} else if ec.MaxValueSize > hotLocalMaxEntry {
			return nil, fmt.Errorf("cache: %s hot_local max value %dKB exceeds admission cap 64KB", ec.Name, ec.MaxValueSize/1024)
		}
	}
	if ec.MaxValueSize <= 0 {
		ec.MaxValueSize = defaultMaxValueKB * 1024
	}
	if ec.Jitter <= 0 {
		ec.Jitter = defaultJitter
	}

	core := &entityCore{name: ec.Name}
	core.class.Store(&ec.Class)
	s.mu.Lock()
	if _, ok := s.entries[ec.Name]; ok {
		s.mu.Unlock()
		return nil, fmt.Errorf("cache: entity %q already registered", ec.Name)
	}
	s.entries[ec.Name] = core
	s.mu.Unlock()

	return &Entity[T]{
		store: s, name: ec.Name, class: ec.Class, consistency: ec.Consistency,
		classification: ec.Classification,
		ttl:            ec.TTL, negativeTTL: ec.NegativeTTL, maxSize: ec.MaxValueSize,
		jitter: ec.Jitter, loader: ec.Loader,
	}, nil
}

// —— 失效与统计 ——

// Invalidate 使实体失效（generation 自增：有 L2 → Redis INCR 广播语义，
// 各节点下次读 gen 不匹配自动 miss；无 L2 → 本地自增）。
// strong 一致性的精确删除配合 DelKeys。
// genSnapshot 是 generation 的本地快照（收敛窗口内免 RTT）。
type genSnapshot struct {
	val int64
	at  time.Time
}

const genSnapshotTTL = time.Second // 跨节点失效收敛上限（登记：窗口内可能脏读，TTL 兜底）

func (s *Store) Invalidate(ctx context.Context, name string) error {
	s.mu.RLock()
	core, ok := s.entries[name]
	s.mu.RUnlock()
	if !ok {
		return fmt.Errorf("cache: entity %q not registered", name)
	}
	if s.l2 != nil {
		newGen, err := s.l2.Incr(ctx, s.genKey(name))
		if err != nil {
			// generation 自增失败：event/bounded 按 bypass 继续（TTL 兜底）；strong 应 fail
			return fmt.Errorf("cache: invalidate %s: %w", name, err)
		}
		if s.metrics != nil {
			s.metrics.CacheInvalidate.WithLabelValues(name).Inc()
		}
		core.generation.Store(newGen)
		delete(s.genSnap, name) // 本地快照失效，下次读拉新
		return nil
	}
	core.generation.Add(1)
	return nil
}

// genFor 取实体当前 generation（跨节点单源=L2；本地 1s 快照收敛窗口）。
// 窗口内其他节点 Invalidate 的变更最多延迟 1s 生效——TTL 兜底脏读上限。
func (s *Store) genFor(ctx context.Context, core *entityCore) int64 {
	name := core.name
	if s.l2 == nil {
		return core.generation.Load()
	}
	s.mu.RLock()
	snap, ok := s.genSnap[name]
	s.mu.RUnlock()
	if ok && s.now().Sub(snap.at) < genSnapshotTTL {
		return snap.val
	}
	val, hit, err := s.l2.Get(ctx, s.genKey(name))
	if err == nil && hit {
		var v int64
		if _, err := fmt.Sscan(string(val), &v); err == nil {
			// 拉到与本地不一致的 gen=其他节点失效广播已收敛到本节点（OBS 指标）
			if v != core.generation.Load() && s.metrics != nil {
				s.metrics.CacheGenDrift.WithLabelValues(name).Inc()
			}
			core.generation.Store(v)
			s.mu.Lock()
			s.genSnap[name] = genSnapshot{val: v, at: s.now()}
			s.mu.Unlock()
			return v
		}
	}
	// L2 无 gen 键（未失效过）或读失败：本地值兜底
	return core.generation.Load()
}

// DelKeys 删除 L2 指定键（strong exact 失效用）。
func (s *Store) DelKeys(ctx context.Context, name string, keys []string) error {
	if s.l2 == nil {
		return nil
	}
	full := make([]string, 0, len(keys))
	for _, k := range keys {
		full = append(full, s.l2Key(name, k))
	}
	return s.l2.Del(ctx, full...)
}

// Stats 返回实体命中统计（诊断/降级依据， 指标消费）。
func (s *Store) Stats(name string) (hits, misses int64, ok bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	core, ok := s.entries[name]
	if !ok {
		return 0, 0, false
	}
	return core.hits.Load(), core.misses.Load(), true
}

// EffectiveClass 当前有效 class（自动降级后与声明不同）。
func (s *Store) EffectiveClass(name string) (Class, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	core, ok := s.entries[name]
	if !ok {
		return "", false
	}
	return *core.class.Load(), true
}

// demoteEntity 将 hot_local 降级为 shared（contracts admission.auto_demote.action）。
func (s *Store) demoteEntity(name string) {
	shared := ClassShared
	s.mu.RLock()
	core, ok := s.entries[name]
	s.mu.RUnlock()
	if ok && core.class.Load() != nil && *core.class.Load() == ClassHotLocal {
		core.class.Store(&shared)
		if s.metrics != nil {
			s.metrics.CacheDemoted.WithLabelValues(name).Inc()
		}
	}
}

// —— 键与工具 ——

// l1Key 进程内 L1 键（不跨节点/环境——无需 namespace 段）。
func l1Key(name, key string) string { return name + ":" + key }

// l2Key L2（Redis）值键：ns:<name>:<key>——ns=na:{env}:cache 装配注入，
// 完整形如 na:dev:cache:rbac.permissions:u1（namespaces.yaml kernel_plane_keys）。
func (s *Store) l2Key(name, key string) string { return s.ns + ":" + name + ":" + key }

// genKey L2 generation 计数键：ns:gen:<name>——完整形如 na:dev:cache:gen:<name>。
func (s *Store) genKey(name string) string { return s.ns + ":gen:" + name }

func sanitizeName(name string) error {
	if name == "" || strings.ContainsAny(name, " \n\r\t") || strings.Contains(name, "..") {
		return fmt.Errorf("cache: entity name %q 非法（防键注入）", name)
	}
	return nil
}

// jitterTTL 给 TTL 施加 ±jitter 抖动（防同时过期）。
var jitterCounter atomic.Uint64

// jitterTTL 给 TTL 施加 ±jitter 抖动（计数器混合纳秒——同 tick 两次调用不同，防同时过期）。
func jitterTTL(ttl time.Duration, jitter float64, now func() time.Time) time.Duration {
	if jitter <= 0 {
		return ttl
	}
	n := now().UnixNano() + int64(jitterCounter.Add(1))
	factor := 1.0 + (float64(n%1000)/1000.0*2-1)*jitter
	out := time.Duration(float64(ttl) * factor)
	if out <= 0 {
		return ttl
	}
	return out
}
