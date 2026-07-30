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
	"NetyAdmin/internal/pkg/utils"
	contentRepo "NetyAdmin/internal/repository/content"
	storageService "NetyAdmin/internal/service/storage"
)

type CategoryService interface {
	Create(ctx context.Context, adminID uint, req *contentDto.CreateContentCategoryDTO) (*contentEntity.ContentCategory, error)
	Update(ctx context.Context, adminID uint, id uint, req *contentDto.UpdateContentCategoryDTO) (*contentEntity.ContentCategory, error)
	Delete(ctx context.Context, id uint) error
	GetByID(ctx context.Context, id uint) (*contentEntity.ContentCategory, error)
	List(ctx context.Context, query *contentDto.ContentCategoryListQueryDTO) ([]*contentEntity.ContentCategory, int64, error)
	GetTree(ctx context.Context, forceRefresh bool) ([]contentDto.ContentCategoryTreeDTO, error)
	GetDescendantIDs(ctx context.Context, categoryID uint) ([]uint, error)
}

type categoryService struct {
	repo           contentRepo.ContentCategoryRepository
	articleRepo    contentRepo.ContentArticleRepository
	storageService storageService.ConfigService
	cache          cache.ConfigCache
	watcher        configsync.ConfigWatcher
	tm             database.TxManager
}

func NewCategoryService(repo contentRepo.ContentCategoryRepository, articleRepo contentRepo.ContentArticleRepository, storageService storageService.ConfigService, cache cache.ConfigCache, watcher configsync.ConfigWatcher, tm database.TxManager) CategoryService {
	return &categoryService{
		repo:           repo,
		articleRepo:    articleRepo,
		storageService: storageService,
		cache:          cache,
		watcher:        watcher,
		tm:             tm,
	}
}

func (s *categoryService) getCategoryCacheTTL() time.Duration {
	val, ok := s.watcher.GetConfig(cache.ConfigGroupContentCache, cache.ConfigKeyCategoryCacheTTL)
	if ok {
		if mins, err := time.ParseDuration(val + "m"); err == nil {
			return mins
		}
	}
	return 60 * time.Minute
}

func (s *categoryService) Create(ctx context.Context, adminID uint, req *contentDto.CreateContentCategoryDTO) (*contentEntity.ContentCategory, error) {
	if req.Code != "" {
		exists, err := s.repo.ExistsByCode(ctx, req.Code)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, errorx.New(errorx.CodeAlreadyExists, "分类编码已存在")
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
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errorx.New(errorx.CodeNotFound, "存储配置不存在")
			}
			slog.Error("storageService.GetByID failed", "storageConfigID", *req.StorageConfigID, "err", err)
			return nil, fmt.Errorf("storageService.GetByID: %w", err)
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
	if err := s.cache.InvalidateByTags(ctx, cache.TagContentCategoryTree); err != nil {
		slog.Error("invalidate cache failed", "tag", cache.TagContentCategoryTree, "err", err)
	}

	return category, nil
}

func (s *categoryService) Update(ctx context.Context, adminID uint, id uint, req *contentDto.UpdateContentCategoryDTO) (*contentEntity.ContentCategory, error) {
	category, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.New(errorx.CodeNotFound, "分类不存在")
		}
		slog.Error("repo.GetByID failed", "categoryID", id, "err", err)
		return nil, fmt.Errorf("repo.GetByID: %w", err)
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
			return nil, errorx.New(errorx.CodeBadRequest, "父级分类不能是自己")
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
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return nil, errorx.New(errorx.CodeNotFound, "存储配置不存在")
				}
				slog.Error("storageService.GetByID failed", "storageConfigID", *req.StorageConfigID, "err", err)
				return nil, fmt.Errorf("storageService.GetByID: %w", err)
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
	if err := s.cache.InvalidateByTags(ctx, cache.TagContentCategoryTree); err != nil {
		slog.Error("invalidate cache failed", "tag", cache.TagContentCategoryTree, "err", err)
	}

	return category, nil
}

func (s *categoryService) Delete(ctx context.Context, id uint) error {
	// TM 单事务原子完成「前置校验 + 硬删除」。
	// 避免 TOCTOU 竞态：检查与删除之间被并发请求插入文章或子分类。
	txCtx, tx := s.tm.Begin(ctx)

	// 前置校验：有文章则拒绝
	articleCount, err := s.articleRepo.CountByCategory(txCtx, id)
	if err != nil {
		slog.Error("category delete: count articles failed", "categoryID", id, "err", err)
		s.tm.Rollback(tx)
		return errorx.New(errorx.CodeInternalError, "查询分类下文章失败")
	}
	if articleCount > 0 {
		s.tm.Rollback(tx)
		return errorx.New(errorx.CodeCategoryHasArticles)
	}

	// 前置校验：有子分类则拒绝
	childrenCount, err := s.repo.CountChildren(txCtx, id)
	if err != nil {
		slog.Error("category delete: count children failed", "categoryID", id, "err", err)
		s.tm.Rollback(tx)
		return errorx.New(errorx.CodeInternalError, "查询子分类失败")
	}
	if childrenCount > 0 {
		s.tm.Rollback(tx)
		return errorx.New(errorx.CodeCategoryHasChildren)
	}

	// 执行硬删除
	if err := s.repo.Delete(txCtx, id); err != nil {
		slog.Error("category delete: delete failed", "categoryID", id, "err", err)
		s.tm.Rollback(tx)
		return errorx.New(errorx.CodeInternalError, "分类删除失败")
	}

	if err := s.tm.Commit(tx); err != nil {
		slog.Error("category delete: commit failed", "categoryID", id, "err", err)
		return errorx.New(errorx.CodeInternalError, "事务提交失败")
	}

	// 失效树缓存
	if err := s.cache.InvalidateByTags(ctx, cache.TagContentCategoryTree); err != nil {
		slog.Error("invalidate cache failed", "tag", cache.TagContentCategoryTree, "err", err)
	}
	return nil
}

func (s *categoryService) GetByID(ctx context.Context, id uint) (*contentEntity.ContentCategory, error) {
	category, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.New(errorx.CodeNotFound, "分类不存在")
		}
		slog.Error("repo.GetByID failed", "categoryID", id, "err", err)
		return nil, fmt.Errorf("repo.GetByID: %w", err)
	}
	return category, nil
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
		if err := s.cache.InvalidateByTags(ctx, cache.TagContentCategoryTree); err != nil {
			slog.Error("invalidate cache failed", "tag", cache.TagContentCategoryTree, "err", err)
		}
	}

	err := s.cache.FetchFast(ctx, cacheKey, "content_category_cache", []string{cache.TagContentCategoryTree}, s.getCategoryCacheTTL(), &tree, loader)
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

func (s *categoryService) GetDescendantIDs(ctx context.Context, categoryID uint) ([]uint, error) {
	tree, err := s.GetTree(ctx, false)
	if err != nil {
		return nil, err
	}

	ids := []uint{categoryID}
	findAndCollect(tree, categoryID, &ids)
	return ids, nil
}

func findAndCollect(tree []contentDto.ContentCategoryTreeDTO, targetID uint, ids *[]uint) {
	for i := range tree {
		if tree[i].ID == targetID {
			collectChildIDs(tree[i].Children, ids)
			return
		}
		if len(tree[i].Children) > 0 {
			findAndCollect(tree[i].Children, targetID, ids)
		}
	}
}

func collectChildIDs(children []contentDto.ContentCategoryTreeDTO, ids *[]uint) {
	for i := range children {
		*ids = append(*ids, children[i].ID)
		if len(children[i].Children) > 0 {
			collectChildIDs(children[i].Children, ids)
		}
	}
}
