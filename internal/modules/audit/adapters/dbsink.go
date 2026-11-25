// dbsink.go：operation_logs DB Sink（AGENTS §11 admin 操作日志入 DB，
// 查询/保留/导出）。批量走 pgx CopyFrom（二进制协议最快路径）；
// 整批失败返回错误由 Service 决策（重试/丢弃计数），Sink 无状态无重试。
package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/netyilei/netyadmin/internal/kernel/database"
	"github.com/netyilei/netyadmin/internal/kernel/security/primitives"
	"github.com/netyilei/netyadmin/internal/modules/audit/domain"
)

// DBSink 是 operation_logs 表的批量写入端点（pool/clock 构造期注入）。
type DBSink struct {
	pool  *database.Pool
	clock primitives.Clock // created_at 兜底补值（业务时间；nil=SystemClock）
}

// NewDBSink 创建（pool 非空——由模块装配在 AttachDB 后构造）。
func NewDBSink(pool *database.Pool) *DBSink { return &DBSink{pool: pool} }

// Name 实现 Sink。
func (s *DBSink) Name() string { return "db" }

// auditColumns 与 CopyFrom 列序一致（表结构 11100）。
var auditColumns = []string{
	"id", "type", "actor_type", "actor_id", "action", "resource", "result",
	"biz_code", "ip", "user_agent", "request_id", "trace_id", "node_id",
	"detail", "created_at",
}

// Flush 批量写入（CopyFrom 单事务整批提交；记录零值 CreatedAt 补 now UTC）。
func (s *DBSink) Flush(ctx context.Context, records []domain.Record) error {
	if len(records) == 0 {
		return nil
	}
	var now time.Time
	if s.clock != nil {
		now = s.clock.Now().UTC()
	} else {
		now = time.Now().UTC()
	}
	rows := make([][]any, 0, len(records))
	for _, r := range records {
		created := r.CreatedAt
		if created.IsZero() {
			created = now
		}
		detail, err := json.Marshal(orEmptyMap(r.Detail))
		if err != nil {
			return fmt.Errorf("audit: dbsink marshal detail %s: %w", r.ID, err)
		}
		rows = append(rows, []any{
			r.ID, int16(r.Type), r.ActorType, r.ActorID, r.Action, r.Resource, r.Result,
			r.BizCode, r.IP, r.UserAgent, r.RequestID, r.TraceID, r.NodeID,
			detail, created,
		})
	}
	_, err := s.pool.CopyFrom(ctx, pgx.Identifier{"operation_logs"}, auditColumns, pgx.CopyFromRows(rows))
	if err != nil {
		return fmt.Errorf("audit: dbsink copyfrom %d records: %w", len(records), err)
	}
	return nil
}

func orEmptyMap(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	return m
}
