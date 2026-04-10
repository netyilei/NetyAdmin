package content

import (
	"context"
	"errors"
	"time"

	contentEntity "netyadmin/internal/domain/entity/content"
	contentDto "netyadmin/internal/interface/admin/dto/content"
	"netyadmin/internal/pkg/cache"
	"netyadmin/internal/pkg/configsync"
	"netyadmin/internal/pkg/errorx"
	"netyadmin/internal/pkg/utils"
	contentRepo "netyadmin/internal/repository/content"
	storageService "netyadmin/internal/service/storage"
)

type CategoryService interface {
	Create(ctx context.Context, adminID uint, req *contentDto.CreateContentCategoryDTO) (*contentEntity.ContentCategory, error)
	Update(ctx context.Context, adminID uint, id uint, req *contentDto.UpdateContentCategoryDTO) (*contentEntity.ContentCategory, error)
	Delete(ctx context.Context, id uint) error
	GetByID(ctx context.Context, id uint) (*contentEntity.ContentCategory, error)
	List(ctx context.Context, query *contentDto.ContentCategoryListQueryDTO) ([]*contentEntity.ContentCategory, int64, error)
	GetTree(ctx context.Context, forceRefresh bool) ([]contentDto.ContentCategoryTreeDTO, error)
}

type categoryService struct {
	repo           contentRepo.ContentCategoryRepository
	storageService storageService.ConfigService
	cache          cache.LazyCacheManager
	watcher        configsync.ConfigWatcher
}

func NewCategoryService(repo contentRepo.ContentCategoryRepository, storageService storageService.ConfigService, cache cache.LazyCacheManager, watcher configsync.ConfigWatcher) CategoryService {
	return &categoryService{
		repo:           repo,
		storageService: storageService,
		cache:          cache,
		watcher:        watcher,
	}
}

func (s *categoryService) Create(ctx context.Context, adminID uint, req *contentDto.CreateContentCategoryDTO) (*contentEntity.ContentCategory, error) {
	if req.Code != "" {
		exists, err := s.repo.ExistsByCode(ctx, req.Code)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, errors.New("分类编码已存在")
		}
	}

	contentType := contentEntity.ContentTypeRichText
	if req.ContentType == "plaintext" {
		contentType = contentEntity.ContentTypePlainText
	}

	status := "1"
	if req.Status == "0" {
		status = "0"
	}

	if req.StorageConfigID != nil && *req.StorageConfigID > 0 {
		_, err := s.storageService.GetByID(ctx, *req.StorageConfigID)
		if err != nil {
			return nil, errorx.New(errorx.CodeNotFound, "存储配置不存在")
		}
	}

	category := &contentEntity.ContentCategory{
		ParentID:        req.ParentID,
		Name:            req.Name,
		Code:            req.Code,
		Icon:            req.Icon,
		Sort:            req.Sort,
		StorageConfigID: req.StorageConfigID,
		ContentType:     contentType,
		Status:          status,
		Remark:          req.Remark,
	}
	category.CreatedBy = adminID
	category.UpdatedBy = adminID

	if err := s.repo.Create(ctx, category); err != nil {
		return nil, err
	}

	// 失效树缓存
	_ = s.cache.InvalidateByTags(ctx, cache.TagContentCategoryTree)

	return category, nil
}

func (s *categoryService) Update(ctx context.Context, adminID uint, id uint, req *contentDto.UpdateContentCategoryDTO) (*contentEntity.ContentCategory, error) {
	category, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Code != "" && req.Code != category.Code {
		exists, err := s.repo.ExistsByCode(ctx, req.Code, id)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, errors.New("分类编码已存在")
		}
		category.Code = req.Code
	}

	if req.Name != "" {
		category.Name = req.Name
	}
	if req.Icon != "" {
		category.Icon = req.Icon
	}
	if req.ParentID != category.ParentID {
		// 防止循环引用
		if req.ParentID == id {
			return nil, errors.New("父级分类不能是自己")
		}
		category.ParentID = req.ParentID
	}
	if req.ContentType != "" {
		category.ContentType = contentEntity.ContentTypeRichText
		if req.ContentType == "plaintext" {
			category.ContentType = contentEntity.ContentTypePlainText
		}
	}

	if req.StorageConfigID != nil {
		if *req.StorageConfigID > 0 {
			_, err := s.storageService.GetByID(ctx, *req.StorageConfigID)
			if err != nil {
				return nil, errorx.New(errorx.CodeNotFound, "存储配置不存在")
			}
		}
		category.StorageConfigID = req.StorageConfigID
	}

	if req.Status != "" {
		category.Status = req.Status
	}
	category.Sort = req.Sort
	category.Remark = req.Remark
	category.UpdatedBy = adminID

	if err := s.repo.Update(ctx, category); err != nil {
		return nil, err
	}

	// 失效树缓存
	_ = s.cache.InvalidateByTags(ctx, cache.TagContentCategoryTree)

	return category, nil
}

func (s *categoryService) Delete(ctx context.Context, id uint) error {
	hasChildren, err := s.repo.HasChildren(ctx, id)
	if err != nil {
		return err
	}
	if hasChildren {
		return errors.New("该分类下存在子分类，无法删除")
	}

	hasArticles, err := s.repo.HasArticles(ctx, id)
	if err != nil {
		return err
	}
	if hasArticles {
		return errors.New("该分类下存在文章，无法删除")
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}

	// 失效树缓存
	_ = s.cache.InvalidateByTags(ctx, cache.TagContentCategoryTree)

	return nil
}

func (s *categoryService) GetByID(ctx context.Context, id uint) (*contentEntity.ContentCategory, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *categoryService) List(ctx context.Context, query *contentDto.ContentCategoryListQueryDTO) ([]*contentEntity.ContentCategory, int64, error) {
	repoQuery := &contentRepo.ContentCategoryQuery{
		Current: query.Current,
		Size:    query.Size,
		Name:    query.Name,
		Status:  query.Status,
	}
	return s.repo.List(ctx, repoQuery)
}

func (s *categoryService) GetTree(ctx context.Context, forceRefresh bool) ([]contentDto.ContentCategoryTreeDTO, error) {
	cacheKey := cache.KeyContentCategoryTree()
	var tree []contentDto.ContentCategoryTreeDTO

	loader := func() (interface{}, error) {
		categories, err := s.repo.GetTree(ctx)
		if err != nil {
			return nil, err
		}
		return s.buildTree(categories), nil
	}

	// 如果强制刷新，先失效标签
	if forceRefresh {
		_ = s.cache.InvalidateByTags(ctx, cache.TagContentCategoryTree)
	}

	err := s.cache.Fetch(ctx, cacheKey, "content_category_cache", []string{cache.TagContentCategoryTree}, 1*time.Hour, &tree, loader)
	return tree, err
}

func (s *categoryService) buildTree(categories []*contentEntity.ContentCategory) []contentDto.ContentCategoryTreeDTO {
	return utils.BuildTree(
		categories,
		func(c *contentEntity.ContentCategory) uint { return c.ParentID },
		func(c *contentEntity.ContentCategory) uint { return c.ID },
		func(cat *contentEntity.ContentCategory, children []contentDto.ContentCategoryTreeDTO) (contentDto.ContentCategoryTreeDTO, bool) {
			if children == nil {
				children = make([]contentDto.ContentCategoryTreeDTO, 0)
			}
			return contentDto.ContentCategoryTreeDTO{
				ID:          cat.ID,
				ParentID:    cat.ParentID,
				Name:        cat.Name,
				Code:        cat.Code,
				Icon:        cat.Icon,
				Sort:        cat.Sort,
				ContentType: string(cat.ContentType),
				Status:      cat.Status,
				Children:    children,
			}, true
		},
	)
}
