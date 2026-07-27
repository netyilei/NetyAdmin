package open_platform

import (
	"context"
	"fmt"

	"NetyAdmin/internal/domain/entity/open_platform"
	"NetyAdmin/internal/pkg/errorx"
	openDto "NetyAdmin/internal/interface/admin/dto/open_platform"
	openRepo "NetyAdmin/internal/repository/open_platform"
)

type RecordFunc func(ctx context.Context, log *open_platform.OpenPlatformLog) error

type OpenLogService interface {
	Record(ctx context.Context, req *openDto.RecordOpenLogReq) error
	ListLogs(ctx context.Context, req *openDto.OpenLogQuery) ([]*open_platform.OpenPlatformLog, int64, error)
	GetLog(ctx context.Context, id uint64) (*open_platform.OpenPlatformLog, error)
	DeleteBatch(ctx context.Context, ids []uint64) error
	ClearOldLogs(ctx context.Context, days int) error
	GetStatistics(ctx context.Context, req *openDto.StatisticsQuery) (interface{}, error)
}

type openLogService struct {
	repo       openRepo.OpenLogRepository
	recordFunc RecordFunc
}

func NewOpenLogService(repo openRepo.OpenLogRepository, recordFunc RecordFunc) OpenLogService {
	return &openLogService{
		repo:       repo,
		recordFunc: recordFunc,
	}
}

func (s *openLogService) Record(ctx context.Context, req *openDto.RecordOpenLogReq) error {
	log := &open_platform.OpenPlatformLog{
		AppID:         req.AppID,
		AppKey:        req.AppKey,
		ApiPath:       req.ApiPath,
		ApiMethod:     req.ApiMethod,
		ClientIP:      req.ClientIP,
		StatusCode:    req.StatusCode,
		Latency:       req.Latency,
		RequestHeader: req.RequestHeader,
		RequestBody:   req.RequestBody,
		ResponseBody:  req.ResponseBody,
		ErrorMsg:      req.ErrorMsg,
		// Task 8.5: 透传 request_id 到 entity，便于通过 request_id 关联异步日志与原始请求。
		// recordFunc 闭包由 wire.go 注入（多态分发给 logBus.Record），最终写入 DB。
		RequestID: req.RequestID,
	}
	return s.recordFunc(ctx, log)
}

func (s *openLogService) ListLogs(ctx context.Context, req *openDto.OpenLogQuery) ([]*open_platform.OpenPlatformLog, int64, error) {
	// service 层接收 admin DTO，内部构造 repository query（spec B10：service 不应依赖 handler 构造的 repo 类型）
	repoQuery := &openRepo.OpenLogRepoQuery{
		Page:       req.Current,
		PageSize:   req.Size,
		AppID:      req.AppID,
		AppKey:     req.AppKey,
		ApiPath:    req.ApiPath,
		StatusCode: req.StatusCode,
		StartTime:  req.StartTime,
		EndTime:    req.EndTime,
	}
	return s.repo.List(ctx, repoQuery)
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

// GetStatistics dispatches to the concrete repository method based on req.Type.
// Single switch site — no separate validStatTypes map (default branch rejects).
func (s *openLogService) GetStatistics(ctx context.Context, req *openDto.StatisticsQuery) (interface{}, error) {
	repoQuery := &openRepo.StatisticsRepoQuery{
		Type:        req.Type,
		StartTime:   req.StartTime,
		EndTime:     req.EndTime,
		AppID:       req.AppID,
		Granularity: req.Granularity,
	}

	switch req.Type {
	case "trend":
		return s.repo.GetTrendStats(ctx, repoQuery)
	case "top_apps":
		return s.repo.GetTopAppsStats(ctx, repoQuery)
	case "top_apis":
		return s.repo.GetTopApisStats(ctx, repoQuery)
	case "status_distribution":
		return s.repo.GetStatusDistributionStats(ctx, repoQuery)
	case "latency_stats":
		return s.repo.GetLatencyStats(ctx, repoQuery)
	case "overview":
		return s.repo.GetOverviewStats(ctx, repoQuery)
	default:
		return nil, errorx.New(errorx.CodeStatisticsInvalidParams, fmt.Sprintf("不支持的统计类型: %s", req.Type))
	}
}
