// versioning.go：接入面版本化与弃用机制（TR-04；api-style §1/）。
// 版本路由：RegisterVersioned 按前缀（/admin/v1 等）注册并回写 API-Version 头；
// 弃用三步（公告→双发窗口→major 删除）：DeprecatedRoute 标记 → 响应头
// Deprecation/Sunset + Link替代（RFC 859445 口径）；OpenAPI spec 生成随
// 首个带注解端点落地（check-openapi 门禁已备，登记）。
package transport

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// DeprecationMeta 是弃用路由元数据（api-style §1 弃用三步）。
type DeprecationMeta struct {
	// DeprecatedSince 是弃用公告版本（如 "v1.2"；公告步）。
	DeprecatedSince string
	// Sunset 是删除日期（双发窗口终点；RFC 8594 HTTP-date）。
	Sunset time.Time
	// Successor 是替代端点路径（RFC 9745 Link rel="successor-version"）。
	Successor string
}

// VersionedRoute 是版本化注册项。
type VersionedRoute struct {
	Method     string
	Path       string // 全路径（含版本前缀）
	Module     string
	Handler    gin.HandlerFunc
	Deprecated *DeprecationMeta // nil = 正常路由
}

// RegisterVersioned 注册路由并附加版本/弃用响应头（header 回写在业务 handler
// 前置——统一面，业务零感知）。
func (s *Server) RegisterVersioned(r VersionedRoute) error {
	headers := versionHeaders(r.Path)
	if r.Deprecated != nil {
		headers = append(headers, deprecationHeaders(*r.Deprecated)...)
	}
	wrapped := func(c *gin.Context) {
		for i := 0; i+1 < len(headers); i += 2 {
			c.Header(headers[i], headers[i+1])
		}
		r.Handler(c)
	}
	return s.Register(r.Method, r.Path, r.Module, wrapped)
}

// versionHeaders 回写 API-Version（路径 /<surface>/v<N>/... 提取；不匹配不写）。
func versionHeaders(path string) []string {
	for i := 0; i+2 < len(path); i++ {
		if path[i] != '/' || path[i+1] != 'v' {
			continue
		}
		j := i + 2
		for j < len(path) && path[j] >= '0' && path[j] <= '9' && j-i-2 < 2 {
			j++
		}
		if j == i+2 || (j < len(path) && path[j] != '/') {
			continue
		}
		return []string{"API-Version", path[i+2 : j]}
	}
	return nil
}

// deprecationHeaders 组装 RFC 8594（Sunset）45（Link successor）+ 自有
// Deprecation 头（api-style §1 弃用三步机器可读化）。
func deprecationHeaders(d DeprecationMeta) []string {
	h := []string{"Deprecation", d.DeprecatedSince}
	if !d.Sunset.IsZero() {
		h = append(h, "Sunset", d.Sunset.UTC().Format(http.TimeFormat))
	}
	if d.Successor != "" {
		h = append(h, "Link", `<`+d.Successor+`>; rel="successor-version"`)
	}
	return h
}
