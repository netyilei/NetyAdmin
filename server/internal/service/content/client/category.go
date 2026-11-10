package client

import (
	"context"

	contentService "NetyAdmin/internal/service/content/admin"
)

// CategoryTreeService 客户端分类树服务（角色命名，避免与 admin 包 CategoryService 同名）
// 委托 admin 端实现，仅暴露返回基本类型的方法，不暴露 entity。
type CategoryTreeService interface {
	GetDescendantIDs(ctx context.Context, categoryID uint) ([]uint, error)
}

type categoryTreeService struct {
	adminSvc contentService.CategoryService
}

func NewCategoryTreeService(adminSvc contentService.CategoryService) CategoryTreeService {
	return &categoryTreeService{adminSvc: adminSvc}
}

func (s *categoryTreeService) GetDescendantIDs(ctx context.Context, categoryID uint) ([]uint, error) {
	return s.adminSvc.GetDescendantIDs(ctx, categoryID)
}
