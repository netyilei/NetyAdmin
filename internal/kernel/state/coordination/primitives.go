// primitives.go：锁/Nonce/幂等/选主/计数器/限流——Coordination 平面五原语 + 限流策略引擎。
// 统一模式：Redis 操作 → 失败按 Policy 分发（contracts/state-policies.yaml 镜像，
// decideUnavailable 统一决策）；无前置探活（操作错误即不可用信号，不做双 RTT）。
package coordination

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// releaseLockLua 释放锁的原子校验：持有者匹配才删（防误删他人锁）。
const releaseLockLua = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("DEL", KEYS[1])
end
return 0
`

// renewLockLua 续租：持有者匹配才 PEXPIRE。
const renewLockLua = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("PEXPIRE", KEYS[1], ARGV[2])
end
return 0
`

// slidingWindowLua ZSET 滑动窗口：清理窗外 → 计数 → 未超限则记录 → 设置过期。
const slidingWindowLua = `
local key = KEYS[1]
local now = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local limit = tonumber(ARGV[3])
redis.call("ZREMRANGEBYSCORE", key, 0, now - window)
local count = redis.call("ZCARD", key)
if count >= limit then
	return -count
end
local member = now .. "-" .. math.random(100000, 999999)
redis.call("ZADD", key, now, member)
redis.call("PEXPIRE", key, window)
return limit - count - 1
`

// —— 互斥锁（policy: lock） ——

// LockKey 尝试获取互斥锁（SET NX PX + 持有者 token）。
// 返回 (holder, acquired)；acquired=false 表示已被占用。
func (c *Coordination) LockKey(ctx context.Context, key string, ttl time.Duration) (holder string, acquired bool, err error) {
	holder = newHolder()
	ok, err := c.client.SetNX(ctx, c.key(prefixLock, key), holder, ttl)
	if err == nil {
		return holder, ok, nil
	}
	allow, local, derr := c.decide("lock", PrimitivePolicy("lock"))
	if derr != nil {
		return "", false, derr
	}
	if local {
		if c.local.setNX(c.key(prefixLock, key), ttl) {
			return "local:" + holder, true, nil
		}
		return "", false, nil
	}
	if allow {
		return "fail-open", true, nil // lock 策略无 fail_open 声明；防御分支
	}
	return "", false, ErrUnavailable
}

// UnlockKey 释放锁（Lua 原子校验持有者；本地回退锁走本地删除）。
func (c *Coordination) UnlockKey(ctx context.Context, key, holder string) error {
	return c.release(ctx, c.key(prefixLock, key), holder)
}

// RenewLock 续租（校验持有者后 PEXPIRE；非持有者 ErrDenied）。
func (c *Coordination) RenewLock(ctx context.Context, key, holder string, ttl time.Duration) error {
	return c.renew(ctx, c.key(prefixLock, key), holder, ttl)
}

// release/renew 内部实现：拿完整键（各调用方自带前缀）。
func (c *Coordination) release(ctx context.Context, fullKey, holder string) error {
	if _, err := c.client.Eval(ctx, releaseLockLua, []string{fullKey}, holder); err != nil {
		if _, local, derr := c.decide("lock", PrimitivePolicy("lock")); derr == nil && local {
			c.local.del(fullKey) // 本地回退锁：持有者直接删（本地单进程无竞态）
			return nil
		}
		return fmt.Errorf("coordination: unlock %s: %w", fullKey, err)
	}
	return nil
}

func (c *Coordination) renew(ctx context.Context, fullKey, holder string, ttl time.Duration) error {
	v, err := c.client.Eval(ctx, renewLockLua, []string{fullKey}, holder, ttl.Milliseconds())
	if err != nil {
		return fmt.Errorf("coordination: renew %s: %w", fullKey, err)
	}
	if n, _ := v.(int64); n == 0 {
		return ErrDenied
	}
	return nil
}

// —— Nonce（防重放；policy: nonce） ——

// CheckNonce 首次出现返回 true 并占用（TTL 内拒绝重放）。
func (c *Coordination) CheckNonce(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	ok, err := c.client.SetNX(ctx, c.key(prefixNonce, key), 1, ttl)
	if err == nil {
		return ok, nil
	}
	allow, local, derr := c.decide("nonce", PrimitivePolicy("nonce"))
	if derr != nil {
		return false, derr
	}
	if local {
		return c.local.setNX(c.key(prefixNonce, key), ttl), nil
	}
	if allow {
		return true, nil
	}
	return false, ErrUnavailable
}

// ReleaseIdempotency 释放幂等键（5xx 未提交场景允许重试；Del 幂等可重入）。
// 降级语义同 BeginIdempotency（single 本地回退 / multi fail-closed——释放失败
// 返回错误，调用方按不可用处理：键将按 TTL 自然过期，重试窗口收窄但不破坏正确性）。
func (c *Coordination) ReleaseIdempotency(ctx context.Context, key string) error {
	delErr := c.client.Del(ctx, c.key(prefixIdem, key))
	if delErr == nil {
		return nil
	}
	_, local, derr := c.decide("idempotency", PrimitivePolicy("idempotency"))
	if derr != nil {
		return derr
	}
	if local {
		c.local.del(c.key(prefixIdem, key))
		return nil
	}
	// multi fail-closed 与 BeginIdempotency 对称：不可用哨兵（调用方映射 10010）；
	// 释放失败时键按 TTL 过期兜底，重试窗口收窄但不破坏正确性。
	return fmt.Errorf("%w: release idempotency %s: %v", ErrUnavailable, key, delErr)
}

// —— 幂等键（policy: idempotency，multi fail-closed；local_or_db 的 DB 分支随幂等表接入） ——

// BeginIdempotency 幂等键首次出现返回 nil；重复出现返回 ErrDenied。
func (c *Coordination) BeginIdempotency(ctx context.Context, key string, ttl time.Duration) error {
	ok, err := c.client.SetNX(ctx, c.key(prefixIdem, key), 1, ttl)
	if err == nil {
		if !ok {
			return ErrDenied
		}
		return nil
	}
	allow, local, derr := c.decide("idempotency", PrimitivePolicy("idempotency"))
	if derr != nil {
		return derr
	}
	if local {
		if !c.local.setNX(c.key(prefixIdem, key), ttl) {
			return ErrDenied
		}
		return nil
	}
	if allow {
		return nil
	}
	return ErrUnavailable
}

// —— 选主（policy: leader；multi 的 PG advisory 分支随接入，当前 fail） ——

// Campaign 尝试成为 leader（Redis 租约 = 锁语义；键与 Renew/Resign 同前缀）。
func (c *Coordination) Campaign(ctx context.Context, name string, ttl time.Duration) (holder string, isLeader bool, err error) {
	holder = newHolder()
	ok, err := c.client.SetNX(ctx, c.key(prefixLeader, name), holder, ttl)
	if err == nil {
		return holder, ok, nil
	}
	// PG advisory 分支：multi + Redis 挂 + 策略 pg_advisory_lock_or_fail
	// + pool 已注入 → 转会话级 advisory 选主（advisory.go；fail-closed 兜底）。
	if c.pgPool != nil && c.topo == TopologyMulti && PrimitivePolicy("leader").OnMulti == PGAdvisoryOrFail {
		return c.pgCampaign(ctx, name)
	}
	allow, local, derr := c.decide("leader", PrimitivePolicy("leader"))
	if derr != nil {
		return "", false, derr
	}
	if local {
		if c.local.setNX(c.key(prefixLeader, name), ttl) {
			return "local:" + holder, true, nil
		}
		return "", false, nil
	}
	if allow {
		return "fail-open", true, nil
	}
	return "", false, ErrUnavailable
}

// RenewLeadership 续租 leader 租约（PG advisory 分支=会话锁随连接生命周期，
// 无续租语义——持有者校验后恒成功；分派按 holder 前缀——Redis 挂时 Redis 分支
// 不再吞掉 PG holder 的操作）。
func (c *Coordination) RenewLeadership(ctx context.Context, name, holder string, ttl time.Duration) error {
	if isPGHolder(holder) {
		return c.pgRenew(ctx, name, holder)
	}
	return c.renew(ctx, c.key(prefixLeader, name), holder, ttl)
}

// Resign 主动让位（分派按 holder 前缀——pg: 前缀走 advisory 面）。
func (c *Coordination) Resign(ctx context.Context, name, holder string) error {
	if isPGHolder(holder) {
		return c.pgResign(ctx, name, holder)
	}
	return c.release(ctx, c.key(prefixLeader, name), holder)
}

// isPGHolder 判定 holder 是否 PG advisory 分支产物（pgCampaign 返回 "pg:"+name）。
func isPGHolder(holder string) bool { return len(holder) > 3 && holder[:3] == "pg:" }

// —— 计数器（policy: counter；首增设 TTL，n>1 不续期——窗口按到期重置） ——

// IncrCounter 原子递增（返回递增后的值；Redis 不可用时按策略）。
func (c *Coordination) IncrCounter(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	n, err := c.client.IncrBy(ctx, c.key(prefixCounter, key), 1)
	if err == nil {
		if n == 1 {
			if _, xerr := c.client.Expire(ctx, c.key(prefixCounter, key), ttl); xerr != nil {
				return n, fmt.Errorf("coordination: counter %s expire: %w", key, xerr)
			}
		}
		return n, nil
	}
	allow, local, derr := c.decide("counter", PrimitivePolicy("counter"))
	if derr != nil {
		return 0, derr
	}
	if local {
		return c.local.incr(c.key(prefixCounter, key), ttl), nil
	}
	if allow {
		return 1, nil // fail_open：视为首次
	}
	return 0, ErrUnavailable
}

// incrRenewLua 原子递增+按滑动 TTL 续期（序号类计数：持续流量下不因窗口到期
// 归零——与 IncrCounter 的「首设 TTL、n>1 不续期」固定窗口语义区分）。
const incrRenewLua = `
local n = redis.call("INCR", KEYS[1])
redis.call("PEXPIRE", KEYS[1], ARGV[1])
return n
`

// IncrSequence 原子递增并滑动续期（多节点单调序号——WS 频道 seq 等；每次
// 调用都刷新 TTL，空闲超时才重置）。Redis 不可用时按 counter 策略
// （single=本地回退；multi=fail_closed——跨节点序号不能失真）。
func (c *Coordination) IncrSequence(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	v, err := c.client.Eval(ctx, incrRenewLua, []string{c.key(prefixCounter, key)}, ttl.Milliseconds())
	if err == nil {
		n, _ := v.(int64)
		return n, nil
	}
	allow, local, derr := c.decide("counter", PrimitivePolicy("counter"))
	if derr != nil {
		return 0, derr
	}
	if local {
		return c.local.incrSliding(c.key(prefixCounter, key), ttl), nil
	}
	if allow {
		return 1, nil // fail_open：视为首次
	}
	return 0, ErrUnavailable
}

// —— 限流（滑动窗口 Lua；命名策略见 contracts，未知名 fail-closed） ——

// AllowByPolicy 按命名策略限流（dimensionKey 已含维度值）。
// 超限返回 ErrDenied；Redis 不可用按策略 fail_open（放行）或 fail_closed。
func (c *Coordination) AllowByPolicy(ctx context.Context, policyName, dimensionKey string, limit int, window time.Duration) error {
	policy := RateLimitPolicy(policyName)
	v, err := c.client.Eval(ctx, slidingWindowLua,
		[]string{c.key(prefixRate, policyName+":"+dimensionKey)}, // 铁律 6：键必须带 ns 环境段
		time.Now().UnixMilli(), window.Milliseconds(), limit)
	if err == nil {
		if n, _ := v.(int64); n < 0 {
			return ErrDenied
		}
		return nil
	}
	allow, _, derr := c.decide("ratelimit:"+policyName, policy)
	if derr != nil {
		return derr
	}
	if allow {
		return nil // fail_open
	}
	return ErrUnavailable // fail_closed
}

// —— 帮手 ——

// newHolder 生成锁持有者 token（随机，防跨进程误删）。
func newHolder() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("holder-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// —— 存活标记（Heartbeat/Alive；多节点 liveness 判定，审计 A12-3） ——

// Heartbeat 标记实例存活（TTL 续期刷新；键 coord:{ns}:alive:{name}）。
// Redis 不可用时按 topology 判定：single 本地降级、multi fail-closed 返回错误
// （调用方告警——心跳断意味着他节点将视本节点为死）。
func (c *Coordination) Heartbeat(ctx context.Context, name string, ttl time.Duration) error {
	err := c.client.SetTTL(ctx, c.key(prefixAlive, name), "1", ttl)
	if err == nil {
		return nil
	}
	if allow, local, derr := c.decide("heartbeat", PrimitivePolicy("lock")); derr == nil {
		if local {
			c.local.setTTL(c.key(prefixAlive, name), "1", ttl)
			return nil
		}
		if allow {
			return nil // fail-open：标记尽力而为
		}
	}
	return fmt.Errorf("%w: heartbeat %s", ErrUnavailable, name)
}

// Alive 实例存活判定（存活标记键是否存在且未过期）。
func (c *Coordination) Alive(ctx context.Context, name string) (bool, error) {
	ok, err := c.client.Exists(ctx, c.key(prefixAlive, name))
	if err == nil {
		return ok, nil
	}
	if allow, local, derr := c.decide("alive", PrimitivePolicy("lock")); derr == nil {
		if local {
			return c.local.exists(c.key(prefixAlive, name)), nil
		}
		if allow {
			return true, nil // fail-open：视为存活（保守——宁可留 phantom 不误标死）
		}
	}
	return false, fmt.Errorf("%w: alive check %s", ErrUnavailable, name)
}
