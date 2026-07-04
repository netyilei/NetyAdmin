package log

import "time"

type LogType string

const (
	LogTypeOperation LogType = "operation"
	LogTypeError     LogType = "error"
	LogTypeOpen      LogType = "open"
	LogTypeTask      LogType = "task"
)

type LogPriority int

const (
	PriorityP0 LogPriority = iota
	PriorityP1
	PriorityP2
)

// LogEntry 是 LogBus 多态分发的统一日志接口。
// GetRequestID 用于跨 goroutine / 跨节点传播 request_id（Task 8.5）：
// 中间件（recovery / open_platform_auth）在构造 entry 时填入 RequestID，
// LogBus 刷盘时通过 GetRequestID() 取出写入 DB 列（admin_error_log.request_id /
// sys_open_platform_logs.request_id / sys_task_logs.request_id）。
// Operation 表 admin_operation_log 已有 request_id 列但 entity 暂未填充
// （未来 operation_log 写入方改造后再启用，本 spec 范围内不强制）。
type LogEntry interface {
	GetLogType() LogType
	GetCreatedAt() time.Time
	GetRequestID() string
}
