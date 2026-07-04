package log

import (
	"context"

	logVO "NetyAdmin/internal/domain/vo/log"
	logDto "NetyAdmin/internal/interface/admin/dto/log"
	logRepo "NetyAdmin/internal/repository/log"
)

type OperationService interface {
	List(ctx context.Context, req *logDto.OperationQueryReq) (*logVO.OperationListVO, error)
	Delete(ctx context.Context, id uint) error
	DeleteBatch(ctx context.Context, ids []uint) error
}

type operationService struct {
	logRepo *logRepo.OperationRepository
}

func NewOperationService(logRepo *logRepo.OperationRepository) OperationService {
	return &operationService{logRepo: logRepo}
}

func (s *operationService) List(ctx context.Context, req *logDto.OperationQueryReq) (*logVO.OperationListVO, error) {
	// service 层接收 admin DTO，内部构造 repository query（spec B10：service 不应依赖 handler 构造的 repo 类型）
	repoQuery := &logRepo.OperationQuery{
		AdminID:   req.AdminID,
		Action:    req.Action,
		StartDate: req.StartDate,
		EndDate:   req.EndDate,
		Page:      req.Current,
		PageSize:  req.Size,
	}
	logs, total, err := s.logRepo.List(ctx, repoQuery)
	if err != nil {
		return nil, err
	}

	list := make([]logVO.OperationVO, 0, len(logs))
	for _, log := range logs {
		list = append(list, logVO.OperationVO{
			ID:        log.ID,
			AdminID:   log.AdminID,
			Username:  log.Username,
			Action:    log.Action,
			Resource:  log.Resource,
			Detail:    log.Detail,
			IP:        log.IP,
			UserAgent: log.UserAgent,
			CreatedAt: log.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	return &logVO.OperationListVO{
		Records: list,
		Current: req.Current,
		Size:    req.Size,
		Total:   total,
	}, nil
}

func (s *operationService) Delete(ctx context.Context, id uint) error {
	return s.logRepo.Delete(ctx, id)
}

func (s *operationService) DeleteBatch(ctx context.Context, ids []uint) error {
	return s.logRepo.DeleteBatch(ctx, ids)
}
