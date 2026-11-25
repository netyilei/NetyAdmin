// query.go：审计查询契约——operation_logs 读面（列表/详情/导出）。
// 无删除面：logging.md §3「追加写、不可篡改」——留痕清理只走 RetentionJob
// （保留期按合规）；V1 的 log delete 端点与该红线冲突，V2 不做（注明）。
// 与写面（Record/Sink）分离：查询是同步读，写入是异步批量——两条独立通道。
// security 子面 = Type 过滤（Type=3），不另设表/端点族（§8.5 统一记录模型）。
package domain

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound 是查询无结果哨兵（adapters 转译 pgx.ErrNoRows；HTTP 10003）。
var ErrNotFound = errors.New("audit: record not found")

// QueryFilter 是审计列表过滤（零值字段=不过滤）。
type QueryFilter struct {
	Type      RecordType // 0=全部；3=security 子面
	Action    string     // 前缀匹配（LIKE 转义收口在 application）
	ActorID   string
	Result    string
	RequestID string
	TimeFrom  time.Time // 含
	TimeTo    time.Time // 含
	Offset    int
	Limit     int
}

// QueryRepo 是审计读面存储契约（application 定义、adapters 实现）。
type QueryRepo interface {
	// List 分页查询（按 created_at 倒序）。
	List(ctx context.Context, f QueryFilter) ([]Record, int64, error)
	// Get 详情（detail jsonb 反序列化进 Record.Detail）。
	Get(ctx context.Context, id string) (Record, error)
	// ExportList 导出拉取（同过滤条件、无分页；limit 为行数硬上限）。
	ExportList(ctx context.Context, f QueryFilter, limit int) ([]Record, error)
}
