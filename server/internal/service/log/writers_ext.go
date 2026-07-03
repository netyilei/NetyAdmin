package log

import (
	"context"

	logEntity "NetyAdmin/internal/domain/entity/log"
	openEntity "NetyAdmin/internal/domain/entity/open_platform"
	openRepo "NetyAdmin/internal/repository/open_platform"
	taskEntity "NetyAdmin/internal/domain/entity/task"
	taskRepo "NetyAdmin/internal/repository/task"
)

// filterAndBatch 是 4 套 LogBatchWriter 的统一实现骨架（重构清单 B-OTHER-4）。
//
// 各 writer 的 WriteBatch 结构完全相同：判空 → 类型断言过滤 → 调 repo 的 Batch* 方法。
// 通过泛型收敛为单一实现，writer 只需提供：
//   - cast: 把 LogEntry 类型断言为目标类型 T（返回 T, bool；false 表示类型不符跳过）
//   - batch: repo 的批量写入方法（接收 []T）
//
// T 通常是实体指针类型（如 *logEntity.Operation），batch 接收 []T 即 []*Operation。
// 这消除了 operation/error/open/task 四个 writer 的复制粘贴，且新增日志类型只需声明 cast+batch。
func filterAndBatch[T any](
	ctx context.Context,
	entries []logEntity.LogEntry,
	cast func(logEntity.LogEntry) (T, bool),
	batch func(context.Context, []T) error,
) error {
	if len(entries) == 0 {
		return nil
	}
	logs := make([]T, 0, len(entries))
	for _, e := range entries {
		if item, ok := cast(e); ok {
			logs = append(logs, item)
		}
	}
	if len(logs) == 0 {
		return nil
	}
	return batch(ctx, logs)
}

// --- open log writer ---

type openLogWriter struct {
	repo openRepo.OpenLogRepository
}

func NewOpenLogWriter(repo openRepo.OpenLogRepository) LogBatchWriter {
	return &openLogWriter{repo: repo}
}

func (w *openLogWriter) WriteBatch(ctx context.Context, entries []logEntity.LogEntry) error {
	return filterAndBatch(ctx, entries, func(e logEntity.LogEntry) (*openEntity.OpenPlatformLog, bool) {
		op, ok := e.(*openEntity.OpenPlatformLog)
		return op, ok
	}, w.repo.BatchCreate)
}

// --- task log writer ---

type taskLogWriter struct {
	repo taskRepo.TaskLogRepository
}

func NewTaskLogWriter(repo taskRepo.TaskLogRepository) LogBatchWriter {
	return &taskLogWriter{repo: repo}
}

func (w *taskLogWriter) WriteBatch(ctx context.Context, entries []logEntity.LogEntry) error {
	return filterAndBatch(ctx, entries, func(e logEntity.LogEntry) (*taskEntity.TaskLog, bool) {
		tl, ok := e.(*taskEntity.TaskLog)
		return tl, ok
	}, w.repo.BatchCreate)
}
