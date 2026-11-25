// ipac_engine.go：IP 访问控制引擎（—— ipac 引擎收拢 gw；
// 配置组模型：规则归策略组，应用引用组集；「禁止 > 开放」组集严格合成）。
//
// 判定语义（用户 2026-11-21 定档「按严格方式」）：
//   - 组集内任一组 deny 规则命中 → 拒（deny 跨组恒优先——全局/应用分层废除）
//   - 组集内 allow 命中（且无 deny 命中）→ 放
//   - 组集非空但无任何命中 → 拒（fail-closed 白名单语义——启用组=显式管控）
//   - 组集空（应用未启用任何组）→ 不做 IP 判定（不限——零配置不拒）
package application

import (
	"context"
	"fmt"
	"net/netip"
	"strings"
	"sync/atomic"

	"github.com/netyilei/netyadmin/internal/kernel/database"
	"github.com/netyilei/netyadmin/internal/modules/gw/domain"
)

// loadedIPRule 是解析后的启用规则（快照内）。
type loadedIPRule struct {
	ID       string
	GroupID  string
	Prefixes []netip.Prefix
	Action   domain.IPAction
}

// ipSnapshot 是一次装载完成的不可变启用规则集。
type ipSnapshot struct {
	Rules []loadedIPRule
}

// IPEngine 是 IP 规则快照持有者（Reload 全量替换；MatchGroups 只读）。
type IPEngine struct {
	snap atomic.Pointer[ipSnapshot]
}

// NewIPEngine 创建（未 Reload 时 MatchGroups 全拒——fail-closed）。
func NewIPEngine() *IPEngine { return &IPEngine{} }

// Reload 从库装载启用规则并原子替换快照（CIDR 解析失败整批拒绝——fail-closed，
// 调用方告警保留旧快照）。
func (e *IPEngine) Reload(ctx context.Context, pool *database.Pool) error {
	rows, err := pool.Query(ctx,
		`SELECT id, group_id, cidrs, action FROM gw_ip_rules WHERE enabled = true`)
	if err != nil {
		return fmt.Errorf("gw: load ip rules: %w", err)
	}
	defer rows.Close()

	snap := &ipSnapshot{}
	for rows.Next() {
		var r loadedIPRule
		var cidrs string
		var actionNum int16
		if err := rows.Scan(&r.ID, &r.GroupID, &cidrs, &actionNum); err != nil {
			return fmt.Errorf("gw: scan ip rule: %w", err)
		}
		prefixes, err := parseIPCIDRs(cidrs)
		if err != nil {
			return fmt.Errorf("gw: ip rule %s: %w", r.ID, err)
		}
		switch domain.IPAction(actionNum) {
		case domain.IPActionAllow, domain.IPActionDeny:
			r.Action = domain.IPAction(actionNum)
		default:
			return fmt.Errorf("gw: ip rule %s: unknown action %d", r.ID, actionNum)
		}
		r.Prefixes = prefixes
		snap.Rules = append(snap.Rules, r)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("gw: iterate ip rules: %w", err)
	}
	e.snap.Store(snap)
	return nil
}

// MatchGroups 组集严格合成判定（应用启用的组集；deny 跨组恒优先；组集空=不限）。
func (e *IPEngine) MatchGroups(ip netip.Addr, groupIDs []string) domain.IPDecision {
	snap := e.snap.Load()
	if snap == nil {
		return domain.IPDecision{Allowed: false, Reason: "fail_closed"}
	}
	if len(groupIDs) == 0 {
		// 组集空=应用未配置 IP 管控（零配置不拒——部署体验保留）
		return domain.IPDecision{Allowed: true, Reason: "no_group"}
	}
	allowHit := false
	hitID := ""
	for i := range snap.Rules {
		r := &snap.Rules[i]
		if !groupIn(r.GroupID, groupIDs) || !ipInPrefixes(r.Prefixes, ip) {
			continue
		}
		switch r.Action {
		case domain.IPActionDeny:
			return domain.IPDecision{Allowed: false, Reason: r.ID} // deny 命中即拒（恒优先）
		case domain.IPActionAllow:
			if !allowHit {
				allowHit = true
				hitID = r.ID
			}
		default:
			return domain.IPDecision{Allowed: false, Reason: "unknown_action"}
		}
	}
	if allowHit {
		return domain.IPDecision{Allowed: true, Reason: hitID}
	}
	// 组集非空无命中 → deny（fail-closed 白名单语义）
	return domain.IPDecision{Allowed: false, Reason: "no_match"}
}

func groupIn(groupID string, groupIDs []string) bool {
	for _, g := range groupIDs {
		if g == groupID {
			return true
		}
	}
	return false
}

func ipInPrefixes(prefixes []netip.Prefix, ip netip.Addr) bool {
	for _, p := range prefixes {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}

// ValidateIPCIDRs 校验逗号分隔 CIDR 列表（管理面写路径复用引擎解析口径——
// 非法条目/空列表=拒绝）。
func ValidateIPCIDRs(csv string) ([]netip.Prefix, error) {
	return parseIPCIDRs(csv)
}

func parseIPCIDRs(csv string) ([]netip.Prefix, error) {
	var out []netip.Prefix
	for _, part := range strings.Split(csv, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		p, err := netip.ParsePrefix(part)
		if err != nil {
			return nil, fmt.Errorf("cidr %q: %w", part, err)
		}
		out = append(out, p.Masked())
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("empty cidr list")
	}
	return out, nil
}
