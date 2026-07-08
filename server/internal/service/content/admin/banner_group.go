package admin

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"

	contentEntity "NetyAdmin/internal/domain/entity/content"
	contentDto "NetyAdmin/internal/interface/admin/dto/content"
	"NetyAdmin/internal/pkg/cache"
	"NetyAdmin/internal/pkg/configsync"
	"NetyAdmin/internal/pkg/database"
	"NetyAdmin/internal/pkg/errorx"
	contentRepo "NetyAdmin/internal/repository/content"
	storageService "NetyAdmin/internal/service/storage"
)

type BannerGroupService interface {
	Create(ctx context.Context, adminID uint, req *contentDto.CreateContentBannerGroupDTO) (*contentEntity.ContentBannerGroup, error)
	Update(ctx context.Context, adminID uint, id uint, req *contentDto.UpdateContentBannerGroupDTO) (*contentEntity.ContentBannerGroup, error)
	Delete(ctx context.Context, id uint) error
	GetByID(ctx context.Context, id uint) (*contentEntity.ContentBannerGroup, error)
	GetByIDWithBanners(ctx context.Context, id uint) (*contentEntity.ContentBannerGroup, error)
	List(ctx context.Context, query *contentDto.ContentBannerGroupListQueryDTO) ([]*contentEntity.ContentBannerGroup, int64, error)
	GetAll(ctx context.Context) ([]*contentEntity.ContentBannerGroup, error)
	GetByCode(ctx context.Context, code string) (*contentEntity.ContentBannerGroup, error)
}

type bannerGroupService struct {
	repo           contentRepo.ContentBannerGroupRepository
	bannerItemRepo contentRepo.ContentBannerItemRepository
	storageService storageService.ConfigService
	cache          cache.ConfigCache
	watcher        configsync.ConfigWatcher
	tm             *database.TransactionManager
}

func NewBannerGroupService(repo contentRepo.ContentBannerGroupRepository, bannerItemRepo contentRepo.ContentBannerItemRepository, storageService storageService.ConfigService, cache cache.ConfigCache, watcher configsync.ConfigWatcher, tm *database.TransactionManager) BannerGroupService {
	return &bannerGroupService{
		repo:           repo,
		bannerItemRepo: bannerItemRepo,
		storageService: storageService,
		cache:          cache,
		watcher:        watcher,
		tm:             tm,
	}
}

func (s *bannerGroupService) getBannerCacheTTL() time.Duration {
	val, ok := s.watcher.GetConfig(cache.ConfigGroupContentCache, cache.ConfigKeyBannerCacheTTL)
	if ok {
		if mins, err := time.ParseDuration(val + "m"); err == nil {
			return mins
		}
	}
	return 30 * time.Minute
}

func (s *bannerGroupService) Create(ctx context.Context, adminID uint, req *contentDto.CreateContentBannerGroupDTO) (*contentEntity.ContentBannerGroup, error) {
	exists, err := s.repo.ExistsByCode(ctx, req.Code)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, errorx.New(errorx.CodeAlreadyExists, "Banner组编码已存在")
	}

	if req.StorageConfigID != nil && *req.StorageConfigID > 0 {
		_, err := s.storageService.GetByID(ctx, *req.StorageConfigID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errorx.New(errorx.CodeNotFound, "存储配置不存在")
			}
			slog.Error("storageService.GetByID failed", "storageConfigID", *req.StorageConfigID, "err", err)
			return nil, fmt.Errorf("storageService.GetByID: %w", err)
		}
	}

	status := "1"
	if req.Status == "0" {
		status = "0"
	}

	maxItems := req.MaxItems
	if maxItems <= 0 {
		maxItems = 10
	}

	interval := req.Interval
	if interval <= 0 {
		interval = 5000
	}

	group := &contentEntity.ContentBannerGroup{
		Name:            req.Name,
		Code:            req.Code,
		Description:     req.Description,
		Position:        req.Position,
		Width:           req.Width,
		Height:          req.Height,
		MaxItems:        maxItems,
		AutoPlay:        req.AutoPlay,
		Interval:        interval,
		Sort:            req.Sort,
		StorageConfigID: req.StorageConfigID,
		Status:          status,
		Remark:          req.Remark,
	}
	group.CreatedBy = adminID
	group.UpdatedBy = adminID

	if err := s.repo.Create(ctx, group); err != nil {
		return nil, err
	}

	if err := s.cache.InvalidateByTags(ctx, cache.TagContentBanner); err != nil {
		slog.Error("invalidate cache failed", "tag", cache.TagContentBanner, "err", err)
	}

	return group, nil
}

func (s *bannerGroupService) Update(ctx context.Context, adminID uint, id uint, req *contentDto.UpdateContentBannerGroupDTO) (*contentEntity.ContentBannerGroup, error) {
	group, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// 编码唯一性校验：如果提供了新编码且与旧编码不同
	if req.Code != nil && *req.Code != "" && *req.Code != group.Code {
		exists, err := s.repo.ExistsByCode(ctx, *req.Code, id)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, errorx.New(errorx.CodeAlreadyExists, "Banner组编码已存在")
		}
		group.Code = *req.Code
	} else if req.Code != nil && *req.Code == "" {
		return nil, errorx.New(errorx.CodeInvalidParams, "Banner组编码不能为空")
	}

	if req.Name != nil && *req.Name != "" {
		group.Name = *req.Name
	} else if req.Name != nil && *req.Name == "" {
		return nil, errorx.New(errorx.CodeInvalidParams, "Banner组名称不能为空")
	}
	if req.Description != "" {
		group.Description = req.Description
	}
	if req.Position != "" {
		group.Position = req.Position
	}
	if req.Width > 0 {
		group.Width = req.Width
	}
	if req.Height > 0 {
		group.Height = req.Height
	}
	if req.MaxItems > 0 {
		group.MaxItems = req.MaxItems
	}
	if req.AutoPlay != nil {
		group.AutoPlay = *req.AutoPlay
	}
	if req.Interval != nil && *req.Interval > 0 {
		group.Interval = *req.Interval
	}

	if req.StorageConfigID != nil {
		if *req.StorageConfigID > 0 {
			_, err := s.storageService.GetByID(ctx, *req.StorageConfigID)
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return nil, errorx.New(errorx.CodeNotFound, "存储配置不存在")
				}
				slog.Error("storageService.GetByID failed", "storageConfigID", *req.StorageConfigID, "err", err)
				return nil, fmt.Errorf("storageService.GetByID: %w", err)
			}
		}
		group.StorageConfigID = req.StorageConfigID
	}

	if req.Status != "" {
		group.Status = req.Status
	}
	if req.Remark != "" {
		group.Remark = req.Remark
	}
	group.Sort = req.Sort
	group.UpdatedBy = adminID

	if err := s.repo.Update(ctx, group); err != nil {
		return nil, err
	}

	if err := s.cache.InvalidateByTags(ctx, cache.TagContentBanner); err != nil {
		slog.Error("invalidate cache failed", "tag", cache.TagContentBanner, "err", err)
	}

	return group, nil
}

func (s *bannerGroupService) Delete(ctx context.Context, id uint) error {
	// TM 单事务原子完成「前置校验 + 硬删除」。
	// 避免 TOCTOU 竞态：检查与删除之间被并发请求插入 Banner 项。
	txCtx, tx := s.tm.Begin(ctx)

	itemCount, err := s.bannerItemRepo.CountByGroupID(txCtx, id)
	if err != nil {
		slog.Error("banner group delete: count items failed", "groupID", id, "err", err)
		s.tm.Rollback(tx)
		return errorx.New(errorx.CodeInternalError, "查询 Banner 项失败")
	}
	if itemCount > 0 {
		s.tm.Rollback(tx)
		return errorx.New(errorx.CodeBannerGroupHasItems)
	}

	if err := s.repo.Delete(txCtx, id); err != nil {
		slog.Error("banner group delete: delete failed", "groupID", id, "err", err)
		s.tm.Rollback(tx)
		return errorx.New(errorx.CodeInternalError, "Banner 组删除失败")
	}

	if err := s.tm.Commit(tx); err != nil {
		slog.Error("banner group delete: commit failed", "groupID", id, "err", err)
		return errorx.New(errorx.CodeInternalError, "事务提交失败")
	}

	if err := s.cache.InvalidateByTags(ctx, cache.TagContentBanner); err != nil {
		slog.Error("invalidate cache failed", "tag", cache.TagContentBanner, "err", err)
	}
	return nil
}

func (s *bannerGroupService) GetByID(ctx context.Context, id uint) (*contentEntity.ContentBannerGroup, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *bannerGroupService) GetByIDWithBanners(ctx context.Context, id uint) (*contentEntity.ContentBannerGroup, error) {
	return s.repo.GetByIDWithBanners(ctx, id)
}

func (s *bannerGroupService) List(ctx context.Context, query *contentDto.ContentBannerGroupListQueryDTO) ([]*contentEntity.ContentBannerGroup, int64, error) {
	repoQuery := &contentRepo.ContentBannerGroupQuery{
		Current:     query.Current,
		Size:        query.Size,
		Name:        query.Name,
		Code:        query.Code,
		Description: query.Description,
		Position:    query.Position,
		Status:      query.Status,
	}
	return s.repo.List(ctx, repoQuery)
}

func (s *bannerGroupService) GetAll(ctx context.Context) ([]*contentEntity.ContentBannerGroup, error) {
	return s.repo.GetAll(ctx)
}

func (s *bannerGroupService) GetByCode(ctx context.Context, code string) (*contentEntity.ContentBannerGroup, error) {
	cacheKey := cache.KeyContentBannerGroupByCode(code)
	cacheTags := []string{cache.TagContentBanner}
	ttl := s.getBannerCacheTTL()

	var group *contentEntity.ContentBannerGroup
	loader := func() (interface{}, error) {
		return s.repo.GetByCode(ctx, code)
	}

	err := s.cache.FetchFast(ctx, cacheKey, "content_banner_cache", cacheTags, ttl, &group, loader)
	if err != nil {
		return nil, err
	}

	return group, nil
}
