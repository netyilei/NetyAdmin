// getorload.go：GetOrLoad 主链路——L1→L2→loader（singleflight）→ 回填，
// generation 校验、负缓存、命中率统计与 hot_local 自动降级。
package cache

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"golang.org/x/sync/singleflight"
)

// inflightGroup 包装 singleflight（按实体+键合并并发加载）。
type inflightGroup struct{ g singleflight.Group }

func newInflight() inflightGroup { return inflightGroup{} }

// cachedEnvelope 是 L1/L2 值的统一信封：generation 校验 + 负缓存标记 + 数据。
// Enc 标记 L2 值已加密（classification=pii；L1 恒明文——encrypt_l1: false）。
type cachedEnvelope struct {
	Gen  int64           `json:"g"`
	Neg  bool            `json:"neg,omitempty"` // 负缓存（loader 确认不存在）
	Enc  bool            `json:"enc,omitempty"` // Data 为 AES-GCM 密文（PII 实体 L2 面）
	Exp  int64           `json:"e,omitempty"`   // 过期 UnixNano（L1 过期面；L2 由 Redis TTL 管）
	Data json.RawMessage `json:"d,omitempty"`
}

// GetOrLoad 主链路（按 class/consistency 分发）：
//   - bypass：直接 loader；
//   - hot_local：L1 → L2 → loader（回填 L1+L2）；
//   - shared：L2 → loader（回填 L2）；
//   - strong：跳过 L1，L2 读/写失败=fail（on_error，contracts consistency.strong）；bounded：不校验 generation；
//   - 负缓存：loader 确认不存在时按 NegativeTTL 缓存"不存在"；
//   - 统计与自动降级：hot_local 命中率 <70% 或滚动窗口 QPS <100（满窗观测后）
//     时降为 shared（admission.auto_demote，demote.go）。
//
// 返回 (值, 是否存在, 错误)；loader 确认不存在时返回 ErrNotFound。
func (e *Entity[T]) GetOrLoad(ctx context.Context, key string) (T, bool, error) {
	var zero T
	if e.consistency == ConsistencyBypass {
		v, found, err := e.loader(ctx, key)
		if err != nil {
			return zero, false, err
		}
		if !found {
			return zero, false, ErrNotFound
		}
		return v, true, nil
	}

	core := e.store.entries[e.name]
	// 有效 class 从 core 读取（hot_local 降级后 GetOrLoad 立即走 shared 路径）
	effClass := *core.class.Load()
	core.qps.observe(e.store.now(), e.store.demote.window, e.store.demote.buckets) // auto_demote.min_qps 观测
	gen := e.store.genFor(ctx, core)

	// L1（hot_local only；strong 跳过）
	if effClass == ClassHotLocal && e.consistency != ConsistencyStrong {
		if v, ok, hit := e.l1Lookup(ctx, key, gen); hit {
			if !ok {
				// 负缓存命中：计入 miss（降级语义：全部负命中=无价值）并返回 ErrNotFound
				core.misses.Add(1)
				e.store.observeMiss(e.name)
				e.checkDemote(core)
				return zero, false, ErrNotFound
			}
			core.hits.Add(1)
			e.store.observeHit(e.name)
			return v, true, nil
		}
	}
	// L2（hot_local/shared；strong 跳过 L1 直接 L2）
	if e.store.l2 != nil && (effClass == ClassShared || effClass == ClassHotLocal) {
		v, ok, hit, err := e.l2Lookup(ctx, key, gen)
		if err != nil && e.consistency == ConsistencyStrong {
			// strong on_error=fail（contracts consistency.strong）——L2 读失败不静默
			// 回源（读少写多高敏配置的 DB 压力保护）；event/bounded 按 on_error=
			// bypass_to_db 落 loader（下方 miss 路径）
			return zero, false, err
		}
		if hit {
			if !ok {
				core.misses.Add(1)
				e.store.observeMiss(e.name)
				e.checkDemote(core)
				return zero, false, ErrNotFound
			}
			core.hits.Add(1)
			e.store.observeHit(e.name)
			if effClass == ClassHotLocal { // L2 命中回填 L1
				e.l1Store(ctx, key, gen, v, false, e.ttl)
			}
			return v, true, nil
		}
	}
	core.misses.Add(1)
	e.store.observeMiss(e.name)
	e.checkDemote(core)

	// singleflight 合并并发加载
	v, found, err := e.loadSingleflight(ctx, key, gen)
	if err != nil {
		return zero, false, err
	}
	return v, found, nil
}

// loadSingleflight 合并并发加载并回填各级缓存。
func (e *Entity[T]) loadSingleflight(ctx context.Context, key string, gen int64) (T, bool, error) {
	var zero T
	v, found, err := e.loadSingleflightInner(ctx, key, gen)
	if err != nil {
		return zero, false, err
	}
	if !found {
		return zero, false, ErrNotFound
	}
	return v, true, nil
}

func (e *Entity[T]) loadSingleflightInner(ctx context.Context, key string, gen int64) (T, bool, error) {
	var zero T
	sfKey := e.name + ":" + key
	val, err, _ := e.store.inflight.g.Do(sfKey, func() (any, error) {
		return e.loadAndFill(ctx, key, gen)
	})
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return zero, false, ErrNotFound
		}
		return zero, false, err
	}
	t, ok := val.(T)
	if !ok {
		// 负缓存结果（negResult 包装）
		if _, isNeg := val.(negResult); isNeg {
			return zero, false, ErrNotFound
		}
		return zero, false, ErrNotFound
	}
	return t, true, nil
}

// negResult 标记负缓存装载（singleflight 共享给并发等待者）。
type negResult struct{}

// loadAndFill loader 加载 + 各级回填（singleflight 单飞内执行）。
func (e *Entity[T]) loadAndFill(ctx context.Context, key string, gen int64) (any, error) {
	v, found, err := e.loader(ctx, key)
	if err != nil {
		return nil, err
	}
	if !found {
		// 负缓存：L1（hot_local）+ L2（hot_local/shared）
		neg := cachedEnvelope{Gen: gen, Neg: true, Exp: e.expAt(e.negativeTTL)}
		b, _ := json.Marshal(neg)
		ttl := jitterTTL(e.negativeTTL, e.jitter, e.store.now)
		if effClassOf(e) == ClassHotLocal {
			_ = e.store.l1.Set(l1Key(e.name, key), b)
		}
		if e.store.l2 != nil && (effClassOf(e) == ClassShared || effClassOf(e) == ClassHotLocal) {
			_ = e.store.l2.Set(ctx, e.store.l2Key(e.name, key), b, ttl)
		}
		return negResult{}, nil
	}
	// 正缓存
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	if err := checkSize(e.name, b, e.maxSize); err != nil {
		return nil, err
	}
	env := cachedEnvelope{Gen: gen, Exp: e.expAt(e.ttl), Data: b}
	envB, _ := json.Marshal(env)
	ttl := jitterTTL(e.ttl, e.jitter, e.store.now)
	if effClassOf(e) == ClassHotLocal {
		_ = e.store.l1.Set(l1Key(e.name, key), envB) // L1 明文（contracts cache.pii.encrypt_l1=false）
	}
	if e.store.l2 != nil && (effClassOf(e) == ClassShared || effClassOf(e) == ClassHotLocal) {
		l2B := envB
		if e.classification == DataClassPII {
			// cache.pii.encrypt_l2：L2 值写前 AES-256-GCM 加密（密钥在 Register 已 fail-closed 校验）。
			// 密文经 base64 编码为 JSON 字符串装入 Data（加密面信封 Enc=true——与明文 RawMessage 区分）。
			// 加密失败=不可写共享 Redis（fail-closed）：strong 报错；其余跳过回填（本次值照常返回）。
			sealed, encErr := encryptL2Value(e.store.l2EncryptKey, b)
			if encErr != nil {
				if e.consistency == ConsistencyStrong {
					return nil, encErr
				}
				return v, nil
			}
			encData, marshalErr := json.Marshal(base64.StdEncoding.EncodeToString(sealed))
			if marshalErr != nil {
				if e.consistency == ConsistencyStrong {
					return nil, marshalErr
				}
				return v, nil
			}
			l2Env := env
			l2Env.Data = encData
			l2Env.Enc = true
			l2B, _ = json.Marshal(l2Env)
		}
		if err := e.store.l2.Set(ctx, e.store.l2Key(e.name, key), l2B, ttl); err != nil {
			// L2 写失败按一致性：event/bounded 继续（L1/直读兜底）；strong fail
			if e.consistency == ConsistencyStrong {
				return nil, err
			}
		}
	}
	return v, nil
}

// effClassOf 读实体的有效 class（降级感知）。
func effClassOf[T any](e *Entity[T]) Class {
	if core, ok := e.store.entries[e.name]; ok {
		return *core.class.Load()
	}
	return e.class
}

// l1Lookup 读 L1 并校验 generation。
func (e *Entity[T]) l1Lookup(_ context.Context, key string, gen int64) (T, bool, bool) {
	var zero T
	raw, err := e.store.l1.Get(l1Key(e.name, key))
	if err != nil {
		return zero, false, false
	}
	var env cachedEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return zero, false, false // 解码失败视为 miss
	}
	if e.consistency != ConsistencyBounded && env.Gen != gen {
		return zero, false, false // generation 失效
	}
	if e.l1Expired(env) {
		return zero, false, false // L1 过期
	}
	if env.Neg {
		return zero, false, true // 负缓存命中：确认不存在
	}
	var v T
	if err := json.Unmarshal(env.Data, &v); err != nil {
		return zero, false, false
	}
	return v, true, true
}

// l2Lookup 读 L2 并校验 generation（返回值不回填 L1——由调用方决定）。
// L2 读取错误单独返回（strong on_error=fail 分档用）；解码失败/未命中=miss。
func (e *Entity[T]) l2Lookup(ctx context.Context, key string, gen int64) (T, bool, bool, error) {
	var zero T
	raw, hit, err := e.store.l2.Get(ctx, e.store.l2Key(e.name, key))
	if err != nil {
		return zero, false, false, err
	}
	if !hit {
		return zero, false, false, nil
	}
	var env cachedEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return zero, false, false, nil // 解码失败视为 miss（§5.6 脏数据）
	}
	if e.consistency != ConsistencyBounded && env.Gen != gen {
		return zero, false, false, nil // generation 失效
	}
	if env.Neg {
		return zero, false, true, nil
	}
	if env.Enc {
		// PII 实体 L2 密文（Data=base64 字符串）：密钥缺失/解密失败（密钥轮换、脏数据）一律视为 miss（§5.6）
		if len(e.store.l2EncryptKey) == 0 {
			return zero, false, false, nil
		}
		var encData string
		if err := json.Unmarshal(env.Data, &encData); err != nil {
			return zero, false, false, nil
		}
		sealed, err := base64.StdEncoding.DecodeString(encData)
		if err != nil {
			return zero, false, false, nil
		}
		plain, derr := decryptL2Value(e.store.l2EncryptKey, sealed)
		if derr != nil {
			return zero, false, false, nil
		}
		env.Data = plain
	}
	var v T
	if err := json.Unmarshal(env.Data, &v); err != nil {
		return zero, false, false, nil
	}
	return v, true, true, nil
}

// l1Store 直接写 L1（L2 命中后回填）。
func (e *Entity[T]) l1Store(_ context.Context, key string, gen int64, v T, neg bool, ttl time.Duration) {
	if neg {
		negB, _ := json.Marshal(cachedEnvelope{Gen: gen, Neg: true, Exp: e.expAt(e.negativeTTL)})
		_ = e.store.l1.Set(l1Key(e.name, key), negB)
		return
	}
	b, err := json.Marshal(v)
	if err != nil {
		return
	}
	envB, _ := json.Marshal(cachedEnvelope{Gen: gen, Exp: e.expAt(ttl), Data: b})
	_ = e.store.l1.Set(l1Key(e.name, key), envB)
	_ = ttl
}

// checkDemote 降级评估（contracts admission.auto_demote；每 missStep 次 miss 评估一次，
// 不自动升级）。两档任一命中即降为 shared：
//   - 命中率档：累计 hits/(hits+misses) < min_hit_ratio；
//   - QPS 档：滚动窗口（window）平均 QPS < min_qps，且观测已满窗（未满窗不判定，
//     防启动即降级——「持续 10 分钟」语义）。
func (e *Entity[T]) checkDemote(core *entityCore) {
	if e.class != ClassHotLocal {
		return
	}
	pol := e.store.demote
	m := core.misses.Load()
	if m == 0 || m%pol.missStep != 0 {
		return
	}
	h := core.hits.Load()
	if total := h + m; total > 0 && float64(h)/float64(total) < pol.minHitRatio {
		e.store.demoteEntity(e.name)
		return
	}
	if qps, mature := core.qps.rate(e.store.now(), pol.window, pol.buckets); mature && qps < pol.minQPS {
		e.store.demoteEntity(e.name)
	}
}

// expAt 过期时间戳（UnixNano；L1 过期面，与 generation 同源校验）。
func (e *Entity[T]) expAt(ttl time.Duration) int64 { return e.store.now().Add(ttl).UnixNano() }

// l1Expired L1 条目是否过期（负缓存与正缓存同面；0=无过期信息视为过期，保守）。
func (e *Entity[T]) l1Expired(env cachedEnvelope) bool {
	return env.Exp <= 0 || env.Exp < e.store.now().UnixNano()
}

func checkSize(name string, b []byte, max int) error {
	if len(b) > max {
		return errors.New("cache: value exceeds max size (drop or raise admission)")
	}
	return nil
}
