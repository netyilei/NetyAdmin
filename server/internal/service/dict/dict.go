package dict

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"gorm.io/gorm"

	dictEntity "NetyAdmin/internal/domain/entity/dict"
	dictDto "NetyAdmin/internal/interface/admin/dto/dict"
	"NetyAdmin/internal/pkg/cache"
	"NetyAdmin/internal/pkg/database"
	"NetyAdmin/internal/pkg/errorx"
	dictRepo "NetyAdmin/internal/repository/dict"
)

type DictService interface {
	// 类型
	ListType(ctx context.Context, name, code, status string, page, pageSize int) ([]dictEntity.DictType, int64, error)
	CreateType(ctx context.Context, req *dictDto.CreateDictTypeReq) error
	UpdateType(ctx context.Context, req *dictDto.UpdateDictTypeReq) error
	DeleteType(ctx context.Context, id uint) error

	// 数据
	ListData(ctx context.Context, dictCode string) ([]dictEntity.DictData, error)
	ListDataFull(ctx context.Context, dictCode, label, status string, page, pageSize int) ([]dictEntity.DictData, int64, error)
	CreateData(ctx context.Context, req *dictDto.CreateDictDataReq) error
	UpdateData(ctx context.Context, req *dictDto.UpdateDictDataReq) error
	DeleteData(ctx context.Context, id uint) error
}

type dictService struct {
	dictRepo dictRepo.DictRepository
	cacheFast cache.ConfigCache
	tm       *database.TransactionManager
}

func NewDictService(dictRepo dictRepo.DictRepository, cacheFast cache.ConfigCache, tm *database.TransactionManager) DictService {
	return &dictService{
		dictRepo: dictRepo,
		cacheFast: cacheFast,
		tm:       tm,
	}
}

func (s *dictService) ListType(ctx context.Context, name, code, status string, page, pageSize int) ([]dictEntity.DictType, int64, error) {
	return s.dictRepo.ListType(ctx, name, code, status, page, pageSize)
}

func (s *dictService) CreateType(ctx context.Context, req *dictDto.CreateDictTypeReq) error {
	t := &dictEntity.DictType{
		Name:        req.Name,
		Code:        req.Code,
		Status:      req.Status,
		Description: req.Description,
	}
	return s.dictRepo.CreateType(ctx, t)
}

func (s *dictService) UpdateType(ctx context.Context, req *dictDto.UpdateDictTypeReq) error {
	// 复用 old 实例做 patch 后 Save，避免构造新 entity 导致 CreatedAt 等零值字段覆盖数据库已有值（Save 是全字段更新）。
	// Code 为业务唯一标识，创建后不可变更，Update 不修改 Code（基座设计原则）。
	old, err := s.dictRepo.GetTypeById(ctx, req.ID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errorx.New(errorx.CodeNotFound, fmt.Sprintf("字典类型 %d 不存在", req.ID))
		}
		slog.Error("dictRepo.GetTypeById failed", "typeID", req.ID, "err", err)
		return fmt.Errorf("dictRepo.GetTypeById: %w", err)
	}
	old.Name = req.Name
	old.Status = req.Status
	old.Description = req.Description
	if err := s.dictRepo.UpdateType(ctx, old); err != nil {
		return fmt.Errorf("dictRepo.UpdateType: %w", err)
	}
	// Code 未变更，只需失效当前 Code 的缓存
	tag := cache.TagDict(old.Code)
	if cErr := s.cacheFast.InvalidateByTags(ctx, tag); cErr != nil {
		slog.Error("invalidate cache failed", "tag", tag, "err", cErr)
	}
	return nil
}

func (s *dictService) DeleteType(ctx context.Context, id uint) error {
	// 用 WithTransaction 闭包 API 包裹「查询类型 + 删除字典数据 + 删除字典类型」三步原子写。
	// 任一步失败自动 Rollback；panic 自动 Rollback 后重抛让 recovery 中间件捕获 + Sentry 上报。
	// 缓存失效在 WithTransaction 返回 nil 后用原始 ctx 执行（保证「DB 已提交 → 缓存失效」顺序）。
	var dictCode string
	err := s.tm.WithTransaction(ctx, func(txCtx context.Context) error {
		t, err := s.dictRepo.GetTypeById(txCtx, id)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errorx.New(errorx.CodeNotFound, fmt.Sprintf("字典类型 %d 不存在", id))
			}
			slog.Error("dictRepo.GetTypeById failed", "typeID", id, "err", err)
			return fmt.Errorf("dictRepo.GetTypeById: %w", err)
		}
		dictCode = t.Code
		if err := s.dictRepo.DeleteDataByTypeCode(txCtx, t.Code); err != nil {
			slog.Error("dict delete type: delete data failed", "typeID", id, "code", t.Code, "err", err)
			return errorx.New(errorx.CodeInternalError, fmt.Sprintf("字典类型 %d 删除失败", id))
		}
		if err := s.dictRepo.DeleteType(txCtx, id); err != nil {
			slog.Error("dict delete type: delete type failed", "typeID", id, "err", err)
			return errorx.New(errorx.CodeInternalError, fmt.Sprintf("字典类型 %d 删除失败", id))
		}
		return nil
	})
	if err != nil {
		return err
	}
	// 事务后失效缓存
	tag := cache.TagDict(dictCode)
	if cErr := s.cacheFast.InvalidateByTags(ctx, tag); cErr != nil {
		slog.Error("invalidate cache failed", "tag", tag, "err", cErr)
	}
	return nil
}

func (s *dictService) ListData(ctx context.Context, dictCode string) ([]dictEntity.DictData, error) {
	var list []dictEntity.DictData
	cacheKey := cache.KeyDictData(dictCode)
	tag := cache.TagDict(dictCode)

	err := s.cacheFast.FetchFast(ctx, cacheKey, "dict", []string{tag}, cache.TTL_Default, &list, func() (interface{}, error) {
		return s.dictRepo.ListData(ctx, dictCode)
	})

	return list, err
}

func (s *dictService) ListDataFull(ctx context.Context, dictCode, label, status string, page, pageSize int) ([]dictEntity.DictData, int64, error) {
	return s.dictRepo.ListDataFull(ctx, dictCode, label, status, page, pageSize)
}

func (s *dictService) CreateData(ctx context.Context, req *dictDto.CreateDictDataReq) error {
	// 校验 DictCode 对应的 DictType 存在
	if _, err := s.dictRepo.GetTypeByCode(ctx, req.DictCode); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errorx.New(errorx.CodeNotFound, fmt.Sprintf("字典类型 %s 不存在", req.DictCode))
		}
		slog.Error("dictRepo.GetTypeByCode failed", "dictCode", req.DictCode, "err", err)
		return fmt.Errorf("dictRepo.GetTypeByCode: %w", err)
	}
	d := &dictEntity.DictData{
		DictCode: req.DictCode,
		Label:    req.Label,
		Value:    req.Value,
		TagType:  req.TagType,
		OrderBy:  req.OrderBy,
		Status:   req.Status,
		Remark:   req.Remark,
	}
	if err := s.dictRepo.CreateData(ctx, d); err != nil {
		return fmt.Errorf("dictRepo.CreateData: %w", err)
	}
	tag := cache.TagDict(req.DictCode)
	if cErr := s.cacheFast.InvalidateByTags(ctx, tag); cErr != nil {
		slog.Error("invalidate cache failed", "tag", tag, "err", cErr)
	}
	return nil
}

func (s *dictService) UpdateData(ctx context.Context, req *dictDto.UpdateDictDataReq) error {
	// 复用 old 实例做 patch 后 Save，避免构造新 entity 导致 CreatedAt 等零值字段覆盖数据库已有值（Save 是全字段更新）。
	// DictCode 为业务关联标识，创建后不可变更（字典数据归属固定的字典类型，不可跨类型迁移）。
	old, err := s.dictRepo.GetDataById(ctx, req.ID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errorx.New(errorx.CodeNotFound, fmt.Sprintf("字典数据 %d 不存在", req.ID))
		}
		slog.Error("dictRepo.GetDataById failed", "dataID", req.ID, "err", err)
		return fmt.Errorf("dictRepo.GetDataById: %w", err)
	}
	old.Label = req.Label
	old.Value = req.Value
	old.TagType = req.TagType
	old.OrderBy = req.OrderBy
	old.Status = req.Status
	old.Remark = req.Remark
	if err := s.dictRepo.UpdateData(ctx, old); err != nil {
		return fmt.Errorf("dictRepo.UpdateData: %w", err)
	}
	// DictCode 未变更，只需失效当前 DictCode 的缓存
	tag := cache.TagDict(old.DictCode)
	if cErr := s.cacheFast.InvalidateByTags(ctx, tag); cErr != nil {
		slog.Error("invalidate cache failed", "tag", tag, "err", cErr)
	}
	return nil
}

func (s *dictService) DeleteData(ctx context.Context, id uint) error {
	d, err := s.dictRepo.GetDataById(ctx, id)
	if err != nil {
		return fmt.Errorf("dictRepo.GetDataById: %w", err)
	}
	if err := s.dictRepo.DeleteData(ctx, id); err != nil {
		return fmt.Errorf("dictRepo.DeleteData: %w", err)
	}
	tag := cache.TagDict(d.DictCode)
	if cErr := s.cacheFast.InvalidateByTags(ctx, tag); cErr != nil {
		slog.Error("invalidate cache failed", "tag", tag, "err", cErr)
	}
	return nil
}
