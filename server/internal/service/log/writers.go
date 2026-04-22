package log

import (
	"context"

	logEntity "NetyAdmin/internal/domain/entity/log"
	logRepo "NetyAdmin/internal/repository/log"
)

type operationLogWriter struct {
	repo *logRepo.OperationRepository
}

func NewOperationLogWriter(repo *logRepo.OperationRepository) LogBatchWriter {
	return &operationLogWriter{repo: repo}
}

func (w *operationLogWriter) WriteBatch(ctx context.Context, entries []logEntity.LogEntry) error {
	if len(entries) == 0 {
		return nil
	}
	logs := make([]*logEntity.Operation, 0, len(entries))
	for _, e := range entries {
		if op, ok := e.(*logEntity.Operation); ok {
			logs = append(logs, op)
		}
	}
	return w.repo.BatchCreate(ctx, logs)
}

type errorLogWriter struct {
	repo *logRepo.ErrorRepository
}

func NewErrorLogWriter(repo *logRepo.ErrorRepository) LogBatchWriter {
	return &errorLogWriter{repo: repo}
}

func (w *errorLogWriter) WriteBatch(ctx context.Context, entries []logEntity.LogEntry) error {
	if len(entries) == 0 {
		return nil
	}
	logs := make([]*logEntity.Error, 0, len(entries))
	for _, e := range entries {
		if err, ok := e.(*logEntity.Error); ok {
			logs = append(logs, err)
		}
	}
	return w.repo.BatchUpsertByHash(ctx, logs)
}
