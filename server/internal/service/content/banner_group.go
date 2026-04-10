package content

import (
	"context"

	contentEntity "netyadmin/internal/domain/entity/content"
	contentDto "netyadmin/internal/interface/admin/dto/content"
	"netyadmin/internal/pkg/errorx"
	contentRepo "netyadmin/internal/repository/content"
	storageService "netyadmin/internal/service/storage"
)

type BannerGroupService interface {
	Create(ctx context.Context, adminID uint, req *contentDto.CreateContentBannerGroupDTO) (*contentEntity.ContentBannerGroup, error)
	Update(ctx context.Context, adminID uint, id uint, req *contentDto.UpdateContentBannerGroupDTO) (*contentEntity.ContentBannerGroup, error)
	Delete(ctx context.Context, id uint) error
	GetByID(ctx context.Context, id uint) (*contentEntity.ContentBannerGroup, error)
	GetByIDWithBanners(ctx context.Context, id uint) (*contentEntity.ContentBannerGroup, error)
	List(ctx context.Context, query *contentDto.ContentBannerGroupListQueryDTO) ([]*contentEntity.ContentBannerGroup, int64, error)
	GetAll(ctx context.Context) ([]*contentEntity.ContentBannerGroup, error)
}

type bannerGroupService struct {
	repo           contentRepo.ContentBannerGroupRepository
	storageService storageService.ConfigService
}

func NewBannerGroupService(repo contentRepo.ContentBannerGroupRepository, storageService storageService.ConfigService) BannerGroupService {
	return &bannerGroupService{
		repo:           repo,
		storageService: storageService,
	}
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
			return nil, errorx.New(errorx.CodeNotFound, "存储配置不存在")
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
				return nil, errorx.New(errorx.CodeNotFound, "存储配置不存在")
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

	return group, nil
}

func (s *bannerGroupService) Delete(ctx context.Context, id uint) error {
	hasBanners, err := s.repo.HasBanners(ctx, id)
	if err != nil {
		return err
	}
	if hasBanners {
		return errorx.New(errorx.CodeBadRequest, "该Banner组下存在Banner项，无法删除")
	}

	return s.repo.Delete(ctx, id)
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
