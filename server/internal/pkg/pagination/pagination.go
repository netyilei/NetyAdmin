package pagination

import (
	"NetyAdmin/internal/domain/entity"

	"gorm.io/gorm"
)

// MaxPageSize 分页大小上限，防止 DoS
const MaxPageSize = 100

type Query struct {
	Page     int `form:"page" json:"page"`
	PageSize int `form:"pageSize" json:"pageSize"`
}

func (q *Query) Normalize() {
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.PageSize <= 0 {
		q.PageSize = entity.DefaultPageSize
	}
	if q.PageSize > MaxPageSize {
		q.PageSize = MaxPageSize
	}
}

func (q *Query) Offset() int {
	return (q.Page - 1) * q.PageSize
}

func Paginate(page, pageSize int) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if page <= 0 {
			page = 1
		}
		if pageSize <= 0 {
			pageSize = entity.DefaultPageSize
		}
		if pageSize > MaxPageSize {
			pageSize = MaxPageSize
		}
		return db.Offset((page - 1) * pageSize).Limit(pageSize)
	}
}

func NormalizeSize(size int) int {
	if size <= 0 {
		return entity.DefaultPageSize
	}
	if size > MaxPageSize {
		return MaxPageSize
	}
	return size
}
