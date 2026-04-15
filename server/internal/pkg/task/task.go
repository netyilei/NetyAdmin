package task

import (
	"context"
	"encoding/json"
	"time"
)

// Dispatcher 任务分发器接口
type Dispatcher interface {
	// Dispatch 投递一个子任务到队列
	Dispatch(ctx context.Context, taskName string, payload interface{}) error
}

// TaskType 任务执行类型
type TaskType string

const (
	TypeOnce     TaskType = "once"     // 启动时执行一次
	TypeInterval TaskType = "interval" // 按间隔时间循环执行
	TypeCron     TaskType = "cron"     // 按 Cron 表达式定时执行
)

// 系统预定义的优先级权重
const (
	WeightSystem    = 100 // 最高优先级，涉及数据库迁移等基础环境
	WeightEssential = 80  // 核心功能优先级
	WeightNormal    = 50  // 普通业务优先级
	WeightLow       = 10  // 低优先级
)

// TaskMetadata 任务元数据
type TaskMetadata struct {
	Name        string   // 任务唯一标识 (用于配置文件匹配)
	DisplayName string   // 任务显示名称 (用于日志和控制台)
	Type        TaskType // 任务类型
	Spec        string   // 执行参数 (间隔或 Cron)
	Weight      int      // 权重 (数值越大越早运行)
	Enabled     bool     // 是否启用
}

// ExecutionInfo 单次执行信息
type ExecutionInfo struct {
	StartTime time.Time
	EndTime   time.Time
	Duration  time.Duration
	Status    string // success, error
	Message   string // 结果详情或错误信息
}

// RuntimeState 任务实时运行状态
type RuntimeState struct {
	IsRunning      bool
	LastRunTime    time.Time
	LastDuration   time.Duration
	LastStatus     string
	LastMessage    string
	NextRunTime    time.Time
	ExecutionCount uint64
}

// Task 任务接口
type Task interface {
	Name() string
	DisplayName() string
	// Run 由调度引擎定时触发 (生产者角色)
	Run(ctx context.Context) error
	// Execute 处理队列中的具体任务载荷 (消费者角色)
	Execute(ctx context.Context, payload json.RawMessage) error
}

// TaskWithMetadata 允许任务自带默认元数据
type TaskWithMetadata interface {
	Task
	DefaultMetadata() TaskMetadata
}
