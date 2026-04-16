package open_platform

import (
	"context"

	"NetyAdmin/internal/domain/entity/open_platform"
	openRepo "NetyAdmin/internal/repository/open_platform"
)

type OpenLogService interface {
	Record(ctx context.Context, log *open_platform.OpenPlatformLog) error
	ListLogs(ctx context.Context, query *openRepo.OpenLogRepoQuery) ([]*open_platform.OpenPlatformLog, int64, error)
	GetLog(ctx context.Context, id uint64) (*open_platform.OpenPlatformLog, error)
	DeleteBatch(ctx context.Context, ids []uint64) error
	ClearOldLogs(ctx context.Context, days int) error
}

type openLogService struct {
	repo openRepo.OpenLogRepository
}

func NewOpenLogService(repo openRepo.OpenLogRepository) OpenLogService {
	return &openLogService{repo: repo}
}

func (s *openLogService) Record(ctx context.Context, log *open_platform.OpenPlatformLog) error {
	return s.repo.Create(ctx, log)
}

func (s *openLogService) ListLogs(ctx context.Context, query *openRepo.OpenLogRepoQuery) ([]*open_platform.OpenPlatformLog, int64, error) {
	return s.repo.List(ctx, query)
}

func (s *openLogService) GetLog(ctx context.Context, id uint64) (*open_platform.OpenPlatformLog, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *openLogService) DeleteBatch(ctx context.Context, ids []uint64) error {
	return s.repo.DeleteBatch(ctx, ids)
}

func (s *openLogService) ClearOldLogs(ctx context.Context, days int) error {
	return s.repo.Clear(ctx, days)
}
