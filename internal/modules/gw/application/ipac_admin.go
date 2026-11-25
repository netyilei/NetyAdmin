// ipac_admin.go：IP 策略组/规则管理（——gw 治理页 IP tab 消费；
// 组/规则 CRUD + 变更后引擎 Reload + Bus 广播回调）。
package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"

	"github.com/netyilei/netyadmin/internal/kernel/database"
	"github.com/netyilei/netyadmin/internal/modules/gw/domain"
)

// IPACManager 是 IP 策略组/规则管理器（组/规则 CRUD pool 直调 SQL——gw repo 惯例；
// 变更后引擎 Reload + onChanged 广播回调（Bus DomainBroadcast——跨节点 reload））。
type IPACManager struct {
	engine    *IPEngine
	pool      *database.Pool
	logger    *slog.Logger
	onChanged func(context.Context)
}

// NewIPACManager 创建（engine=快照持有者；logger=Reload 失败告警面；
// onChanged=变更广播回调可空）。
func NewIPACManager(engine *IPEngine, pool *database.Pool,
	logger *slog.Logger, onChanged func(context.Context)) *IPACManager {
	if logger == nil {
		logger = slog.Default()
	}
	return &IPACManager{engine: engine, pool: pool, logger: logger, onChanged: onChanged}
}

const ipGroupCols = `id, name, remark, created_at, updated_at`

func scanIPGroup(row interface{ Scan(...any) error }) (*domain.IPGroup, error) {
	var g domain.IPGroup
	if err := row.Scan(&g.ID, &g.Name, &g.Remark, &g.CreatedAt, &g.UpdatedAt); err != nil {
		return nil, err
	}
	return &g, nil
}

// ListGroups 组列表。
func (m *IPACManager) ListGroups(ctx context.Context) ([]domain.IPGroup, error) {
	rows, err := m.pool.Query(ctx, `SELECT `+ipGroupCols+` FROM gw_ip_groups ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("gw: list ip groups: %w", err)
	}
	defer rows.Close()
	out := make([]domain.IPGroup, 0, 8)
	for rows.Next() {
		g, err := scanIPGroup(rows)
		if err != nil {
			return nil, fmt.Errorf("gw: scan ip group: %w", err)
		}
		out = append(out, *g)
	}
	return out, rows.Err()
}

// GetGroup 取组。
func (m *IPACManager) GetGroup(ctx context.Context, id string) (*domain.IPGroup, error) {
	g, err := scanIPGroup(m.pool.QueryRow(ctx, `SELECT `+ipGroupCols+` FROM gw_ip_groups WHERE id=$1`, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrIPGroupNotFound
		}
		return nil, fmt.Errorf("gw: get ip group: %w", err)
	}
	return g, nil
}

// GroupsExist 组集存在性校验（应用绑定面——全部命中才算存在；防幽灵组引用）。
func (m *IPACManager) GroupsExist(ctx context.Context, ids []string) error {
	uniq := make([]string, 0, len(ids))
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		if id != "" && !seen[id] {
			seen[id] = true
			uniq = append(uniq, id)
		}
	}
	if len(uniq) == 0 {
		return nil
	}
	var hit int
	if err := m.pool.QueryRow(ctx,
		`SELECT count(*) FROM gw_ip_groups WHERE id = ANY($1)`, uniq).Scan(&hit); err != nil {
		return fmt.Errorf("gw: check ip groups: %w", err)
	}
	if hit != len(uniq) {
		return domain.ErrIPGroupNotFound
	}
	return nil
}

// InsertGroup 创建组。
func (m *IPACManager) InsertGroup(ctx context.Context, g *domain.IPGroup) error {
	_, err := m.pool.Exec(ctx,
		`INSERT INTO gw_ip_groups (id, name, remark, created_at, updated_at) VALUES ($1,$2,$3,now(),now())`,
		g.ID, g.Name, g.Remark)
	if database.IsUniqueViolation(err) {
		return domain.ErrIPGroupNameTaken
	}
	if err != nil {
		return fmt.Errorf("gw: insert ip group: %w", err)
	}
	return nil
}

// UpdateGroup 更新组。
func (m *IPACManager) UpdateGroup(ctx context.Context, g *domain.IPGroup) error {
	tag, err := m.pool.Exec(ctx,
		`UPDATE gw_ip_groups SET name=$2, remark=$3, updated_at=now() WHERE id=$1`,
		g.ID, g.Name, g.Remark)
	if err != nil {
		return fmt.Errorf("gw: update ip group: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrIPGroupNotFound
	}
	return nil
}

// DeleteGroup 删除组（占用守卫事务化——2 根修：原 count→DELETE 两步
// check-then-act 有窗口，计数后并发把组挂上应用即产生悬空引用；改条件删除：
// 判定与删除同语句原子——应用组集引用中=0 行删除；规则随 FK CASCADE；
// 删除成功后引擎 Reload+广播）。
func (m *IPACManager) DeleteGroup(ctx context.Context, id string) error {
	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("gw: delete ip group tx: %w", err)
	}
	defer tx.Rollback(context.WithoutCancel(ctx)) //nolint:errcheck // 成功路径 Commit 覆盖
	tag, err := tx.Exec(ctx,
		`DELETE FROM gw_ip_groups g
		 WHERE g.id = $1
		   AND NOT EXISTS (SELECT 1 FROM gw_apps a WHERE $1 = ANY(a.ip_group_ids))`, id)
	if err != nil {
		return fmt.Errorf("gw: delete ip group: %w", err)
	}
	if tag.RowsAffected() == 0 {
		// 同事务快照分类（判定已原子完成；仅区分 4049 错误面）：
		// 组不存在或并发删除者先赢 → NotFound；存在且引用中 → InUse
		var exists, refs bool
		if err := tx.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM gw_ip_groups WHERE id = $1),
			        EXISTS(SELECT 1 FROM gw_apps WHERE $1 = ANY(ip_group_ids))`, id).
			Scan(&exists, &refs); err != nil {
			return fmt.Errorf("gw: classify ip group delete: %w", err)
		}
		if exists && refs {
			return domain.ErrIPGroupInUse
		}
		return domain.ErrIPGroupNotFound
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("gw: delete ip group commit: %w", err)
	}
	m.changed(ctx) // 组删=组内规则 CASCADE 清失——快照必须同步
	return nil
}

const ipRuleCols = `id, group_id, cidrs, action, priority, enabled, remark, created_at, updated_at`

func scanIPRule(row interface{ Scan(...any) error }) (*domain.IPRule, error) {
	var r domain.IPRule
	if err := row.Scan(&r.ID, &r.GroupID, &r.CIDRs, &r.Action, &r.Priority, &r.Enabled, &r.Remark, &r.CreatedAt, &r.UpdatedAt); err != nil {
		return nil, err
	}
	return &r, nil
}

// ListRulesByGroup 组内规则列表。
func (m *IPACManager) ListRulesByGroup(ctx context.Context, groupID string) ([]domain.IPRule, error) {
	rows, err := m.pool.Query(ctx, `SELECT `+ipRuleCols+` FROM gw_ip_rules WHERE group_id=$1 ORDER BY priority DESC, id ASC`, groupID)
	if err != nil {
		return nil, fmt.Errorf("gw: list ip rules: %w", err)
	}
	defer rows.Close()
	out := make([]domain.IPRule, 0, 16)
	for rows.Next() {
		r, err := scanIPRule(rows)
		if err != nil {
			return nil, fmt.Errorf("gw: scan ip rule: %w", err)
		}
		out = append(out, *r)
	}
	return out, rows.Err()
}

// CreateRule 创建规则（CIDR 引擎口径校验；非法条目/空列表 →
// ErrIPCIDRInvalid（5——原裸解析错误不在 writeGwError 映射内=500）。
func (m *IPACManager) CreateRule(ctx context.Context, r *domain.IPRule, cidrs string) error {
	if _, err := ValidateIPCIDRs(cidrs); err != nil {
		return fmt.Errorf("%w: %w", domain.ErrIPCIDRInvalid, err)
	}
	if !r.Action.Valid() {
		return domain.ErrIPActionInvalid
	}
	r.CIDRs = cidrs
	_, err := m.pool.Exec(ctx,
		`INSERT INTO gw_ip_rules (id, group_id, cidrs, action, priority, enabled, remark)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		r.ID, r.GroupID, r.CIDRs, int16(r.Action), r.Priority, r.Enabled, r.Remark)
	if err != nil {
		return fmt.Errorf("gw: create ip rule: %w", err)
	}
	m.changed(ctx)
	return nil
}

// DeleteRule 删除规则。
func (m *IPACManager) DeleteRule(ctx context.Context, id string) error {
	tag, err := m.pool.Exec(ctx, `DELETE FROM gw_ip_rules WHERE id=$1`, id)
	if err != nil {
		return fmt.Errorf("gw: delete ip rule: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrIPRuleNotFound
	}
	m.changed(ctx)
	return nil
}

// changed 规则/组变更后：引擎 Reload + Bus 广播回调（module 注入）。
func (m *IPACManager) changed(ctx context.Context) {
	if err := m.engine.Reload(ctx, m.pool); err != nil {
		// 本节点 reload 失败（如 DB 瞬断）仍广播——其他节点可各自成功 reload；
		// 跳过广播会让健康节点也停留在旧快照
		m.logger.Error("gw: ip rules reload failed (broadcast continues)", "err", err)
	}
	if m.onChanged != nil {
		m.onChanged(ctx)
	}
}
