// repo.go：gw 治理存储适配——apps/scopes/apis/scope_apis CRUD +
// edge 判定快照读。SQL 一律参数化；quota jsonb 序列化在 repo 层。
package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/netyilei/netyadmin/internal/kernel/database"
	"github.com/netyilei/netyadmin/internal/modules/gw/domain"
)

// Repo 实现 domain.Repo（pool 构造期注入）。
type Repo struct {
	pool *database.Pool
}

// NewRepo 创建。
func NewRepo(pool *database.Pool) *Repo { return &Repo{pool: pool} }

const appCols = `id, app_key, app_secret_enc, name, status,
	rate_limit_enabled, quota_config, remark, storage_ref, ip_group_ids, created_at, updated_at`

// scanApp 行扫描（quota jsonb 反序列化）。
func scanApp(row pgx.Row) (domain.App, error) {
	var a domain.App
	var status int16
	var quotaRaw []byte
	if err := row.Scan(&a.ID, &a.AppKey, &a.AppSecretEnc, &a.Name, &status,
		&a.RateLimitEnabled, &quotaRaw, &a.Remark, &a.StorageRef,
		&a.IPGroupIDs, &a.CreatedAt, &a.UpdatedAt); err != nil {
		return domain.App{}, err
	}
	a.Status = domain.Status(status)
	if len(quotaRaw) > 0 {
		if err := json.Unmarshal(quotaRaw, &a.Quota); err != nil {
			return domain.App{}, fmt.Errorf("gw: decode quota %s: %w", a.ID, err)
		}
	}
	return a, nil
}

// ListApps 分页列表。
func (r *Repo) ListApps(ctx context.Context, offset, limit int) ([]domain.App, int64, error) {
	var total int64
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM gw_apps`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("gw: count apps: %w", err)
	}
	rows, err := r.pool.Query(ctx,
		`SELECT `+appCols+` FROM gw_apps ORDER BY created_at DESC, id LIMIT $1 OFFSET $2`,
		limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("gw: list apps: %w", err)
	}
	defer rows.Close()
	out := make([]domain.App, 0, limit)
	for rows.Next() {
		a, err := scanApp(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("gw: scan app: %w", err)
		}
		out = append(out, a)
	}
	return out, total, rows.Err()
}

// ListAllForSnapshot 全量应用行（edge 判定快照装载面——含禁用行；行扫描与
// ListApps 共享 scanApp，J-1：快照装载不再另维护一套 cols/scan）。
func (r *Repo) ListAllForSnapshot(ctx context.Context) ([]domain.App, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+appCols+` FROM gw_apps`)
	if err != nil {
		return nil, fmt.Errorf("gw: list all apps: %w", err)
	}
	defer rows.Close()
	out := make([]domain.App, 0, 16)
	for rows.Next() {
		a, err := scanApp(rows)
		if err != nil {
			return nil, fmt.Errorf("gw: scan app: %w", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("gw: iterate apps: %w", err)
	}
	return out, nil
}

// GetApp 按 ID。
func (r *Repo) GetApp(ctx context.Context, id string) (domain.App, error) {
	a, err := scanApp(r.pool.QueryRow(ctx,
		`SELECT `+appCols+` FROM gw_apps WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.App{}, domain.ErrNotFound
	}
	return a, err
}

// GetAppByKey 按 AppKey（edge 热路径入参）。
func (r *Repo) GetAppByKey(ctx context.Context, appKey string) (domain.App, error) {
	a, err := scanApp(r.pool.QueryRow(ctx,
		`SELECT `+appCols+` FROM gw_apps WHERE app_key = $1`, appKey))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.App{}, domain.ErrNotFound
	}
	return a, err
}

// InsertAppWithScopes 创建 App+初始 scopes 单事务（安-F7 根修：原两步写
// 非原子——scopes 写失败遗留无授权 App）。app_key 唯一约束冲突 →
// ErrAppKeyTaken（service 预检的并发兜底——adminapi 映射 409）。
func (r *Repo) InsertAppWithScopes(ctx context.Context, a domain.App, scopes []string) error {
	if a.IPGroupIDs == nil {
		a.IPGroupIDs = []string{} // NOT NULL 列归一（pgx nil 切片→NULL）
	}
	quota, err := json.Marshal(a.Quota)
	if err != nil {
		return fmt.Errorf("gw: encode quota: %w", err)
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("gw: insert app tx: %w", err)
	}
	defer tx.Rollback(context.WithoutCancel(ctx)) //nolint:errcheck // 成功路径 Commit 覆盖
	if _, err := tx.Exec(ctx,
		`INSERT INTO gw_apps (id, app_key, app_secret_enc, name, status,
			rate_limit_enabled, quota_config, remark, storage_ref, ip_group_ids)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		a.ID, a.AppKey, a.AppSecretEnc, a.Name, int16(a.Status),
		a.RateLimitEnabled, quota, a.Remark, a.StorageRef, a.IPGroupIDs); err != nil {
		if database.IsUniqueViolation(err) {
			return domain.ErrAppKeyTaken
		}
		return fmt.Errorf("gw: insert app: %w", err)
	}
	for _, s := range scopes {
		if _, err := tx.Exec(ctx,
			`INSERT INTO gw_app_scopes (app_id, scope) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			a.ID, s); err != nil {
			return fmt.Errorf("gw: insert app scope: %w", err)
		}
	}
	return tx.Commit(ctx)
}

// sqlAppUpdate 是应用基础字段 UPDATE 唯一语句（UpdateApp/UpdateAppWithScopes
// 共用——status/secret 不在此面（各走独立端点）；G1-06 双处漂移消除）。
const sqlAppUpdate = `UPDATE gw_apps SET name = $1, rate_limit_enabled = $2, quota_config = $3,
	remark = $4, storage_ref = $5, ip_group_ids = $6, updated_at = now() WHERE id = $7`

// UpdateApp 基础字段+IP 组集更新（secret 走独立面；status 走独立面）。
func (r *Repo) UpdateApp(ctx context.Context, a domain.App) error {
	if a.IPGroupIDs == nil {
		a.IPGroupIDs = []string{} // NOT NULL 列归一（pgx nil 切片→NULL）
	}
	quota, err := json.Marshal(a.Quota)
	if err != nil {
		return fmt.Errorf("gw: encode quota: %w", err)
	}
	tag, err := r.pool.Exec(ctx, sqlAppUpdate,
		a.Name, a.RateLimitEnabled, quota, a.Remark, a.StorageRef, a.IPGroupIDs, a.ID)
	if err != nil {
		return fmt.Errorf("gw: update app: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// UpdateAppWithScopes 基础字段+scopes 全量替换单事务（应用表单单一保存入口
// ——两步写非原子：scopes 失败遗留半更新状态；对照 InsertAppWithScopes 惯例）。
func (r *Repo) UpdateAppWithScopes(ctx context.Context, a domain.App, scopes []string) error {
	if a.IPGroupIDs == nil {
		a.IPGroupIDs = []string{}
	}
	quota, err := json.Marshal(a.Quota)
	if err != nil {
		return fmt.Errorf("gw: encode quota: %w", err)
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("gw: update app tx: %w", err)
	}
	defer tx.Rollback(context.WithoutCancel(ctx)) //nolint:errcheck // 成功路径 Commit 覆盖
	tag, err := tx.Exec(ctx, sqlAppUpdate,
		a.Name, a.RateLimitEnabled, quota, a.Remark, a.StorageRef, a.IPGroupIDs, a.ID)
	if err != nil {
		return fmt.Errorf("gw: update app: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	if _, err := tx.Exec(ctx, `DELETE FROM gw_app_scopes WHERE app_id = $1`, a.ID); err != nil {
		return fmt.Errorf("gw: update app scopes clear: %w", err)
	}
	for _, s := range scopes {
		if _, err := tx.Exec(ctx,
			`INSERT INTO gw_app_scopes (app_id, scope) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			a.ID, s); err != nil {
			return fmt.Errorf("gw: update app scope: %w", err)
		}
	}
	return tx.Commit(ctx)
}

// UpdateAppStatus 状态位独立写（禁用/启用面——与 UpdateApp 分离）。
func (r *Repo) UpdateAppStatus(ctx context.Context, id string, status domain.Status) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE gw_apps SET status = $1, updated_at = now() WHERE id = $2`, int16(status), id)
	if err != nil {
		return fmt.Errorf("gw: update app status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// UpdateAppSecretEnc 重置 secret（密文替换）。
func (r *Repo) UpdateAppSecretEnc(ctx context.Context, id, secretEnc string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE gw_apps SET app_secret_enc = $1, updated_at = now() WHERE id = $2`, secretEnc, id)
	if err != nil {
		return fmt.Errorf("gw: reset secret: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// DeleteApp 删除（app_scopes 级联清理）。
func (r *Repo) DeleteApp(ctx context.Context, id string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("gw: delete app tx: %w", err)
	}
	defer tx.Rollback(context.WithoutCancel(ctx)) //nolint:errcheck // 成功路径 Commit 覆盖
	if _, err := tx.Exec(ctx, `DELETE FROM gw_app_scopes WHERE app_id = $1`, id); err != nil {
		return fmt.Errorf("gw: delete app scopes: %w", err)
	}
	tag, err := tx.Exec(ctx, `DELETE FROM gw_apps WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("gw: delete app: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return tx.Commit(ctx)
}

// AppScopes 聚合读。
func (r *Repo) AppScopes(ctx context.Context, appID string) ([]string, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT scope FROM gw_app_scopes WHERE app_id = $1 ORDER BY scope`, appID)
	if err != nil {
		return nil, fmt.Errorf("gw: app scopes: %w", err)
	}
	defer rows.Close()
	out := make([]string, 0, 8)
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, fmt.Errorf("gw: scan app scope: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ListScopeGroups 全量。
func (r *Repo) ListScopeGroups(ctx context.Context) ([]domain.ScopeGroup, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, code, name, description, status, created_at, updated_at
		FROM gw_scope_groups ORDER BY code`)
	if err != nil {
		return nil, fmt.Errorf("gw: list scope groups: %w", err)
	}
	defer rows.Close()
	out := make([]domain.ScopeGroup, 0, 16)
	for rows.Next() {
		var v domain.ScopeGroup
		var status int16
		if err := rows.Scan(&v.ID, &v.Code, &v.Name, &v.Description, &status, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, fmt.Errorf("gw: scan scope group: %w", err)
		}
		v.Status = domain.Status(status)
		out = append(out, v)
	}
	return out, rows.Err()
}

// GetScopeGroup 按 code（scope 合法性判定）。
func (r *Repo) GetScopeGroup(ctx context.Context, code string) (domain.ScopeGroup, error) {
	var v domain.ScopeGroup
	var status int16
	err := r.pool.QueryRow(ctx,
		`SELECT id, code, name, description, status, created_at, updated_at
		FROM gw_scope_groups WHERE code = $1`, code).
		Scan(&v.ID, &v.Code, &v.Name, &v.Description, &status, &v.CreatedAt, &v.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ScopeGroup{}, domain.ErrScopeNotFound
	}
	if err != nil {
		return domain.ScopeGroup{}, fmt.Errorf("gw: get scope group: %w", err)
	}
	v.Status = domain.Status(status)
	return v, nil
}

// GetScopeGroupByID 按 ID（管理面 RESTful 主键寻址）。
func (r *Repo) GetScopeGroupByID(ctx context.Context, id string) (domain.ScopeGroup, error) {
	var v domain.ScopeGroup
	var status int16
	err := r.pool.QueryRow(ctx,
		`SELECT id, code, name, description, status, created_at, updated_at
		FROM gw_scope_groups WHERE id = $1`, id).
		Scan(&v.ID, &v.Code, &v.Name, &v.Description, &status, &v.CreatedAt, &v.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ScopeGroup{}, domain.ErrScopeNotFound
	}
	if err != nil {
		return domain.ScopeGroup{}, fmt.Errorf("gw: get scope group by id: %w", err)
	}
	v.Status = domain.Status(status)
	return v, nil
}

// InsertScopeGroup 创建（code 唯一约束冲突 → ErrScopeCodeTaken）。
func (r *Repo) InsertScopeGroup(ctx context.Context, g domain.ScopeGroup) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO gw_scope_groups (id, code, name, description, status)
		VALUES ($1, $2, $3, $4, $5)`,
		g.ID, g.Code, g.Name, g.Description, int16(g.Status))
	if database.IsUniqueViolation(err) {
		return domain.ErrScopeCodeTaken
	}
	if err != nil {
		return fmt.Errorf("gw: insert scope group: %w", err)
	}
	return nil
}

// UpdateScopeGroup 更新（code 不可变——授权引用按 code）。
func (r *Repo) UpdateScopeGroup(ctx context.Context, g domain.ScopeGroup) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE gw_scope_groups SET name = $2, description = $3, status = $4, updated_at = now() WHERE id = $1`,
		g.ID, g.Name, g.Description, int16(g.Status))
	if err != nil {
		return fmt.Errorf("gw: update scope group: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrScopeNotFound
	}
	return nil
}

// DeleteScopeGroup 删除（占用守卫事务化——2 根修：原守卫在 service 层
// count→DELETE 两步 check-then-act 有窗口；改条件删除，判定与删除同语句原子——
// gw_app_scopes 按 code / gw_scope_apis 按 id 任一引用=0 行删除。
// ErrScopeNotFound（组不存在或并发删除者先赢）/ ErrScopeGroupInUse（引用中））。
func (r *Repo) DeleteScopeGroup(ctx context.Context, id string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("gw: delete scope group tx: %w", err)
	}
	defer tx.Rollback(context.WithoutCancel(ctx)) //nolint:errcheck // 成功路径 Commit 覆盖
	tag, err := tx.Exec(ctx,
		`DELETE FROM gw_scope_groups g
		 WHERE g.id = $1
		   AND NOT EXISTS (SELECT 1 FROM gw_app_scopes s WHERE s.scope = g.code)
		   AND NOT EXISTS (SELECT 1 FROM gw_scope_apis a WHERE a.scope_id = g.id)`, id)
	if err != nil {
		return fmt.Errorf("gw: delete scope group: %w", err)
	}
	if tag.RowsAffected() == 0 {
		// 同事务快照分类（判定已原子完成；仅区分 4049 错误面）
		var exists, appRefs, apiRefs bool
		if err := tx.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM gw_scope_groups WHERE id = $1),
			        EXISTS(SELECT 1 FROM gw_app_scopes
			               WHERE scope = (SELECT code FROM gw_scope_groups WHERE id = $1)),
			        EXISTS(SELECT 1 FROM gw_scope_apis WHERE scope_id = $1)`, id).
			Scan(&exists, &appRefs, &apiRefs); err != nil {
			return fmt.Errorf("gw: classify scope group delete: %w", err)
		}
		if exists && (appRefs || apiRefs) {
			return domain.ErrScopeGroupInUse
		}
		return domain.ErrScopeNotFound
	}
	return tx.Commit(ctx)
}

// ListApis 全量（grouped 收敛为平列表+grp 字段——前端分组）。
func (r *Repo) ListApis(ctx context.Context) ([]domain.Api, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, method, path, name, grp, description, status, created_at, updated_at
		FROM gw_apis ORDER BY grp, path`)
	if err != nil {
		return nil, fmt.Errorf("gw: list apis: %w", err)
	}
	defer rows.Close()
	out := make([]domain.Api, 0, 64)
	for rows.Next() {
		var v domain.Api
		var status int16
		if err := rows.Scan(&v.ID, &v.Method, &v.Path, &v.Name, &v.Grp, &v.Description, &status, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, fmt.Errorf("gw: scan api: %w", err)
		}
		v.Status = domain.Status(status)
		out = append(out, v)
	}
	return out, rows.Err()
}

// GetApi 按 ID。
func (r *Repo) GetApi(ctx context.Context, id string) (domain.Api, error) {
	var v domain.Api
	var status int16
	err := r.pool.QueryRow(ctx,
		`SELECT id, method, path, name, grp, description, status, created_at, updated_at
		FROM gw_apis WHERE id = $1`, id).
		Scan(&v.ID, &v.Method, &v.Path, &v.Name, &v.Grp, &v.Description, &status, &v.CreatedAt, &v.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Api{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Api{}, fmt.Errorf("gw: get api: %w", err)
	}
	v.Status = domain.Status(status)
	return v, nil
}

// InsertApi 创建。
func (r *Repo) InsertApi(ctx context.Context, v domain.Api) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO gw_apis (id, method, path, name, grp, description, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		v.ID, v.Method, v.Path, v.Name, v.Grp, v.Description, int16(v.Status))
	if err != nil {
		return fmt.Errorf("gw: insert api: %w", err)
	}
	return nil
}

// UpdateApi 更新。
func (r *Repo) UpdateApi(ctx context.Context, v domain.Api) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE gw_apis SET method = $1, path = $2, name = $3, grp = $4, description = $5,
		status = $6, updated_at = now() WHERE id = $7`,
		v.Method, v.Path, v.Name, v.Grp, v.Description, int16(v.Status), v.ID)
	if err != nil {
		return fmt.Errorf("gw: update api: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// DeleteApi 删除（scope_apis 级联清理）。
func (r *Repo) DeleteApi(ctx context.Context, id string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("gw: delete api tx: %w", err)
	}
	defer tx.Rollback(context.WithoutCancel(ctx)) //nolint:errcheck // 成功路径 Commit 覆盖
	if _, err := tx.Exec(ctx, `DELETE FROM gw_scope_apis WHERE api_id = $1`, id); err != nil {
		return fmt.Errorf("gw: delete api scopes: %w", err)
	}
	tag, err := tx.Exec(ctx, `DELETE FROM gw_apis WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("gw: delete api: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return tx.Commit(ctx)
}

// ScopeApiIDs 读 scope 白名单。
func (r *Repo) ScopeApiIDs(ctx context.Context, scopeID string) ([]string, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT api_id FROM gw_scope_apis WHERE scope_id = $1 ORDER BY api_id`, scopeID)
	if err != nil {
		return nil, fmt.Errorf("gw: scope api ids: %w", err)
	}
	defer rows.Close()
	out := make([]string, 0, 16)
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, fmt.Errorf("gw: scan scope api: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// SetScopeApis 全量替换（scope→API 白名单；单事务）。
func (r *Repo) SetScopeApis(ctx context.Context, scopeID string, apiIDs []string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("gw: set scope apis tx: %w", err)
	}
	defer tx.Rollback(context.WithoutCancel(ctx)) //nolint:errcheck // 成功路径 Commit 覆盖
	if _, err := tx.Exec(ctx, `DELETE FROM gw_scope_apis WHERE scope_id = $1`, scopeID); err != nil {
		return fmt.Errorf("gw: set scope apis clear: %w", err)
	}
	for _, id := range apiIDs {
		if _, err := tx.Exec(ctx,
			`INSERT INTO gw_scope_apis (scope_id, api_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			scopeID, id); err != nil {
			return fmt.Errorf("gw: set scope apis insert: %w", err)
		}
	}
	return tx.Commit(ctx)
}
