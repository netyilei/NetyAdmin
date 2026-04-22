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

type LogEntry interface {
	GetLogType() LogType
	GetCreatedAt() time.Time
}
