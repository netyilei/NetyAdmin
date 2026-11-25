// sink.go：审计 Sink 契约（DB/文件/SIEM/OTLP 可插拔）。
// 记录只产生一次，由 Audit 统一存储/查询；Observability 不重复落审计表。
package domain

import "context"

// Sink 是审计记录的批量写入端点（异步批量由 Service 编排；Sink 只管批量落盘）。
type Sink interface {
	// Name 是 sink 标识（诊断/日志用）。
	Name() string
	// Flush 批量写入（实现方必须整体幂等——重试由 Service 决定，失败返回错误；
	// 记录 CreatedAt 为零值时实现方自行补 now UTC）。
	Flush(ctx context.Context, records []Record) error
}
