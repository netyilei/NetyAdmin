package system

import (
	"context"

	"netyadmin/internal/domain/entity/system"
	"netyadmin/internal/pkg/cache"
	sysRepo "netyadmin/internal/repository/system"
)

type DictService interface {
	// 类型
	ListType(ctx context.Context, name, code, status string, page, pageSize int) ([]system.DictType, int64, error)
	CreateType(ctx context.Context, t *system.DictType) error
	UpdateType(ctx context.Context, t *system.DictType) error
	DeleteType(ctx context.Context, id uint) error

	// 数据
	ListData(ctx context.Context, dictCode string) ([]system.DictData, error)
	ListDataFull(ctx context.Context, dictCode, label, status string, page, pageSize int) ([]system.DictData, int64, error)
	CreateData(ctx context.Context, d *system.DictData) error
	UpdateData(ctx context.Context, d *system.DictData) error
	DeleteData(ctx context.Context, id uint) error
}

type dictService struct {
	dictRepo sysRepo.DictRepository
	cacheMgr cache.LazyCacheManager
}

func NewDictService(dictRepo sysRepo.DictRepository, cacheMgr cache.LazyCacheManager) DictService {
	return &dictService{
		dictRepo: dictRepo,
		cacheMgr: cacheMgr,
	}
}

func (s *dictService) ListType(ctx context.Context, name, code, status string, page, pageSize int) ([]system.DictType, int64, error) {
	return s.dictRepo.ListType(ctx, name, code, status, page, pageSize)
}

func (s *dictService) CreateType(ctx context.Context, t *system.DictType) error {
	return s.dictRepo.CreateType(ctx, t)
}

func (s *dictService) UpdateType(ctx context.Context, t *system.DictType) error {
	err := s.dictRepo.UpdateType(ctx, t)
	if err == nil {
		// 修改类型后，失效该类型下所有数据的缓存
		_ = s.cacheMgr.InvalidateByTags(ctx, cache.TagDict(t.Code))
	}
	return err
}

func (s *dictService) DeleteType(ctx context.Context, id uint) error {
	t, err := s.dictRepo.GetTypeById(ctx, id)
	if err != nil {
		return err
	}
	err = s.dictRepo.DeleteType(ctx, id)
	if err == nil {
		_ = s.cacheMgr.InvalidateByTags(ctx, cache.TagDict(t.Code))
	}
	return err
}

func (s *dictService) ListData(ctx context.Context, dictCode string) ([]system.DictData, error) {
	var list []system.DictData
	cacheKey := cache.KeyDictData(dictCode)
	tag := cache.TagDict(dictCode)

	err := s.cacheMgr.Fetch(ctx, cacheKey, "dict", []string{tag}, cache.TTL_Default, &list, func() (interface{}, error) {
		return s.dictRepo.ListData(ctx, dictCode)
	})

	return list, err
}

func (s *dictService) ListDataFull(ctx context.Context, dictCode, label, status string, page, pageSize int) ([]system.DictData, int64, error) {
	return s.dictRepo.ListDataFull(ctx, dictCode, label, status, page, pageSize)
}

func (s *dictService) CreateData(ctx context.Context, d *system.DictData) error {
	err := s.dictRepo.CreateData(ctx, d)
	if err == nil {
		_ = s.cacheMgr.InvalidateByTags(ctx, cache.TagDict(d.DictCode))
	}
	return err
}

func (s *dictService) UpdateData(ctx context.Context, d *system.DictData) error {
	err := s.dictRepo.UpdateData(ctx, d)
	if err == nil {
		_ = s.cacheMgr.InvalidateByTags(ctx, cache.TagDict(d.DictCode))
	}
	return err
}

func (s *dictService) DeleteData(ctx context.Context, id uint) error {
	d, err := s.dictRepo.GetDataById(ctx, id)
	if err != nil {
		return err
	}
	err = s.dictRepo.DeleteData(ctx, id)
	if err == nil {
		_ = s.cacheMgr.InvalidateByTags(ctx, cache.TagDict(d.DictCode))
	}
	return err
}
