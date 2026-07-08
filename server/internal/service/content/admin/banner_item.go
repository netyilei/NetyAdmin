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
	"NetyAdmin/internal/pkg/errorx"
	contentRepo "NetyAdmin/internal/repository/content"
)

type BannerItemService interface {
	Create(ctx context.Context, adminID uint, req *contentDto.CreateContentBannerItemDTO) (*contentEntity.ContentBannerItem, error)
	Update(ctx context.Context, adminID uint, id uint, req *contentDto.UpdateContentBannerItemDTO) (*contentEntity.ContentBannerItem, error)
	Delete(ctx context.Context, id uint) error
	GetByID(ctx context.Context, id uint) (*contentEntity.ContentBannerItem, error)
	List(ctx context.Context, query *contentDto.ContentBannerItemListQueryDTO) ([]*contentEntity.ContentBannerItem, int64, error)
	GetByGroupID(ctx context.Context, groupID uint) ([]*contentEntity.ContentBannerItem, error)
	IncrementClickCount(ctx context.Context, id uint) error
}

type bannerItemService struct {
	repo        contentRepo.ContentBannerItemRepository
	groupRepo   contentRepo.ContentBannerGroupRepository
	articleRepo contentRepo.ContentArticleRepository
	cache       cache.ConfigCache
}

func NewBannerItemService(
	repo contentRepo.ContentBannerItemRepository,
	groupRepo contentRepo.ContentBannerGroupRepository,
	articleRepo contentRepo.ContentArticleRepository,
	cache cache.ConfigCache,
) BannerItemService {
	return &bannerItemService{
		repo:        repo,
		groupRepo:   groupRepo,
		articleRepo: articleRepo,
		cache:       cache,
	}
}

func (s *bannerItemService) Create(ctx context.Context, adminID uint, req *contentDto.CreateContentBannerItemDTO) (*contentEntity.ContentBannerItem, error) {
	group, err := s.groupRepo.GetByID(ctx, req.GroupID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.New(errorx.CodeNotFound, "Banner组不存在")
		}
		slog.Error("groupRepo.GetByID failed", "groupID", req.GroupID, "err", err)
		return nil, fmt.Errorf("groupRepo.GetByID: %w", err)
	}

	count, err := s.repo.CountByGroupID(ctx, req.GroupID)
	if err != nil {
		return nil, err
	}
	if int(count) >= group.MaxItems {
		return nil, errorx.New(errorx.CodeBadRequest, "已达到Banner组最大数量限制")
	}

	linkType := contentEntity.LinkTypeNone
	if req.LinkType == "internal" {
		linkType = contentEntity.LinkTypeInternal
	} else if req.LinkType == "external" {
		linkType = contentEntity.LinkTypeExternal
	} else if req.LinkType == "article" {
		linkType = contentEntity.LinkTypeArticle
		if req.LinkArticleID != nil {
			_, err := s.articleRepo.GetByID(ctx, *req.LinkArticleID)
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return nil, errorx.New(errorx.CodeNotFound, "关联文章不存在")
				}
				slog.Error("articleRepo.GetByID failed", "articleID", *req.LinkArticleID, "err", err)
				return nil, fmt.Errorf("articleRepo.GetByID: %w", err)
			}
		}
	}

	var startTime, endTime *time.Time
	if req.StartTime != nil {
		t, err := time.Parse(time.RFC3339, *req.StartTime)
		if err == nil {
			startTime = &t
		}
	}
	if req.EndTime != nil {
		t, err := time.Parse(time.RFC3339, *req.EndTime)
		if err == nil {
			endTime = &t
		}
	}

	status := "1"
	if req.Status == "0" {
		status = "0"
	}

	item := &contentEntity.ContentBannerItem{
		GroupID:       req.GroupID,
		Title:         req.Title,
		Subtitle:      req.Subtitle,
		ImageURL:      req.ImageURL,
		ImageAlt:      req.ImageAlt,
		LinkType:      linkType,
		LinkURL:       req.LinkURL,
		LinkArticleID: req.LinkArticleID,
		Content:       req.Content,
		CustomParams:  req.CustomParams,
		Sort:          req.Sort,
		StartTime:     startTime,
		EndTime:       endTime,
		Status:        status,
	}
	item.CreatedBy = adminID
	item.UpdatedBy = adminID

	if err := s.repo.Create(ctx, item); err != nil {
		return nil, err
	}

	if err := s.cache.InvalidateByTags(ctx, cache.TagContentBanner); err != nil {
		slog.Error("invalidate cache failed", "tag", cache.TagContentBanner, "err", err)
	}

	return item, nil
}

func (s *bannerItemService) Update(ctx context.Context, adminID uint, id uint, req *contentDto.UpdateContentBannerItemDTO) (*contentEntity.ContentBannerItem, error) {
	item, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Title != "" {
		item.Title = req.Title
	}
	item.Subtitle = req.Subtitle
	if req.ImageURL != "" {
		item.ImageURL = req.ImageURL
	}
	item.ImageAlt = req.ImageAlt
	if req.LinkType != "" {
		item.LinkType = contentEntity.LinkType(req.LinkType)
	}
	item.LinkURL = req.LinkURL
	if req.LinkArticleID != nil {
		if *req.LinkArticleID > 0 {
			_, err := s.articleRepo.GetByID(ctx, *req.LinkArticleID)
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return nil, errorx.New(errorx.CodeNotFound, "关联文章不存在")
				}
				slog.Error("articleRepo.GetByID failed", "articleID", *req.LinkArticleID, "err", err)
				return nil, fmt.Errorf("articleRepo.GetByID: %w", err)
			}
		}
		item.LinkArticleID = req.LinkArticleID
	}
	item.Content = req.Content
	item.CustomParams = req.CustomParams
	item.Sort = req.Sort
	if req.StartTime != nil {
		t, err := time.Parse(time.RFC3339, *req.StartTime)
		if err == nil {
			item.StartTime = &t
		}
	}
	if req.EndTime != nil {
		t, err := time.Parse(time.RFC3339, *req.EndTime)
		if err == nil {
			item.EndTime = &t
		}
	}
	if req.Status != "" {
		item.Status = req.Status
	}
	item.UpdatedBy = adminID

	if err := s.repo.Update(ctx, item); err != nil {
		return nil, err
	}

	if err := s.cache.InvalidateByTags(ctx, cache.TagContentBanner); err != nil {
		slog.Error("invalidate cache failed", "tag", cache.TagContentBanner, "err", err)
	}

	return item, nil
}

func (s *bannerItemService) Delete(ctx context.Context, id uint) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("bannerItemRepo.Delete: %w", err)
	}
	if err := s.cache.InvalidateByTags(ctx, cache.TagContentBanner); err != nil {
		slog.Error("invalidate cache failed", "tag", cache.TagContentBanner, "err", err)
	}
	return nil
}

func (s *bannerItemService) GetByID(ctx context.Context, id uint) (*contentEntity.ContentBannerItem, error) {
	return s.repo.GetByIDWithGroup(ctx, id)
}

func (s *bannerItemService) List(ctx context.Context, query *contentDto.ContentBannerItemListQueryDTO) ([]*contentEntity.ContentBannerItem, int64, error) {
	repoQuery := &contentRepo.ContentBannerItemQuery{
		Current: query.Current,
		Size:    query.Size,
		GroupID: query.GroupID,
		Title:   query.Title,
		Status:  query.Status,
	}

	if query.StartTime != "" {
		repoQuery.StartTime = query.StartTime
	}
	if query.EndTime != "" {
		repoQuery.EndTime = query.EndTime
	}

	return s.repo.List(ctx, repoQuery)
}

func (s *bannerItemService) GetByGroupID(ctx context.Context, groupID uint) ([]*contentEntity.ContentBannerItem, error) {
	return s.repo.GetByGroupID(ctx, groupID)
}

func (s *bannerItemService) IncrementClickCount(ctx context.Context, id uint) error {
	return s.repo.IncrementClickCount(ctx, id)
}
