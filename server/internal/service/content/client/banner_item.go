package client

import (
	"context"

	contentRepo "NetyAdmin/internal/repository/content"
)

// BannerItemService 客户端 Banner 项服务（仅暴露点击统计）
type BannerItemService interface {
	IncrementClickCount(ctx context.Context, id uint) error
}

type bannerItemService struct {
	repo contentRepo.ContentBannerItemRepository
}

func NewBannerItemService(repo contentRepo.ContentBannerItemRepository) BannerItemService {
	return &bannerItemService{repo: repo}
}

func (s *bannerItemService) IncrementClickCount(ctx context.Context, id uint) error {
	return s.repo.IncrementClickCount(ctx, id)
}
