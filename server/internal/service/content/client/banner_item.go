package client

import (
	"context"

	contentRepo "NetyAdmin/internal/repository/content"
)

// BannerClickService 客户端 Banner 点击计数服务（角色命名，避免与 admin 包 BannerItemService 同名）
type BannerClickService interface {
	IncrementClickCount(ctx context.Context, id uint) error
}

type bannerClickService struct {
	repo contentRepo.ContentBannerItemRepository
}

func NewBannerClickService(repo contentRepo.ContentBannerItemRepository) BannerClickService {
	return &bannerClickService{repo: repo}
}

func (s *bannerClickService) IncrementClickCount(ctx context.Context, id uint) error {
	return s.repo.IncrementClickCount(ctx, id)
}
