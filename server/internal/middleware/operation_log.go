package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	logEntity "NetyAdmin/internal/domain/entity/log"
	"NetyAdmin/internal/pkg/mask"
	logService "NetyAdmin/internal/service/log"
)

func OperationLogger(logBus logService.LogBusService) gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method
		path := c.Request.URL.Path

		// 仅记录 admin 端操作日志（client 端走 openLog 链路，不需要此处记录）。
		if !strings.HasPrefix(path, "/admin/") {
			c.Next()
			return
		}

		// 过滤掉 GET, OPTIONS, HEAD 请求
		if method == "GET" || method == "OPTIONS" || method == "HEAD" {
			c.Next()
			return
		}

		startTime := time.Now()

		var requestBody []byte
		if c.Request.Body != nil {
			readBody, err := io.ReadAll(c.Request.Body)
			if err != nil {
				slog.Warn("operation_log: read request body failed", "error", err, "path", c.Request.URL.Path)
			} else {
				requestBody = readBody
				c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBody))
			}
		}

		// Round 7：删除 responseWriter 响应体捕获（body 从未被读取，捕获全量响应体有内存开销）。
		// 若未来需要记录响应体，应按需重新引入（如仅在 status >= 400 时捕获）。

		c.Next()

		// 路由级显式标记跳过（替代原硬编码字符串匹配，避免路由改名静默失效）。
		// 必须在 c.Next() 之后检查：SkipOperationLog 是路由组级 marker，
		// 执行顺序在 OperationLogger（全局中间件）之后，只有 c.Next() 返回后它才已执行。
		// 例如 operation-logs 自身的删除路由挂了 SkipOperationLog marker。
		if c.GetBool(skipOperationLogKey) {
			return
		}

		latency := time.Since(startTime)
		statusCode := c.Writer.Status()

		var userIDUint uint = 0
		adminID, exists := c.Get("adminID")
		if exists {
			ok := false
			userIDUint, ok = adminID.(uint)
			if !ok {
				slog.Warn("operation_log: adminID type assertion failed", "type", fmt.Sprintf("%T", adminID))
			}
		}

		action := getActionFromMethod(method)
		resource := getResourceFromPath(path)

		var usernameStr string
		if username, exists := c.Get("username"); exists {
			ok := false
			usernameStr, ok = username.(string)
			if !ok {
				slog.Warn("operation_log: username type assertion failed", "type", fmt.Sprintf("%T", username))
			}
		}

		var detail string
		if len(requestBody) > 0 {
			var jsonBody map[string]interface{}
			if err := json.Unmarshal(requestBody, &jsonBody); err == nil {
				// 构建小写敏感字段 set（大小写不敏感匹配，保留原 JSON key 大小写兼容）
				sensitiveSet := make(map[string]struct{}, len(mask.SensitiveFieldKeys))
				for _, k := range mask.SensitiveFieldKeys {
					sensitiveSet[k] = struct{}{}
				}
				for k := range jsonBody {
					if _, ok := sensitiveSet[strings.ToLower(k)]; ok {
						delete(jsonBody, k)
					}
				}
				if sanitized, err := json.Marshal(jsonBody); err == nil {
					detail = string(sanitized)
				}
			} else {
				detail = string(requestBody)
			}
		}

		metrics := fmt.Sprintf("[%s] %s | status=%d, latency=%vms", method, path, statusCode, latency.Milliseconds())
		if detail != "" {
			detail = metrics + " | " + detail
		} else {
			detail = metrics
		}

		log := &logEntity.Operation{
			AdminID:   userIDUint,
			Username:  usernameStr,
			Action:    action,
			Resource:  resource,
			Detail:    detail,
			IP:        c.ClientIP(),
			UserAgent: c.Request.UserAgent(),
		}

		if err := logBus.Record(c.Request.Context(), log); err != nil {
			slog.Error("记录操作日志失败", "err", err, "path", c.Request.URL.Path)
		}
	}
}

func getActionFromMethod(method string) string {
	switch method {
	case "POST":
		return "create"
	case "PUT", "PATCH":
		return "update"
	case "DELETE":
		return "delete"
	default:
		return method
	}
}

func getResourceFromPath(path string) string {
	apiPath := strings.TrimPrefix(path, "/admin")
	apiPath = strings.TrimPrefix(apiPath, "/v1")
	parts := strings.Split(strings.Trim(apiPath, "/"), "/")

	if len(parts) >= 1 && parts[0] != "" {
		return parts[0]
	}

	return "未知"
}
