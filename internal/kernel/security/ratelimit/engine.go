// engine.go：限流策略引擎——命名策略注册表 + 维度键 +
// coordination 执行（滑动窗口 Lua + contracts 失败模式镜像）。
//
// 分工：策略名/失败模式的唯一事实源在 contracts/state-policies.yaml
// coordination.rate_limit.policies（镜像校验 + CI check-contracts-mirror
// 对偶）；本引擎只补「维度 × 限额 × 窗口」的装配面，未登记名一律 fail-closed。
// 算法当前唯一实现=滑动窗口（Lua；tech-stack §9 已否固定窗口库）——
// token_bucket/fixed_window 随具名消费者扩展（登记）。
package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/netyilei/netyadmin/internal/kernel/state/coordination"
)

// Dimension 是限流维度（键提取由调用方完成——kernel 不 import gin）。
type Dimension string

const (
	DimensionIP    Dimension = "ip"    // 客户端 IP（transport ClientIP）
	DimensionUser  Dimension = "user"  // Principal ID（ctx）
	DimensionApp   Dimension = "app"   // App ID（网关）
	DimensionRoute Dimension = "route" // 路由模板
	DimensionAll   Dimension = "global"
)

// ErrDenied/ErrUnavailable 别名（调用方统一 errors.Is 判定；语义实义在）。
var (
	ErrDenied      = coordination.ErrDenied      // 超限（→10007 429 retryable）
	ErrUnavailable = coordination.ErrUnavailable // 不可用且 fail-closed（→10010）
)

// Policy 是一条命名策略的装配声明。
type Policy struct {
	Name      string        `json:"name"`      // 必须已登记 contracts rate_limit.policies
	Dimension Dimension     `json:"dimension"` // 维度
	Limit     int           `json:"limit"`     // 窗口内允许次数（>0）
	Window    time.Duration `json:"window"`    // 窗口（>0）
}

// Engine 是策略注册表（构造后 Register 完成；Allow 只读分发）。
type Engine struct {
	coord    *coordination.Coordination
	mu       sync.RWMutex
	policies map[string]Policy
}

// NewEngine 创建（coord 执行面：Redis 滑动窗口 / single 本地回退，降级语义
// 按 contracts 策略由统一决策）。
func NewEngine(coord *coordination.Coordination) *Engine {
	return &Engine{coord: coord, policies: map[string]Policy{}}
}

// Register 注册策略装配（重复名 / 未登记 contracts 名 / 非法参数 → 错误）。
func (e *Engine) Register(p Policy) error {
	if p.Name == "" {
		return errors.New("ratelimit: policy name required")
	}
	if p.Limit <= 0 || p.Window <= 0 {
		return fmt.Errorf("ratelimit: policy %q limit/window must be positive", p.Name)
	}
	switch p.Dimension {
	case DimensionIP, DimensionUser, DimensionApp, DimensionRoute, DimensionAll:
	default:
		return fmt.Errorf("ratelimit: policy %q unknown dimension %q", p.Name, p.Dimension)
	}
	if !coordination.RateLimitPolicyRegistered(p.Name) {
		return fmt.Errorf("ratelimit: policy %q not registered in contracts/state-policies.yaml (fail-closed)", p.Name)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, dup := e.policies[p.Name]; dup {
		return fmt.Errorf("ratelimit: duplicate policy %q", p.Name)
	}
	e.policies[p.Name] = p
	return nil
}

// Allow 判定 dimKey 维度值是否放行（超限 ErrDenied；不可用按 contracts 语义
// fail-open 放行 / fail-closed ErrUnavailable—— 统一决策并计数）。
// 未注册策略名 → 装配错误（拒绝，防裸名绕过注册表）。
func (e *Engine) Allow(ctx context.Context, name, dimKey string) error {
	e.mu.RLock()
	p, ok := e.policies[name]
	e.mu.RUnlock()
	if !ok {
		return fmt.Errorf("ratelimit: policy %q not registered (fail-closed)", name)
	}
	return e.coord.AllowByPolicy(ctx, name, dimKey, p.Limit, p.Window)
}

// Policy 返回已注册策略快照（诊断面用）。
func (e *Engine) Policies() []Policy {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]Policy, 0, len(e.policies))
	for _, p := range e.policies {
		out = append(out, p)
	}
	return out
}
