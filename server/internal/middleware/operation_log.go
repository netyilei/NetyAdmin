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
	logService "NetyAdmin/internal/service/log"
)

type responseWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w *responseWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

func OperationLogger(logBus logService.LogBusService) gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method
		path := c.Request.URL.Path

		if !strings.HasPrefix(path, "/admin/") {
			c.Next()
			return
		}

		if (method == "DELETE" && strings.HasPrefix(path, "/admin/v1/operation-logs/")) ||
			(method == "POST" && path == "/admin/v1/operation-logs/batch-delete") {
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
			requestBody, _ = io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBody))
		}

		writer := &responseWriter{
			ResponseWriter: c.Writer,
			body:           bytes.NewBufferString(""),
		}
		c.Writer = writer

		c.Next()

		latency := time.Since(startTime)
		statusCode := c.Writer.Status()

		var userIDUint uint = 0
		adminID, exists := c.Get("adminID")
		if exists {
			userIDUint, _ = adminID.(uint)
		}

		action := getActionFromMethod(method)
		// 2. 准确识别批量删除操作
		if method == "POST" && path == "/admin/v1/operation-logs/batch-delete" {
			action = "batch_delete"
		}

		resource := getResourceFromPath(path)

		var usernameStr string
		if username, exists := c.Get("username"); exists {
			usernameStr, _ = username.(string)
		}

		var detail string
		if len(requestBody) > 0 {
			var jsonBody map[string]interface{}
			if err := json.Unmarshal(requestBody, &jsonBody); err == nil {
				delete(jsonBody, "password")
				delete(jsonBody, "oldPassword")
				delete(jsonBody, "newPassword")
				delete(jsonBody, "confirmPassword")
				delete(jsonBody, "secretKey")
				delete(jsonBody, "accessKeySecret")
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
