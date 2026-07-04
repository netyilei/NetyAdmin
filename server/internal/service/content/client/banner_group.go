package client

import (
	"context"
	"time"

	contentEntity "NetyAdmin/internal/domain/entity/content"
	clientDto "NetyAdmin/internal/interface/client/dto/v1"
	"NetyAdmin/internal/pkg/cache"
	"NetyAdmin/internal/pkg/configsync"
	contentRepo "NetyAdmin/internal/repository/content"
)

// BannerGroupService 客户端 Banner 组服务
type BannerGroupService interface {
	GetByCodeVO(ctx context.Context, code string) (*clientDto.ClientBannerGroupVO, error)
}

type bannerGroupService struct {
	repo    contentRepo.ContentBannerGroupRepository
	cache   cache.LazyCacheManager
	watcher configsync.ConfigWatcher
}

func NewBannerGroupService(repo contentRepo.ContentBannerGroupRepository, cache cache.LazyCacheManager, watcher configsync.ConfigWatcher) BannerGroupService {
	return &bannerGroupService{repo: repo, cache: cache, watcher: watcher}
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

func (s *bannerGroupService) GetByCodeVO(ctx context.Context, code string) (*clientDto.ClientBannerGroupVO, error) {
	cacheKey := cache.KeyContentBannerGroupByCode(code)
	cacheTags := []string{cache.TagContentBanner}
	ttl := s.getBannerCacheTTL()

	var group *contentEntity.ContentBannerGroup
	loader := func() (interface{}, error) {
		return s.repo.GetByCode(ctx, code)
	}

	err := s.cache.Fetch(ctx, cacheKey, "content_banner_cache", cacheTags, ttl, &group, loader)
	if err != nil {
		return nil, err
	}

	return bannerGroupToClientVO(group), nil
}

// bannerGroupToClientVO entity 转换为 client DTO VO；IsInTimeRange() 过滤在 service 内部完成
func bannerGroupToClientVO(g *contentEntity.ContentBannerGroup) *clientDto.ClientBannerGroupVO {
	if g == nil {
		return nil
	}
	banners := make([]clientDto.ClientBannerItemVO, 0, len(g.Banners))
	for _, b := range g.Banners {
		if !b.IsInTimeRange() {
			continue
		}
		banners = append(banners, clientDto.ClientBannerItemVO{
			ID:           b.ID,
			Title:        b.Title,
			Subtitle:     b.Subtitle,
			ImageURL:     b.ImageURL,
			ImageAlt:     b.ImageAlt,
			LinkType:     string(b.LinkType),
			LinkURL:      b.LinkURL,
			Content:      b.Content,
			CustomParams: b.CustomParams,
			Sort:         b.Sort,
		})
	}
	return &clientDto.ClientBannerGroupVO{
		ID:          g.ID,
		Name:        g.Name,
		Code:        g.Code,
		Description: g.Description,
		Position:    g.Position,
		Width:       g.Width,
		Height:      g.Height,
		AutoPlay:    g.AutoPlay,
		Interval:    g.Interval,
		Banners:     banners,
	}
}
