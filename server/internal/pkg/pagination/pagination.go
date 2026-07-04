package pagination

import (
	"gorm.io/gorm"
)

// DefaultPageSize 默认分页大小
const DefaultPageSize = 20

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
		q.PageSize = DefaultPageSize
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
			pageSize = DefaultPageSize
		}
		if pageSize > MaxPageSize {
			pageSize = MaxPageSize
		}
		return db.Offset((page - 1) * pageSize).Limit(pageSize)
	}
}

func NormalizeSize(size int) int {
	if size <= 0 {
		return DefaultPageSize
	}
	if size > MaxPageSize {
		return MaxPageSize
	}
	return size
}

// NormalizePagination 规整分页参数，防止 List handler 受恶意大 size 触发 DoS。
//   - current <= 0 → 1
//   - size <= 0 → DefaultPageSize
//   - size > MaxPageSize → MaxPageSize
//
// 返回规整后的 (current, size)，调用方应在 handler 入口先调用本函数再传给 service/repository。
func NormalizePagination(current, size int) (int, int) {
	if current <= 0 {
		current = 1
	}
	size = NormalizeSize(size)
	return current, size
}
