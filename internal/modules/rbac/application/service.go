// service.go：RBAC 编排（RB-01）——Casbin Authorizer 持有者 + 策略装载/重载。
// Authorizer 在 New 时创建（loader 闭包读 s.repo；未接线时空策略=全 deny
// fail-closed），AttachDB 后 Reload 装入真实策略。判定面=authz.Authorizer
// （kernel 契约；adminapi 经 Services.Resolve 消费， 引擎机制）。
package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/netyilei/netyadmin/internal/kernel/bus"
	"github.com/netyilei/netyadmin/internal/kernel/security/authz"
	"github.com/netyilei/netyadmin/internal/modules/rbac/domain"
)

// ErrNotAttached 是 DB 未接线（AttachDB 前 Authorize fail-closed）。
var ErrNotAttached = errors.New("rbac: storage not attached")

// Service 是 RBAC 编排入口。
type Service struct {
	repo       domain.Repo
	authorizer *authz.CasbinAuthorizer
	bus        *bus.Bus     // 策略变更广播源（可 nil=测试；p/g 数据源变更后发布）
	logger     *slog.Logger // 发布失败告警（nil=Default）
	// moduleEnabled 是菜单所属模块的启用判定（组合根在 Registry.Build 后注入；
	// nil=全部可见——判定/管理列表不受影响，仅 RouteTree 展示面过滤）。
	moduleEnabled func(string) bool
}

// SetModuleFilter 注入模块启用判定（组合根启动期调用一次；启用模块集合=
// Registry.Started() 唯一事实源，避免模块开关与菜单可见性脱钩）。
func (s *Service) SetModuleFilter(fn func(string) bool) { s.moduleEnabled = fn }

// New 创建（logger/bus 可空；构造即建 Casbin 实例：loader 闭包读 repo，
// 未接线时空策略全 deny）。
func New(logger *slog.Logger, eventBus *bus.Bus) (*Service, error) {
	if logger == nil {
		logger = slog.Default()
	}
	s := &Service{bus: eventBus, logger: logger}
	a, err := authz.NewCasbinAuthorizer(authz.Options{
		Loader: func(ctx context.Context) ([][]string, error) { return s.loadRows(ctx) },
	})
	if err != nil {
		return nil, fmt.Errorf("rbac: authorizer: %w", err)
	}
	s.authorizer = a
	return s, nil
}

// loadRows 策略行装载（Loader 契约；repo 未接线=空行全 deny）。
func (s *Service) loadRows(ctx context.Context) ([][]string, error) {
	if s.repo == nil {
		return nil, nil
	}
	rows, err := s.repo.LoadPolicyRows(ctx)
	if err != nil {
		return nil, fmt.Errorf("rbac: load policy rows: %w", err)
	}
	out := make([][]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, []string(r))
	}
	return out, nil
}

// AttachDB 接线存储并立即装载（module.go AttachDB 调用；命名与 bus/scheduler
// AttachDB 惯例一致——cmd attachModules 统一断言该签名）。ctx 由调用方限时
// （F3：初始装载超时走 deny-all 降级——DB 挂起不卡死 bootstrap）。
func (s *Service) Attach(ctx context.Context, repo domain.Repo) error {
	if repo == nil {
		return errors.New("rbac: nil repo")
	}
	s.repo = repo
	return s.Reload(ctx)
}

// Reload 重载策略（Bus domain.rbac.policy.changed 订阅回调/管理面写入后）；
// 失败保留旧策略（语义）。ctx 贯穿装载查询（casbin Adapter 无 ctx
// 参数——kernel funcLoader 经 Reload 写锁下交接）。
func (s *Service) Reload(ctx context.Context) error {
	if s.repo == nil {
		return ErrNotAttached
	}
	if err := s.authorizer.Reload(ctx); err != nil {
		return fmt.Errorf("rbac: reload: %w", err)
	}
	return nil
}

// Authorizer 暴露判定面（authz.Authorizer 契约值；adminapi 中间件消费）。
func (s *Service) Authorizer() authz.Authorizer { return s.authorizer }

// EnsureBootstrapBinding 首启绑定：bootstrap admin → super-admin 角色
// （幂等：已绑定跳过；super-admin 角色缺 = seed 未跑 → 报错 fail-closed；
// adminID 空 = bootstrap 账户缺席——不绑定不产脏行，2026-11-21 真机复盘根修）。
func (s *Service) EnsureBootstrapBinding(ctx context.Context, adminID string) (bool, error) {
	if s.repo == nil {
		return false, ErrNotAttached
	}
	if adminID == "" {
		return false, nil
	}
	roleID, err := s.repo.FindRoleIDByCode(ctx, domain.SuperAdminRoleCode)
	if err != nil {
		return false, err
	}
	bound, err := s.repo.HasAdminBinding(ctx, adminID, roleID)
	if err != nil {
		return false, fmt.Errorf("rbac: check binding: %w", err)
	}
	if bound {
		return false, nil
	}
	if err := s.repo.BindAdmin(ctx, adminID, roleID); err != nil {
		return false, err
	}
	return true, nil
}
