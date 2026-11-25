// query.go：审计查询编排——过滤转义收口（application 层单点，
// adapters 不再二次转义）+ 导出行数上限。无删除面：logging.md §3「追加写、
// 不可篡改」——留痕清理只走 RetentionJob（V1 delete 端点与红线冲突不移植）。
package application

import (
	"context"
	"errors"

	"github.com/netyilei/netyadmin/internal/kernel/security/primitives"
	"github.com/netyilei/netyadmin/internal/modules/audit/domain"
)

// exportMaxRows 是单次导出上限（内存拉取面——万行级足够管理面审计导出；
// 超量走过滤缩小范围，不分页流式是登记面增强项）。
const exportMaxRows = 10000

// ErrExportTooMany 是导出超上限错误（调用方缩小过滤范围）。
var ErrExportTooMany = errors.New("audit: export exceeds row cap")

// QueryService 是审计查询编排入口（AttachDB 构造）。
type QueryService struct {
	repo domain.QueryRepo
}

// NewQueryService 创建。
func NewQueryService(repo domain.QueryRepo) *QueryService {
	return &QueryService{repo: repo}
}

// List 分页查询（LIKE 转义收口）。
func (q *QueryService) List(ctx context.Context, f domain.QueryFilter) ([]domain.Record, int64, error) {
	f.Action = primitives.EscapeLike(f.Action)
	return q.repo.List(ctx, f)
}

// Get 详情。
func (q *QueryService) Get(ctx context.Context, id string) (domain.Record, error) {
	return q.repo.Get(ctx, id)
}

// Export 导出拉取（超上限拒绝——调用方缩小过滤范围）。
func (q *QueryService) Export(ctx context.Context, f domain.QueryFilter) ([]domain.Record, error) {
	f.Action = primitives.EscapeLike(f.Action)
	// limit=上限+1：多拉一行判定超限，避免全量进内存后再拒。
	rows, err := q.repo.ExportList(ctx, f, exportMaxRows+1)
	if err != nil {
		return nil, err
	}
	if len(rows) > exportMaxRows {
		return nil, ErrExportTooMany
	}
	return rows, nil
}
