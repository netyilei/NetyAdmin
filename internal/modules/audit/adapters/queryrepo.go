// queryrepo.go：operation_logs 读面存储——过滤分页列表/详情/删除/
// 导出拉取。SQL 一律参数化；detail jsonb 反序列化为结构化 map。
package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/jackc/pgx/v5"

	"github.com/netyilei/netyadmin/internal/kernel/database"
	"github.com/netyilei/netyadmin/internal/modules/audit/domain"
)

// QueryRepo 实现 domain.QueryRepo（pool 构造期注入）。
type QueryRepo struct {
	pool *database.Pool
}

// NewQueryRepo 创建。
func NewQueryRepo(pool *database.Pool) *QueryRepo { return &QueryRepo{pool: pool} }

// queryCols 与 operation_logs 表（迁移 11100）对齐。
const queryCols = `id, type, actor_type, actor_id, action, resource, result,
	biz_code, ip, user_agent, request_id, trace_id, node_id, detail, created_at`

// scanRecord 行扫描（detail jsonb → map）。
func scanRecord(row pgx.Row) (domain.Record, error) {
	var r domain.Record
	var typ int16
	var detail []byte
	if err := row.Scan(&r.ID, &typ, &r.ActorType, &r.ActorID, &r.Action, &r.Resource,
		&r.Result, &r.BizCode, &r.IP, &r.UserAgent, &r.RequestID, &r.TraceID,
		&r.NodeID, &detail, &r.CreatedAt); err != nil {
		return domain.Record{}, err
	}
	r.Type = domain.RecordType(typ)
	if len(detail) > 0 {
		if err := json.Unmarshal(detail, &r.Detail); err != nil {
			return domain.Record{}, fmt.Errorf("audit: query decode detail %s: %w", r.ID, err)
		}
	}
	return r, nil
}

// buildWhere 过滤子句组装（条件增量追加；LIKE 输入已由 application 层转义）。
func buildWhere(f domain.QueryFilter) (string, []any) {
	where := ` WHERE 1=1`
	var args []any
	if f.Type != 0 {
		args = append(args, int16(f.Type))
		where += fmt.Sprintf(` AND type = $%d`, len(args))
	}
	if f.Action != "" {
		// LIKE 转义单点在 application 层——此处直接使用（G1-05；双重转义为评审修复项）
		args = append(args, f.Action+"%")
		where += fmt.Sprintf(` AND action LIKE $%d`, len(args))
	}
	if f.ActorID != "" {
		args = append(args, f.ActorID)
		where += fmt.Sprintf(` AND actor_id = $%d`, len(args))
	}
	if f.Result != "" {
		args = append(args, f.Result)
		where += fmt.Sprintf(` AND result = $%d`, len(args))
	}
	if f.RequestID != "" {
		args = append(args, f.RequestID)
		where += fmt.Sprintf(` AND request_id = $%d`, len(args))
	}
	if !f.TimeFrom.IsZero() {
		args = append(args, f.TimeFrom)
		where += fmt.Sprintf(` AND created_at >= $%d`, len(args))
	}
	if !f.TimeTo.IsZero() {
		args = append(args, f.TimeTo)
		where += fmt.Sprintf(` AND created_at <= $%d`, len(args))
	}
	return where, args
}

// List 分页列表（created_at 倒序）。
func (r *QueryRepo) List(ctx context.Context, f domain.QueryFilter) ([]domain.Record, int64, error) {
	where, args := buildWhere(f)
	var total int64
	if err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM operation_logs`+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("audit: count logs: %w", err)
	}
	q := `SELECT ` + queryCols + ` FROM operation_logs` + where +
		` ORDER BY created_at DESC, id LIMIT $` + strconv.Itoa(len(args)+1) +
		` OFFSET $` + strconv.Itoa(len(args)+2)
	rows, err := r.pool.Query(ctx, q, append(args, f.Limit, f.Offset)...)
	if err != nil {
		return nil, 0, fmt.Errorf("audit: list logs: %w", err)
	}
	defer rows.Close()
	out := make([]domain.Record, 0, f.Limit)
	for rows.Next() {
		rec, err := scanRecord(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("audit: scan log: %w", err)
		}
		out = append(out, rec)
	}
	return out, total, rows.Err()
}

// Get 详情（不存在 → domain.ErrNotFound，哨兵定义在 domain/query.go）。
func (r *QueryRepo) Get(ctx context.Context, id string) (domain.Record, error) {
	rec, err := scanRecord(r.pool.QueryRow(ctx,
		`SELECT `+queryCols+` FROM operation_logs WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Record{}, domain.ErrNotFound
	}
	return rec, err
}

// ExportList 导出拉取（同过滤、无分页；limit 为行数硬上限）。
func (r *QueryRepo) ExportList(ctx context.Context, f domain.QueryFilter, limit int) ([]domain.Record, error) {
	where, args := buildWhere(f)
	q := `SELECT ` + queryCols + ` FROM operation_logs` + where +
		` ORDER BY created_at ASC, id LIMIT $` + strconv.Itoa(len(args)+1)
	rows, err := r.pool.Query(ctx, q, append(args, limit)...)
	if err != nil {
		return nil, fmt.Errorf("audit: export logs: %w", err)
	}
	defer rows.Close()
	out := make([]domain.Record, 0, 256)
	for rows.Next() {
		rec, err := scanRecord(rows)
		if err != nil {
			return nil, fmt.Errorf("audit: scan export: %w", err)
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

// ErrNotFound 是查询无结果哨兵（转译自 pgx.ErrNoRows；HTTP 10003）。
// 哨兵定义在 domain（query.go）——本文件 Get 路径转译。
