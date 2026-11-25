// page.go：列表分页契约（api-style §2——page 1 起 / page_size 默认 20 上限 100；
// 响应 data = {list,total,page,page_size}，空集合返回 10000 + 空 list）。
// 非法 query 取默认值（宽松解析——分页参数不构成安全面，上限钳制兜底）。
package transport

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

// 分页边界（api-style §2 权威值）。
const (
	pageDefaultSize = 20
	pageMaxSize     = 100
	pageMinNumber   = 1
)

// Page 是列表响应信封的 data 结构（字段名 api-style 权威；snake_case 协议元字段）。
type Page struct {
	List     any   `json:"list"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
}

// BindPage 解析 query 分页参数（page/page_size）并钳制到合法区间。
func BindPage(c *gin.Context) (page, size int) {
	page = atoiDefault(c.Query("page"), pageMinNumber)
	size = atoiDefault(c.Query("page_size"), pageDefaultSize)
	if page < pageMinNumber {
		page = pageMinNumber
	}
	if size < 1 {
		size = pageDefaultSize
	}
	if size > pageMaxSize {
		size = pageMaxSize
	}
	return page, size
}

// Offset 分页转 SQL offset（page 从 1 起）。
func Offset(page, size int) int {
	return (page - 1) * size
}

// NewPage 组装列表响应 data。
func NewPage(list any, total int64, page, size int) Page {
	return Page{List: list, Total: total, Page: page, PageSize: size}
}

// atoiDefault 解析失败返回默认（含空串/非数字；负数由调用方钳制）。
func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}
