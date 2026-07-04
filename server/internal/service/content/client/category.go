package client

import (
	"context"

	contentService "NetyAdmin/internal/service/content/admin"
)

// CategoryService 客户端分类服务
// 委托 admin 端实现，仅暴露返回基本类型的方法，不暴露 entity。
type CategoryService interface {
	GetDescendantIDs(ctx context.Context, categoryID uint) ([]uint, error)
}

type categoryService struct {
	adminSvc contentService.CategoryService
}

func NewCategoryService(adminSvc contentService.CategoryService) CategoryService {
	return &categoryService{adminSvc: adminSvc}
}

func (s *categoryService) GetDescendantIDs(ctx context.Context, categoryID uint) ([]uint, error) {
	return s.adminSvc.GetDescendantIDs(ctx, categoryID)
}
