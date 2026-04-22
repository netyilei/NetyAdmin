package log

import (
	"context"

	logEntity "NetyAdmin/internal/domain/entity/log"
	openEntity "NetyAdmin/internal/domain/entity/open_platform"
	openRepo "NetyAdmin/internal/repository/open_platform"
	taskEntity "NetyAdmin/internal/domain/entity/task"
	taskRepo "NetyAdmin/internal/repository/task"
)

type openLogWriter struct {
	repo openRepo.OpenLogRepository
}

func NewOpenLogWriter(repo openRepo.OpenLogRepository) LogBatchWriter {
	return &openLogWriter{repo: repo}
}

func (w *openLogWriter) WriteBatch(ctx context.Context, entries []logEntity.LogEntry) error {
	if len(entries) == 0 {
		return nil
	}
	logs := make([]*openEntity.OpenPlatformLog, 0, len(entries))
	for _, e := range entries {
		if op, ok := e.(*openEntity.OpenPlatformLog); ok {
			logs = append(logs, op)
		}
	}
	return w.repo.BatchCreate(ctx, logs)
}

type taskLogWriter struct {
	repo taskRepo.TaskLogRepository
}

func NewTaskLogWriter(repo taskRepo.TaskLogRepository) LogBatchWriter {
	return &taskLogWriter{repo: repo}
}

func (w *taskLogWriter) WriteBatch(ctx context.Context, entries []logEntity.LogEntry) error {
	if len(entries) == 0 {
		return nil
	}
	logs := make([]*taskEntity.TaskLog, 0, len(entries))
	for _, e := range entries {
		if tl, ok := e.(*taskEntity.TaskLog); ok {
			logs = append(logs, tl)
		}
	}
	return w.repo.BatchCreate(ctx, logs)
}
