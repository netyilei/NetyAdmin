package ipac

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yl2chen/cidranger"
	"gorm.io/gorm"

	"NetyAdmin/internal/domain/entity/ipac"
	ipacDto "NetyAdmin/internal/interface/admin/dto/ipac"
	"NetyAdmin/internal/pkg/database"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/pubsub"
	ipacRepo "NetyAdmin/internal/repository/ipac"
)

type IPACService interface {
	CheckIP(ctx context.Context, ip string, appID *string) (bool, error)
	ReloadCache(ctx context.Context) error
	// NotifyAndReload 先本地 reload，成功后再广播让其他节点 reload
	// 顺序：本地成功 → 广播；本地失败 → error 冒泡，不广播
	// 广播失败仅记录日志，不阻断本机主流程（最终一致性可接受）
	NotifyAndReload(ctx context.Context) error

	// CRUD
	List(ctx context.Context, req *ipacDto.IPACQuery) ([]*ipac.IPAccessControl, int64, error)
	Create(ctx context.Context, req *ipacDto.CreateIPACReq, operatorID uint) error
	Update(ctx context.Context, req *ipacDto.UpdateIPACReq, operatorID uint) error
	Delete(ctx context.Context, id uint) error
	DeleteBatch(ctx context.Context, ids []uint) error
}

type ipacService struct {
	repo     ipacRepo.IPACRepository
	eventBus pubsub.EventBus
	tm       *database.TransactionManager

	mu sync.RWMutex
	// 用 cidranger.Ranger (path-compressed trie) 替代 []*net.IPNet 线性扫描，
	// 将 CheckIP 从 O(N) 降到 O(log N)。
	// 语义约定：
	//   - globalDeny / appRules[].Deny：始终为非空 Ranger（无规则时为空 trie，Contains 返回 false 即可，不需要区分"未配置"与"未命中"）
	//   - globalAllow / appRules[].Allow：用 nil 表示"未配置白名单"，非 nil 表示"已配置且非空"。
	//     nil 时跳过白名单 fail-closed 逻辑（与原 `len(s.globalAllow) > 0` 语义一致）；
	//     非 nil 时 IP 必须 Contains 命中才放行，否则 fail-closed。
	globalDeny  cidranger.Ranger
	globalAllow cidranger.Ranger
	appRules    map[string]appIPRules

	// 缓存版本指纹（SubTask 22.3）：基于规则 ID + 应用 IP 过滤开关集合的 SHA256 摘要。
	// ReloadCache 时先计算新指纹，与 cached 对比，相同则跳过重建（避免无效重建）。
	cachedRuleFingerprint string
	cachedAppStrategyFP   string
}

type appIPRules struct {
	Allow           cidranger.Ranger // nil 表示未配置白名单
	Deny            cidranger.Ranger // 始终非 nil（空 trie 也用 NewPCTrieRanger）
	IPFilterEnabled bool
}

func NewIPACService(repo ipacRepo.IPACRepository, eventBus pubsub.EventBus, tm *database.TransactionManager) IPACService {
	s := &ipacService{
		repo:       repo,
		eventBus:   eventBus,
		tm:         tm,
		appRules:   make(map[string]appIPRules),
		globalDeny: cidranger.NewPCTrieRanger(),
		// globalAllow 初始为 nil，表示未配置白名单
	}
	// Initial load
	if err := s.ReloadCache(context.Background()); err != nil {
		slog.Warn("ipac initial reload cache failed", "err", err)
	}

	return s
}

// computeRuleFingerprint 计算规则集合的指纹（按 ID 排序后 SHA256）。
// 用于 ReloadCache 的版本号 diff（SubTask 22.3）：指纹相同则跳过重建。
func (s *ipacService) computeRuleFingerprint(rules []*ipac.IPAccessControl) string {
	ids := make([]string, 0, len(rules))
	for _, r := range rules {
		// 含 ID + AppID + IPAddr + Type + Status + ExpiredAt，任一字段变更都会触发重建
		expired := ""
		if r.ExpiredAt != nil {
			expired = r.ExpiredAt.UTC().Format(time.RFC3339Nano)
		}
		appID := ""
		if r.AppID != nil {
			appID = *r.AppID
		}
		ids = append(ids, fmt.Sprintf("%d|%s|%s|%d|%d|%s", r.ID, appID, r.IPAddr, r.Type, r.Status, expired))
	}
	sort.Strings(ids)
	h := sha256.Sum256([]byte(strings.Join(ids, ";")))
	return hex.EncodeToString(h[:])
}

// computeAppStrategyFingerprint 计算应用 IP 过滤开关集合的指纹。
func (s *ipacService) computeAppStrategyFingerprint(appStrategies map[string]bool) string {
	keys := make([]string, 0, len(appStrategies))
	for k := range appStrategies {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	h := sha256.Sum256([]byte(strings.Join(keys, ";")))
	return hex.EncodeToString(h[:])
}

// parseIPNet 解析 IP 或 CIDR 字符串为 *net.IPNet。
// 输入为单 IP 时按 /32 (IPv4) 或 /128 (IPv6) 转 CIDR。
func parseIPNet(ipAddr string) *net.IPNet {
	_, ipNet, err := net.ParseCIDR(ipAddr)
	if err == nil {
		return ipNet
	}
	ip := net.ParseIP(ipAddr)
	if ip == nil {
		return nil
	}
	mask := net.CIDRMask(32, 32)
	if ip.To4() == nil {
		mask = net.CIDRMask(128, 128)
	}
	return &net.IPNet{IP: ip, Mask: mask}
}

func (s *ipacService) ReloadCache(ctx context.Context) error {
	rules, err := s.repo.GetAllEffective(ctx)
	if err != nil {
		return err
	}

	appStrategies, err := s.repo.GetAppIPFilterEnabled(ctx)
	if err != nil {
		return err
	}

	// SubTask 22.3：版本号 diff —— 指纹相同则跳过重建
	ruleFP := s.computeRuleFingerprint(rules)
	appFP := s.computeAppStrategyFingerprint(appStrategies)

	s.mu.RLock()
	sameRule := ruleFP == s.cachedRuleFingerprint
	sameApp := appFP == s.cachedAppStrategyFP
	s.mu.RUnlock()

	if sameRule && sameApp {
		// 规则集合与应用策略均未变更，跳过 trie 重建
		return nil
	}

	newGlobalDeny := cidranger.NewPCTrieRanger()
	var newGlobalAllow cidranger.Ranger // nil 表示未配置白名单
	newAppRules := make(map[string]appIPRules)

	// insertFailed 跟踪是否有规则插入失败。若有失败，不更新 fingerprint，
	// 下次 ReloadCache 会重新尝试构建——避免失败的规则因 fingerprint 匹配而永久缺失。
	insertFailed := false

	for _, r := range rules {
		ipNet := parseIPNet(r.IPAddr)
		if ipNet == nil {
			continue
		}

		if r.AppID == nil || *r.AppID == "" {
			if r.Type == ipac.IPACTypeDeny {
				if err := newGlobalDeny.Insert(cidranger.NewBasicRangerEntry(*ipNet)); err != nil {
					slog.Error("ipac: insert global deny rule failed", "ipAddr", r.IPAddr, "err", err)
					insertFailed = true
				}
			} else {
				if newGlobalAllow == nil {
					newGlobalAllow = cidranger.NewPCTrieRanger()
				}
				if err := newGlobalAllow.Insert(cidranger.NewBasicRangerEntry(*ipNet)); err != nil {
					slog.Error("ipac: insert global allow rule failed", "ipAddr", r.IPAddr, "err", err)
					insertFailed = true
				}
			}
		} else {
			appID := *r.AppID
			ar := newAppRules[appID]
			if ar.Deny == nil {
				ar.Deny = cidranger.NewPCTrieRanger()
			}
			if r.Type == ipac.IPACTypeDeny {
				if err := ar.Deny.Insert(cidranger.NewBasicRangerEntry(*ipNet)); err != nil {
					slog.Error("ipac: insert app deny rule failed", "appID", appID, "ipAddr", r.IPAddr, "err", err)
					insertFailed = true
				}
			} else {
				if ar.Allow == nil {
					ar.Allow = cidranger.NewPCTrieRanger()
				}
				if err := ar.Allow.Insert(cidranger.NewBasicRangerEntry(*ipNet)); err != nil {
					slog.Error("ipac: insert app allow rule failed", "appID", appID, "ipAddr", r.IPAddr, "err", err)
					insertFailed = true
				}
			}
			newAppRules[appID] = ar
		}
	}

	for appID := range appStrategies {
		ar := newAppRules[appID]
		if ar.Deny == nil {
			ar.Deny = cidranger.NewPCTrieRanger()
		}
		// ar.Allow 保留 nil（表示该 app 未配置白名单）
		ar.IPFilterEnabled = true
		newAppRules[appID] = ar
	}

	s.mu.Lock()
	s.globalDeny = newGlobalDeny
	s.globalAllow = newGlobalAllow
	s.appRules = newAppRules
	// 仅当所有规则插入成功时才更新 fingerprint；否则保留旧 fingerprint，
	// 下次 ReloadCache 重新尝试构建，确保失败的规则不会永久缺失。
	if !insertFailed {
		s.cachedRuleFingerprint = ruleFP
	}
	s.cachedAppStrategyFP = appFP
	s.mu.Unlock()

	return nil
}

func (s *ipacService) CheckIP(ctx context.Context, ipStr string, appID *string) (bool, error) {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false, nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// 1. 全局封禁 (Global Deny) - 优先级最高
	// globalDeny 始终非 nil（构造时初始化为空 trie），无规则时 Contains 返回 false
	if s.globalDeny != nil {
		contains, err := s.globalDeny.Contains(ip)
		if err == nil && contains {
			return false, nil
		}
	}

	// 2. 全局放行 (Global Allow) - 白名单语义（fail-closed）
	// 配置了全局 Allow 列表时（非 nil），IP 必须在其中才放行，否则拒绝
	// 这与应用级 Allow 的语义保持一致，避免全局白名单形同虚设
	// nil 表示未配置白名单，跳过此分支继续后续校验（与原 `len(s.globalAllow) > 0` 语义一致）
	if s.globalAllow != nil {
		contains, err := s.globalAllow.Contains(ip)
		if err != nil || !contains {
			return false, nil
		}
		return true, nil
	}

	// 3. 应用级校验
	if appID != nil && *appID != "" {
		if rules, ok := s.appRules[*appID]; ok {
			if !rules.IPFilterEnabled {
				return true, nil
			}
			// 先检查应用封禁（rules.Deny 始终非 nil）
			if rules.Deny != nil {
				contains, err := rules.Deny.Contains(ip)
				if err == nil && contains {
					return false, nil
				}
			}
			// 再检查应用放行（白名单语义）
			// rules.Allow == nil 表示该 app 未配置白名单，跳过此分支走默认放行
			if rules.Allow != nil {
				contains, err := rules.Allow.Contains(ip)
				if err != nil || !contains {
					return false, nil // 白名单已配置但 IP 不在其中 → fail-closed
				}
				return true, nil
			}
		}
	}

	// 默认放行
	return true, nil
}

func (s *ipacService) List(ctx context.Context, req *ipacDto.IPACQuery) ([]*ipac.IPAccessControl, int64, error) {
	// service 层接收 admin DTO，内部构造 repository query（spec B10：service 不应依赖 handler 构造的 repo 类型）
	repoQuery := &ipacRepo.IPACQuery{
		AppID:    req.AppID,
		IPAddr:   req.IPAddr,
		Type:     req.Type,
		Status:   req.Status,
		Page:     req.Current,
		PageSize: req.Size,
	}
	return s.repo.List(ctx, repoQuery)
}

// notifyReload 广播 reload 通知给其他节点。失败仅记录日志，不阻断主流程
func (s *ipacService) notifyReload(ctx context.Context) {
	if s.eventBus != nil {
		if err := s.eventBus.Publish(ctx, pubsub.TopicIPACReload, "reload"); err != nil {
			slog.Error("IPAC 广播 reload 失败", "error", err)
		}
	}
}

// NotifyAndReload 先本地 reload，成功后再广播。
// 顺序纠正：先本地后广播，避免本地失败但其他节点已 reload 导致集群行为分裂。
// 防回环：driver 层基于 SenderID 过滤本节点消息，本节点不会收到自己发出的广播
func (s *ipacService) NotifyAndReload(ctx context.Context) error {
	if err := s.ReloadCache(ctx); err != nil {
		return err
	}
	s.notifyReload(ctx)
	return nil
}

// reloadCacheAndBroadcast 刷新本地缓存并广播给其他节点。
// 用于 DeleteBatch 事务失败路径：前序已 commit 的删除需反映到内存，避免已删规则仍生效拦截用户。
// 与 NotifyAndReload 的差异：本方法吞掉 ReloadCache 错误（仅记录日志），确保即使本地 reload 失败也尝试广播。
func (s *ipacService) reloadCacheAndBroadcast(ctx context.Context) {
	if err := s.ReloadCache(ctx); err != nil {
		slog.Error("ipac reload cache failed", "err", err)
	}
	s.notifyReload(ctx)
}

func (s *ipacService) Create(ctx context.Context, req *ipacDto.CreateIPACReq, operatorID uint) error {
	item := &ipac.IPAccessControl{
		AppID:  req.AppID,
		IPAddr: req.IPAddr,
		Type:   req.Type,
		Reason: req.Reason,
		Status: req.Status,
	}
	item.CreatedBy = operatorID

	if req.ExpiredAt != nil && *req.ExpiredAt != "" {
		t, err := time.Parse(time.DateTime, *req.ExpiredAt)
		if err != nil {
			return errorx.New(errorx.CodeInvalidParams, "过期时间格式错误")
		}
		item.ExpiredAt = &t
	}

	if err := s.repo.Create(ctx, item); err != nil {
		return err
	}
	return s.NotifyAndReload(ctx)
}

func (s *ipacService) Update(ctx context.Context, req *ipacDto.UpdateIPACReq, operatorID uint) error {
	// 先取旧值，再用 DTO 字段 patch，避免 repo.Save 全字段更新把 AppID/IPAddr 覆盖为零值
	old, err := s.repo.GetByID(ctx, req.ID)
	if err != nil {
		// 区分「未找到」与真实 DB 错误（连接失败、查询超时等），避免把 DB 异常误判为 NotFound
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errorx.New(errorx.CodeNotFound, "IPAC 规则不存在")
		}
		slog.Error("ipacRepo.GetByID failed", "id", req.ID, "err", err)
		return fmt.Errorf("ipac.Update: GetByID failed: %w", err)
	}

	old.Type = req.Type
	old.Reason = req.Reason
	old.Status = req.Status
	old.UpdatedBy = operatorID

	if req.ExpiredAt != nil && *req.ExpiredAt != "" {
		t, err := time.Parse(time.DateTime, *req.ExpiredAt)
		if err != nil {
			return errorx.New(errorx.CodeInvalidParams, "过期时间格式错误")
		}
		old.ExpiredAt = &t
	}

	if err := s.repo.Update(ctx, old); err != nil {
		return err
	}
	return s.NotifyAndReload(ctx)
}

func (s *ipacService) Delete(ctx context.Context, id uint) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	return s.NotifyAndReload(ctx)
}

func (s *ipacService) DeleteBatch(ctx context.Context, ids []uint) error {
	// 逐条事务 fail-closed：任一 id 事务失败立即返回错误（已提交的 id 保持删除状态，未处理的 id 不受影响）。
	// 业务规则拒绝（id 不存在 / 查询失败）走 continue 跳过并记录到 skipped，不阻断整个批量。
	// 循环结束后调用一次 NotifyAndReload 即可（不必逐条广播），与 admin/role/user DeleteBatch 模式对齐。
	//
	// 设计权衡（vs 旧单批 SQL repo.DeleteBatch）：
	//   - 旧实现：repo.DeleteBatch(ctx, ids) 一条 SQL 删除，无逐条事务边界，无法满足 RULES.md §13 fail-closed 语义
	//   - 新实现：逐条 TM 单事务原子完成硬删除，事务失败 Rollback + 立即 return，已提交 id 不回滚（符合「逐条」语义）
	//   - 性能：N 个 id = N 次事务，DeleteBatch 是低频管理操作，可接受
	var skipped []string
	for _, id := range ids {
		// 检查存在性（不进事务，避免事务内的查询成本）
		exists, err := s.repo.ExistsByID(ctx, id)
		if err != nil {
			skipped = append(skipped, fmt.Sprintf("IPAC 规则 %d：查询失败 %v", id, err))
			continue
		}
		if !exists {
			skipped = append(skipped, fmt.Sprintf("IPAC 规则 %d：不存在", id))
			continue
		}
		// TM 单事务原子完成硬删除（fail-closed）
		txCtx, tx := s.tm.Begin(ctx)
		if err := s.repo.Delete(txCtx, id); err != nil {
			slog.Error("ipac delete batch: delete failed", "id", id, "err", err)
			s.tm.Rollback(tx)
			// 前序已 commit 的删除需反映到内存缓存，避免已删规则仍生效拦截用户
			s.reloadCacheAndBroadcast(ctx)
			return errorx.New(errorx.CodeInternalError, fmt.Sprintf("IPAC 规则 %d 删除失败", id))
		}
		if err := s.tm.Commit(tx); err != nil {
			slog.Error("ipac delete batch: commit failed", "id", id, "err", err)
			// Commit 失败时 tx 已自动回滚，但前序已 commit 的删除仍需刷新缓存
			s.reloadCacheAndBroadcast(ctx)
			return errorx.New(errorx.CodeInternalError, fmt.Sprintf("IPAC 规则 %d 删除失败", id))
		}
	}
	// 全部处理完成后广播一次 reload
	if err := s.NotifyAndReload(ctx); err != nil {
		slog.Warn("ipac delete batch: notify and reload failed", "err", err)
	}
	if len(skipped) > 0 {
		return errorx.New(errorx.CodeForbidden, fmt.Sprintf("部分 IPAC 规则被跳过：%s", strings.Join(skipped, "; ")))
	}
	return nil
}
