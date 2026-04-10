package log

import (
	"context"

	logDto "NetyAdmin/internal/interface/admin/dto/log"
	logEntity "NetyAdmin/internal/domain/entity/log"
	logVO "NetyAdmin/internal/domain/vo/log"
	logRepo "NetyAdmin/internal/repository/log"
)

type OperationService interface {
	Create(ctx context.Context, log *logEntity.Operation) error
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

func (s *operationService) Create(ctx context.Context, log *logEntity.Operation) error {
	return s.logRepo.Create(ctx, log)
}

func (s *operationService) List(ctx context.Context, req *logDto.OperationQueryReq) (*logVO.OperationListVO, error) {
	req.Normalize()

	logs, total, err := s.logRepo.List(ctx, req)
	if err != nil {
		return nil, err
	}

	list := make([]logVO.OperationVO, 0, len(logs))
	for _, log := range logs {
		list = append(list, logVO.OperationVO{
			ID:        log.ID,
			UserID:    log.UserID,
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
