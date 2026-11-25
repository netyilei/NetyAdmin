// casbin.go：Casbin RBAC 实现（——SyncedEnforcer 并发安全）。
// 策略装载器注入（RB-01 接表；测试注入内存行）；策略变更 reload 由调用方
// 经 Bus platform.rbac.policy.changed 事件驱动（订阅装配随——kernel 不持
// topic 常量，登记）。
// 策略只读：adapter 写路径显式拒绝（策略变更走模块 → Bus → Reload）。
package authz

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/casbin/casbin/v3"
	"github.com/casbin/casbin/v3/model"
	"github.com/casbin/casbin/v3/persist"

	"github.com/netyilei/netyadmin/internal/kernel/security/authn"
)

// Options 是 Casbin 装配参数。
type Options struct {
	// ModelText 覆盖默认 RBAC 模型（空=默认，见 model.go）。
	ModelText string
	// Loader 返回全部策略行（行含 ptype 前缀：["p", sub, obj, act] / ["g", role, parent]，
	// persist.LoadPolicyArray 官方语义）。 RB-01 接表；nil = 空策略（全 deny）。
	// ctx=装载限时（F3：persist.Adapter 接口无 ctx 参数——由 Reload
	// 在写锁下显式交接；构造期装载为 Background）。
	Loader func(ctx context.Context) ([][]string, error)
}

// CasbinAuthorizer 是 Authorizer 的 RBAC 实现（admin 主体判定）。
type CasbinAuthorizer struct {
	mu       sync.RWMutex
	enforcer *casbin.SyncedEnforcer
	loader   *funcLoader // ctx 交接点（Reload 写、LoadPolicy 读——mu 同锁串行）
}

// NewCasbinAuthorizer 创建并装载策略（SyncedEnforcer：Enforce 读并发安全）。
func NewCasbinAuthorizer(opts Options) (*CasbinAuthorizer, error) {
	text := opts.ModelText
	if text == "" {
		text = defaultModel
	}
	m, err := model.NewModelFromString(text)
	if err != nil {
		return nil, fmt.Errorf("authz: model: %w", err)
	}
	fl := &funcLoader{loader: opts.Loader}
	e, err := casbin.NewSyncedEnforcer(m, fl)
	if err != nil {
		return nil, fmt.Errorf("authz: enforcer: %w", err)
	}
	return &CasbinAuthorizer{enforcer: e, loader: fl}, nil
}

// Authorize 判定 admin 主体。subject=Principal.ID（角色不进 token——RB-01
// admin→role 映射走 g 策略行（rbac_admin_roles），reload 即生效，无重登录）。
// service/app 主体本实现不覆盖——scope 白名单在 TR-02/ 编排，
// 返回 deny not_supported（非错误）。
func (a *CasbinAuthorizer) Authorize(_ context.Context, p authn.Principal, resource, action string) (Decision, error) {
	if p.Type != authn.PrincipalAdmin {
		return Decision{Allowed: false, Reason: ReasonNotSupported}, nil
	}
	a.mu.RLock()
	e := a.enforcer
	a.mu.RUnlock()
	allowed, err := e.Enforce(p.ID, resource, action)
	if err != nil {
		return Decision{}, fmt.Errorf("authz: enforce: %w", err)
	}
	if allowed {
		return Decision{Allowed: true}, nil
	}
	return Decision{Allowed: false, Reason: ReasonNoMatch}, nil
}

// Reload 重新装载策略（经 Bus platform.rbac.policy.changed 广播驱动；
// 装载失败保留旧策略并返回错误——调用方告警，判定仍 fail-closed 可用）。
// ctx 贯穿 Loader 查询（F3：persist.Adapter 接口无 ctx——写锁下
// 交接，Reload 串行保证不串扰；nil=Background 语义）。
func (a *CasbinAuthorizer) Reload(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.loader.setCtx(ctx)
	if err := a.enforcer.LoadPolicy(); err != nil {
		return fmt.Errorf("authz: reload: %w", err)
	}
	return nil
}

// funcLoader 是策略行装载适配器（persist.Adapter 最小实现；写路径拒绝——
// 策略单一事实源在 rbac 模块表，变更走 Bus reload）。
type funcLoader struct {
	loader func(ctx context.Context) ([][]string, error)
	ctx    context.Context // 当前装载 ctx（构造期 nil=Background；Reload 写锁下更新）
}

// setCtx 交接装载 ctx（仅 Reload 在 mu 写锁下调用；nil 归一 Background）。
func (l *funcLoader) setCtx(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	l.ctx = ctx
}

func (l *funcLoader) LoadPolicy(m model.Model) error {
	if l.loader == nil {
		return nil // 空策略：全 deny（fail-closed）
	}
	ctx := l.ctx
	if ctx == nil {
		ctx = context.Background() // 构造期装载（NewSyncedEnforcer 首次 LoadPolicy）
	}
	rows, err := l.loader(ctx)
	if err != nil {
		return err
	}
	for _, rule := range rows {
		if len(rule) == 0 {
			continue
		}
		if err := persist.LoadPolicyArray(rule, m); err != nil {
			return fmt.Errorf("authz: policy row %v: %w", rule, err)
		}
	}
	return nil
}

func (l *funcLoader) SavePolicy(model.Model) error {
	return errors.New("authz: policy save not supported (read-only; changes reload via bus)")
}

func (l *funcLoader) AddPolicy(_, _ string, _ []string) error {
	return errors.New("authz: policy add not supported (read-only)")
}

func (l *funcLoader) RemovePolicy(_, _ string, _ []string) error {
	return errors.New("authz: policy remove not supported (read-only)")
}

func (l *funcLoader) RemoveFilteredPolicy(_, _ string, _ int, _ ...string) error {
	return errors.New("authz: policy remove not supported (read-only)")
}
