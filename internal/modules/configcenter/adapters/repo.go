// repo.go：configcenter 存储适配——sys_configs CRUD + config_history 追加/
// 读取/整表回写。SQL 一律参数化；group+key 唯一冲突映射业务哨兵。
package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/netyilei/netyadmin/internal/kernel/database"
	"github.com/netyilei/netyadmin/internal/kernel/security/primitives"
	"github.com/netyilei/netyadmin/internal/modules/configcenter/domain"
)

// Repo 实现 domain.Repo（pool 构造期注入）。
type Repo struct {
	pool *database.Pool
}

// NewRepo 创建。
func NewRepo(pool *database.Pool) *Repo { return &Repo{pool: pool} }

const cfgCols = `id, group_name, config_key, config_value, value_type, description,
	is_system, is_public, updated_by, created_at, updated_at`

func scanConfig(row pgx.Row) (domain.Config, error) {
	var c domain.Config
	var vt int16
	err := row.Scan(&c.ID, &c.GroupName, &c.ConfigKey, &c.ConfigValue, &vt,
		&c.Description, &c.IsSystem, &c.IsPublic, &c.UpdatedBy, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return domain.Config{}, err
	}
	c.ValueType = domain.ValueType(vt)
	return c, nil
}

// ListAll 全量配置（watcher 快照装载/管理面列表；LIMIT 上限+1 读面保护——
// 超限报错不截断，静默截断会让快照/回滚语义失真）。
func (r *Repo) ListAll(ctx context.Context) ([]domain.Config, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+cfgCols+` FROM sys_configs ORDER BY group_name, config_key LIMIT $1`,
		domain.MaxTotalKeys+1)
	if err != nil {
		return nil, fmt.Errorf("configcenter: list all: %w", err)
	}
	defer rows.Close()
	out, err := collectConfigs(rows)
	if err != nil {
		return nil, err
	}
	if len(out) > domain.MaxTotalKeys {
		return nil, fmt.Errorf("configcenter: list all: %w: %d keys > cap %d",
			domain.ErrTooManyKeys, len(out), domain.MaxTotalKeys)
	}
	return out, nil
}

// ListPublic 公开键集（部分索引 is_public）。
func (r *Repo) ListPublic(ctx context.Context) ([]domain.Config, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+cfgCols+` FROM sys_configs WHERE is_public ORDER BY group_name, config_key`)
	if err != nil {
		return nil, fmt.Errorf("configcenter: list public: %w", err)
	}
	defer rows.Close()
	return collectConfigs(rows)
}

// Get 单键（group+key 唯一定位）。
func (r *Repo) Get(ctx context.Context, group, key string) (domain.Config, error) {
	c, err := scanConfig(r.pool.QueryRow(ctx,
		`SELECT `+cfgCols+` FROM sys_configs WHERE group_name = $1 AND config_key = $2`, group, key))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Config{}, domain.ErrNotFound
	}
	return c, err
}

// ApplyChange 单事务变更（domain.Change → 新版本；G1-16 根修）：
// pg_advisory_xact_lock 串行化版本分配（多节点并发写同库——号段唯一约束兜底）
// → 变更前整表快照落 history（version=事务内 max+1）→ 写操作 → commit。
// 任一失败整体回滚零孤儿；系统键保护/键存在性在事务内判定（预检外的 TOCTOU 收口）。
func (r *Repo) ApplyChange(ctx context.Context, ch domain.Change) (int64, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("configcenter: change tx: %w", err)
	}
	defer tx.Rollback(context.WithoutCancel(ctx)) //nolint:errcheck // 成功路径 Commit 覆盖
	// 版本分配串行化（advisory xact lock——事务结束自动释放）
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('configcenter_change'))`); err != nil {
		return 0, fmt.Errorf("configcenter: change lock: %w", err)
	}
	var version int64
	if err := tx.QueryRow(ctx,
		`SELECT COALESCE(MAX(version), 0) + 1 FROM config_history`).Scan(&version); err != nil {
		return 0, fmt.Errorf("configcenter: next version: %w", err)
	}

	// 变更前整表快照落史
	all, err := listAllTx(ctx, tx)
	if err != nil {
		return 0, err
	}
	// 键量级上限预检（超限拒绝整体回滚零写入；rollback 的目标
	// 快照量级在 rollbackTx 读出后判定）
	if err := checkKeyBudget(all, ch); err != nil {
		return 0, err
	}
	snap := make([]domain.SnapshotItem, 0, len(all))
	for _, c := range all {
		snap = append(snap, domain.SnapshotItem{
			GroupName: c.GroupName, ConfigKey: c.ConfigKey, ConfigValue: c.ConfigValue,
			ValueType: int8(c.ValueType), Description: c.Description,
			IsSystem: c.IsSystem, IsPublic: c.IsPublic,
		})
	}
	snapJSON, err := json.Marshal(snap)
	if err != nil {
		return 0, fmt.Errorf("configcenter: marshal snapshot: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO config_history (id, version, snapshot, reason, created_by)
		VALUES ($1, $2, $3, $4, $5)`,
		primitives.NewULID(primitives.SystemClock{}), version, snapJSON, ch.Reason, ch.By); err != nil {
		return 0, fmt.Errorf("configcenter: append history v%d: %w", version, err)
	}

	// 写操作（事务内；业务冲突 → 整体回滚）
	switch ch.Kind {
	case domain.ChangeCreate, domain.ChangeUpdate:
		for _, c := range ch.Configs {
			if err := upsertTx(ctx, tx, c, ch.By); err != nil {
				return 0, err
			}
		}
	case domain.ChangeDelete:
		if err := deleteTx(ctx, tx, ch.Group, ch.Key); err != nil {
			return 0, err
		}
	case domain.ChangeRollback:
		if err := rollbackTx(ctx, tx, ch.RollbackTo, ch.By); err != nil {
			return 0, err
		}
	default:
		return 0, fmt.Errorf("configcenter: unknown change kind %d", ch.Kind)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("configcenter: change commit: %w", err)
	}
	return version, nil
}

// confKey 是 group+key 复合定位（map 判重键——结构体键避免拼接歧义）。
type confKey struct{ group, key string }

// checkKeyBudget 变更后键集量级上限判定（MaxTotalKeys）：
// create/update 按投影新增键计数——现存键（update 语义不增量）与批内重复
// 先去重；delete 只减不增免检；rollback 快照量级在 rollbackTx 判定。
func checkKeyBudget(current []domain.Config, ch domain.Change) error {
	if ch.Kind != domain.ChangeCreate && ch.Kind != domain.ChangeUpdate {
		return nil
	}
	existing := make(map[confKey]struct{}, len(current))
	for _, c := range current {
		existing[confKey{c.GroupName, c.ConfigKey}] = struct{}{}
	}
	added := 0
	seen := make(map[confKey]struct{}, len(ch.Configs))
	for _, c := range ch.Configs {
		k := confKey{c.GroupName, c.ConfigKey}
		if _, dup := seen[k]; dup {
			continue
		}
		seen[k] = struct{}{}
		if _, ok := existing[k]; ok {
			continue
		}
		added++
	}
	if len(current)+added > domain.MaxTotalKeys {
		return fmt.Errorf("%w: %d keys + %d new > cap %d",
			domain.ErrTooManyKeys, len(current), added, domain.MaxTotalKeys)
	}
	return nil
}

// upsertTx 事务内单键 upsert（存在→Update 值/描述/公开位；不存在→Insert 全量；
// 保留组命中拒绝——AGENTS §4 不热更面）。
func upsertTx(ctx context.Context, tx pgx.Tx, c domain.Config, by string) error {
	if domain.ReservedGroups[c.GroupName] {
		return fmt.Errorf("%w: group %q reserved (AGENTS §4 不热更面)", domain.ErrReservedGroup, c.GroupName)
	}
	if err := c.Validate(); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx,
		`UPDATE sys_configs SET config_value = $1, value_type = $2, description = $3,
		is_public = $4, updated_by = $5, updated_at = now()
		WHERE group_name = $6 AND config_key = $7`,
		c.ConfigValue, int16(c.ValueType), c.Description, c.IsPublic, by,
		c.GroupName, c.ConfigKey)
	if err != nil {
		return fmt.Errorf("configcenter: upsert update %s.%s: %w", c.GroupName, c.ConfigKey, err)
	}
	if tag.RowsAffected() > 0 {
		return nil
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO sys_configs (id, group_name, config_key, config_value, value_type,
			description, is_system, is_public, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		primitives.NewULID(primitives.SystemClock{}), c.GroupName, c.ConfigKey,
		c.ConfigValue, int16(c.ValueType), c.Description, c.IsSystem, c.IsPublic, by); err != nil {
		return fmt.Errorf("configcenter: upsert insert %s.%s: %w", c.GroupName, c.ConfigKey, err)
	}
	return nil
}

// deleteTx 事务内单键删除（不存在/系统保护判定）。
func deleteTx(ctx context.Context, tx pgx.Tx, group, key string) error {
	if domain.ReservedGroups[group] {
		return fmt.Errorf("%w: group %q reserved", domain.ErrReservedGroup, group)
	}
	var isSystem bool
	err := tx.QueryRow(ctx,
		`SELECT is_system FROM sys_configs WHERE group_name = $1 AND config_key = $2`,
		group, key).Scan(&isSystem)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("configcenter: delete lookup %s.%s: %w", group, key, err)
	}
	if isSystem {
		return domain.ErrSystemProtected
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM sys_configs WHERE group_name = $1 AND config_key = $2`, group, key); err != nil {
		return fmt.Errorf("configcenter: delete %s.%s: %w", group, key, err)
	}
	return nil
}

// rollbackTx 事务内整表写回历史快照（快照含 description/is_system——回滚恢复
// 原描述与系统位；「回滚可清空系统键」语义显式：历史快照为准，评审 P2）。
func rollbackTx(ctx context.Context, tx pgx.Tx, version int64, by string) error {
	var snapJSON []byte
	err := tx.QueryRow(ctx,
		`SELECT snapshot FROM config_history WHERE version = $1`, version).Scan(&snapJSON)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("configcenter: rollback read v%d: %w", version, err)
	}
	var snap []domain.SnapshotItem
	if err := json.Unmarshal(snapJSON, &snap); err != nil {
		return fmt.Errorf("configcenter: rollback decode v%d: %w", version, err)
	}
	if len(snap) > domain.MaxTotalKeys {
		return fmt.Errorf("configcenter: rollback v%d: %w: %d keys > cap %d",
			version, domain.ErrTooManyKeys, len(snap), domain.MaxTotalKeys)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM sys_configs`); err != nil {
		return fmt.Errorf("configcenter: rollback clear: %w", err)
	}
	for _, it := range snap {
		if _, err := tx.Exec(ctx,
			`INSERT INTO sys_configs (id, group_name, config_key, config_value, value_type,
				description, is_system, is_public, updated_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			primitives.NewULID(primitives.SystemClock{}), it.GroupName, it.ConfigKey,
			it.ConfigValue, it.ValueType, it.Description, it.IsSystem, it.IsPublic, by); err != nil {
			return fmt.Errorf("configcenter: rollback insert %s.%s: %w", it.GroupName, it.ConfigKey, err)
		}
	}
	return nil
}

// listAllTx 事务内全量快照读（与 repo.ListAll 同列集）。
func listAllTx(ctx context.Context, tx pgx.Tx) ([]domain.Config, error) {
	rows, err := tx.Query(ctx,
		`SELECT `+cfgCols+` FROM sys_configs ORDER BY group_name, config_key`)
	if err != nil {
		return nil, fmt.Errorf("configcenter: tx list all: %w", err)
	}
	defer rows.Close()
	out := make([]domain.Config, 0, 32)
	for rows.Next() {
		c, err := scanConfig(rows)
		if err != nil {
			return nil, fmt.Errorf("configcenter: tx scan config: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// CurrentVersion 当前全局版本（无历史=0）。
func (r *Repo) CurrentVersion(ctx context.Context) (int64, error) {
	var v int64
	err := r.pool.QueryRow(ctx,
		`SELECT COALESCE(MAX(version), 0) FROM config_history`).Scan(&v)
	if err != nil {
		return 0, fmt.Errorf("configcenter: current version: %w", err)
	}
	return v, nil
}

// AppendHistory 落变更前快照（version 由 application 分配；UNIQUE 冲突=并发写
// 竞态——单写面（管理端点）下不应发生，发生即报错重试）。
func (r *Repo) AppendHistory(ctx context.Context, h domain.HistoryEntry) error {
	snap, err := json.Marshal(h.Snapshot)
	if err != nil {
		return fmt.Errorf("configcenter: marshal snapshot: %w", err)
	}
	_, err = r.pool.Exec(ctx,
		`INSERT INTO config_history (id, version, snapshot, reason, created_by)
		VALUES ($1, $2, $3, $4, $5)`,
		h.ID, h.Version, snap, h.Reason, h.CreatedBy)
	if err != nil {
		return fmt.Errorf("configcenter: append history v%d: %w", h.Version, err)
	}
	return nil
}

// GetHistory 按版本取历史（回滚读源）。
func (r *Repo) GetHistory(ctx context.Context, version int64) (domain.HistoryEntry, error) {
	var h domain.HistoryEntry
	var snap []byte
	err := r.pool.QueryRow(ctx,
		`SELECT id, version, snapshot, reason, created_by, created_at
		FROM config_history WHERE version = $1`, version).
		Scan(&h.ID, &h.Version, &snap, &h.Reason, &h.CreatedBy, &h.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.HistoryEntry{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.HistoryEntry{}, fmt.Errorf("configcenter: get history v%d: %w", version, err)
	}
	if err := json.Unmarshal(snap, &h.Snapshot); err != nil {
		return domain.HistoryEntry{}, fmt.Errorf("configcenter: decode snapshot v%d: %w", version, err)
	}
	return h, nil
}

// —— 内部 ——

// collectConfigs 行集扫描统一出口。
func collectConfigs(rows pgx.Rows) ([]domain.Config, error) {
	out := make([]domain.Config, 0, 32)
	for rows.Next() {
		c, err := scanConfig(rows)
		if err != nil {
			return nil, fmt.Errorf("configcenter: scan config: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ListHistory 输出最近变更历史（版本升序—— 回滚抽屉消费）。
func (r *Repo) ListHistory(ctx context.Context, limit int) ([]domain.HistoryEntry, error) {
	rows, err := r.pool.Query(ctx, `SELECT version, reason, created_by, created_at
		FROM config_history ORDER BY version DESC LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("configcenter: list history: %w", err)
	}
	defer rows.Close()
	out := make([]domain.HistoryEntry, 0, limit)
	for rows.Next() {
		var h domain.HistoryEntry
		if err := rows.Scan(&h.Version, &h.Reason, &h.CreatedBy, &h.CreatedAt); err != nil {
			return nil, fmt.Errorf("configcenter: list history scan: %w", err)
		}
		out = append(out, h)
	}
	return out, rows.Err()
}
