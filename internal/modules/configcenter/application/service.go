// service.go：configcenter 编排——CRUD + 快照读 + 变更历史/回滚 +
// 热更 Watcher。变更编排（AGENTS §4「版本+校验+回滚」）：
//
//	写请求 → 值类型校验 → 事务面①（变更前整表快照落 history vN+1）→ 写操作
//	→ commit 后广播 platform.config.changed（G2：失效在 commit 之后）→ 各节点
//	Watcher reload（含本节点订阅——单节点经 Bus Local 域同步触发）。
//
// Watcher 是进程内快照 + 回调注册表（rbac/ipac reload 同模式）：Get 走内存
// 快照（L1 hot_local 语义——小/全局/读多写少），回调消费者（log 级别/CORS）
// 由 cmd 组合根接线。回滚 = 取历史快照整表写回 + 新版本 + 广播（回滚也是变更）。
package application

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync/atomic"

	"github.com/netyilei/netyadmin/internal/kernel/bus"
	"github.com/netyilei/netyadmin/internal/modules/configcenter/api/events"
	"github.com/netyilei/netyadmin/internal/modules/configcenter/domain"
)

// Service 是配置编排入口（AttachDB 后可用；Watcher 独立构造可先注册回调）。
type Service struct {
	repo    domain.Repo
	bus     *bus.Bus // 变更广播源（可 nil=测试；广播失败告警不回滚——reload 可恢复）
	logger  logger
	version atomic.Int64 // 当前生效版本快照（诊断面/watcher 比对用）
}

// logger 是最小日志面（*slog.Logger 满足；测试注入）。
type logger interface {
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

// New 创建（bus/logger 可空）。
func New(eventBus *bus.Bus, log logger) *Service {
	if log == nil {
		log = nopLogger{}
	}
	return &Service{repo: nil, bus: eventBus, logger: log}
}

// Attach 接线存储（module.go AttachDB；装载快照版本）。ctx 由调用方限时
// （F3：初始装载超时走降级路径——DB 挂起不卡死 bootstrap）。
func (s *Service) Attach(ctx context.Context, repo domain.Repo) error {
	if repo == nil {
		return fmt.Errorf("configcenter: nil repo")
	}
	s.repo = repo
	v, err := repo.CurrentVersion(ctx)
	if err != nil {
		return err
	}
	s.version.Store(v)
	return nil
}

// nopLogger 空实现（测试注入缺省）。
type nopLogger struct{}

func (nopLogger) Warn(string, ...any)  {}
func (nopLogger) Error(string, ...any) {}

// ready 守卫。
func (s *Service) ready() error {
	if s.repo == nil {
		return fmt.Errorf("configcenter: storage not attached")
	}
	return nil
}

// —— 读面 ——

// All 全量配置（管理面列表；量级=运维键，无分页）。
func (s *Service) All(ctx context.Context) ([]domain.Config, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	return s.repo.ListAll(ctx)
}

// Public 公开键集（免鉴权读面；直读 repo——is_public 是低频管理面查询，
// 快照缓存无收益，G1-15 未测量不优化）。
func (s *Service) Public(ctx context.Context) ([]domain.Config, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	return s.repo.ListPublic(ctx)
}

// Get 单键（upsert 判定/诊断面）。
func (s *Service) Get(ctx context.Context, group, key string) (domain.Config, error) {
	if err := s.ready(); err != nil {
		return domain.Config{}, err
	}
	return s.repo.Get(ctx, group, key)
}

// Version 当前生效版本（诊断面）。
func (s *Service) Version() int64 { return s.version.Load() }

// —— 写面（变更编排）——

// 预检语义（G1-02）：业务冲突前置判定不落历史（失败尝试零版本）；事务内
// ApplyChange 兜底预检-写入间 TOCTOU 与系统错误（写失败整体回滚零孤儿——
// G1-16 根修：历史与写同事务）。广播在 ApplyChange commit 成功后（G2）。

// Create 创建键（业务预检：已存在/保留组 → 哨兵，不占版本）。
func (s *Service) Create(ctx context.Context, c domain.Config, operatorID, reason string) error {
	if err := s.ready(); err != nil {
		return err
	}
	if err := precheckCreate(ctx, s.repo, c); err != nil {
		return err
	}
	return s.change(ctx, domain.Change{
		Kind: domain.ChangeCreate, Configs: []domain.Config{c},
		Reason: reason, By: operatorID,
	})
}

// Update 更新键值/描述/公开位（键标识不改；业务预检：不存在 → ErrNotFound）。
func (s *Service) Update(ctx context.Context, c domain.Config, operatorID, reason string) error {
	if err := s.ready(); err != nil {
		return err
	}
	if _, err := s.repo.Get(ctx, c.GroupName, c.ConfigKey); err != nil {
		return err
	}
	return s.change(ctx, domain.Change{
		Kind: domain.ChangeUpdate, Configs: []domain.Config{c},
		Reason: reason, By: operatorID,
	})
}

// BatchUpsert 批量提交（PUT 全量入口；评审修——整批一次变更：
// 单版本/单快照/单广播，中途失败整批回滚无部分应用态）。
func (s *Service) BatchUpsert(ctx context.Context, items []domain.Config, operatorID, reason string) error {
	if err := s.ready(); err != nil {
		return err
	}
	if len(items) == 0 {
		return domain.ErrNotFound
	}
	for i := range items { // 保留组预检（事务内兜底重复校验）
		if domain.ReservedGroups[items[i].GroupName] {
			return fmt.Errorf("%w: group %q reserved", domain.ErrReservedGroup, items[i].GroupName)
		}
		if err := items[i].Validate(); err != nil {
			return err
		}
	}
	return s.change(ctx, domain.Change{
		Kind: domain.ChangeCreate, Configs: items, // upsert 语义由 repo 写面统一
		Reason: reason, By: operatorID,
	})
}

// Delete 删除键（业务预检：不存在/系统保护；事务内兜底）。
func (s *Service) Delete(ctx context.Context, group, key, operatorID, reason string) error {
	if err := s.ready(); err != nil {
		return err
	}
	existing, err := s.repo.Get(ctx, group, key)
	if err != nil {
		return err
	}
	if existing.IsSystem {
		return domain.ErrSystemProtected
	}
	return s.change(ctx, domain.Change{
		Kind: domain.ChangeDelete, Group: group, Key: key,
		Reason: reason, By: operatorID,
	})
}

// Rollback 回滚到历史版本：恢复「version 变更前」状态（即撤销 version 起的
// 全部变更——整表写回该版本快照 + 新版本 + 广播；回滚也是变更，历史链只增
// 不改可再次回滚；业务预检：目标版本不存在 → ErrNotFound）。
func (s *Service) Rollback(ctx context.Context, version int64, operatorID string) error {
	if err := s.ready(); err != nil {
		return err
	}
	if _, err := s.repo.GetHistory(ctx, version); err != nil {
		return err
	}
	return s.change(ctx, domain.Change{
		Kind: domain.ChangeRollback, RollbackTo: version,
		Reason: "rollback to v" + strconv.FormatInt(version, 10), By: operatorID,
	})
}

// precheckCreate 创建前置判定（已存在 → ErrKeyExists；保留组 → ErrReservedGroup）。
func precheckCreate(ctx context.Context, repo domain.Repo, c domain.Config) error {
	if domain.ReservedGroups[c.GroupName] {
		return fmt.Errorf("%w: group %q reserved", domain.ErrReservedGroup, c.GroupName)
	}
	if err := c.Validate(); err != nil {
		return err
	}
	if _, err := repo.Get(ctx, c.GroupName, c.ConfigKey); err == nil {
		return domain.ErrKeyExists
	} else if !errors.Is(err, domain.ErrNotFound) {
		return err
	}
	return nil
}

// change 调事务原语 + commit 后广播 + 版本快照推进（ApplyChange 失败内存版本
// 不动——DB 为准，下次变更事务内重新分配；G1-16 根修语义）。
func (s *Service) change(ctx context.Context, ch domain.Change) error {
	version, err := s.repo.ApplyChange(ctx, ch)
	if err != nil {
		return err
	}
	s.version.Store(version)
	s.broadcast(ctx, version)
	return nil
}

// broadcast 变更广播（commit 后；失败告警不阻断——订阅方可周期对账/下次变更
// 收敛，判定无 fail-closed 面）。
func (s *Service) broadcast(ctx context.Context, version int64) {
	if s.bus == nil {
		return
	}
	payload := fmt.Sprintf(`{"version":%d}`, version)
	// Broadcast 域（审计修：缺省 Local 不出节点——multi 下他节点 stale）
	if err := s.bus.Publish(ctx, bus.Message{Topic: events.TopicConfigChanged, Payload: []byte(payload)}, bus.WithDomain(bus.DomainBroadcast)); err != nil {
		s.logger.Error("configcenter: publish config changed", "err", err)
	}
}

// ListHistory 最近变更历史（回滚抽屉消费；limit 默认 50）。
func (s *Service) ListHistory(ctx context.Context) ([]domain.HistoryEntry, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	return s.repo.ListHistory(ctx, 50)
}
