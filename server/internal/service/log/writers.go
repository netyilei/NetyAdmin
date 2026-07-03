package log

import (
	"context"

	logEntity "NetyAdmin/internal/domain/entity/log"
	logRepo "NetyAdmin/internal/repository/log"
)

// LogBatchWriter 批量写入日志的统一接口（在 logbus.go 中定义，此处保留以避免循环引用）。
// 各模块（operation/error/open/task）的 writer 实现统一收敛为 genericLogWriter（见 writers_ext.go）。

type operationLogWriter struct {
	repo *logRepo.OperationRepository
}

func NewOperationLogWriter(repo *logRepo.OperationRepository) LogBatchWriter {
	return &operationLogWriter{repo: repo}
}

func (w *operationLogWriter) WriteBatch(ctx context.Context, entries []logEntity.LogEntry) error {
	return filterAndBatch(ctx, entries, func(e logEntity.LogEntry) (*logEntity.Operation, bool) {
		op, ok := e.(*logEntity.Operation)
		return op, ok
	}, w.repo.BatchCreate)
}

type errorLogWriter struct {
	repo *logRepo.ErrorRepository
}

func NewErrorLogWriter(repo *logRepo.ErrorRepository) LogBatchWriter {
	return &errorLogWriter{repo: repo}
}

func (w *errorLogWriter) WriteBatch(ctx context.Context, entries []logEntity.LogEntry) error {
	return filterAndBatch(ctx, entries, func(e logEntity.LogEntry) (*logEntity.Error, bool) {
		err, ok := e.(*logEntity.Error)
		return err, ok
	}, w.repo.BatchUpsertByHash)
}
