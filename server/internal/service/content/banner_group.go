package content

import (
	"context"
	"errors"

	contentDto "silentorder/internal/interface/admin/dto/content"
	contentEntity "silentorder/internal/domain/entity/content"
	contentRepo "silentorder/internal/repository/content"
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
	repo contentRepo.ContentBannerGroupRepository
}

func NewBannerGroupService(repo contentRepo.ContentBannerGroupRepository) BannerGroupService {
	return &bannerGroupService{repo: repo}
}

func (s *bannerGroupService) Create(ctx context.Context, adminID uint, req *contentDto.CreateContentBannerGroupDTO) (*contentEntity.ContentBannerGroup, error) {
	exists, err := s.repo.ExistsByCode(ctx, req.Code)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, errors.New("Banner组编码已存在")
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
		Name:        req.Name,
		Code:        req.Code,
		Description: req.Description,
		Position:    req.Position,
		Width:       req.Width,
		Height:      req.Height,
		MaxItems:    maxItems,
		AutoPlay:    req.AutoPlay,
		Interval:    interval,
		Sort:        req.Sort,
		Status:      status,
		Remark:      req.Remark,
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

	if req.Name != "" {
		group.Name = req.Name
	}
	group.Description = req.Description
	group.Position = req.Position
	group.Width = req.Width
	group.Height = req.Height
	if req.MaxItems > 0 {
		group.MaxItems = req.MaxItems
	}
	if req.AutoPlay != nil {
		group.AutoPlay = *req.AutoPlay
	}
	if req.Interval != nil && *req.Interval > 0 {
		group.Interval = *req.Interval
	}
	group.Sort = req.Sort
	if req.Status != "" {
		group.Status = req.Status
	}
	group.Remark = req.Remark
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
		return errors.New("该Banner组下存在Banner项，无法删除")
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
