package response

import (
	"errors"
	"fmt"
	"net/http"

	"NetyAdmin/internal/pkg/errorx"

	"github.com/gin-gonic/gin"
)

type Response struct {
	Code      interface{} `json:"code"`
	Message   string      `json:"msg"`
	Data      interface{} `json:"data,omitempty"`
	RequestID string      `json:"request_id,omitempty"`
}

type PageData struct {
	Records interface{} `json:"records"`
	Current int         `json:"current"`
	Size    int         `json:"size"`
	Total   int64       `json:"total"`
}

func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:      errorx.CodeSuccess.String(),
		Message:   "",
		Data:      data,
		RequestID: c.GetString("requestID"),
	})
}

func SuccessWithMsg(c *gin.Context, message string, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:      errorx.CodeSuccess.String(),
		Message:   message,
		Data:      data,
		RequestID: c.GetString("requestID"),
	})
}

func SuccessWithPage(c *gin.Context, current, size int, total int64, list interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    errorx.CodeSuccess.String(),
		Message: "",
		Data: PageData{
			Records: list,
			Current: current,
			Size:    size,
			Total:   total,
		},
		RequestID: c.GetString("requestID"),
	})
}

// Fail 统一错误响应。
//
// Sentry 错误链保留（SubTask 7.5）：
//   - c.Error(err) 注册原始 error 对象（非字符串）到 Gin 上下文 c.Errors，
//     供后续 ErrorLogger 中间件统一写入 DB error_log 表并上报 Sentry
//   - ErrorLogger 通过 hub.CaptureException(err.Err) 上报，传入完整 error 对象
//   - Sentry SDK 自动遍历 error.Unwrap() 链路，构建 linked exceptions 用于聚合
//   - BizError.Unwrap()（SubTask 7.3）返回 Repository/第三方库原始错误，
//     使 Sentry 可穿透 BizError 包装层定位底层根因（如 gorm.ErrRecordNotFound）
//   - Service 层 fmt.Errorf("repo.Xxx: %w", err) 包装的错误链同样被保留
func Fail(c *gin.Context, err error) {
	_ = c.Error(err) // 注册原始 error（保留完整错误链）到 Gin 上下文，供 ErrorLogger 上报 Sentry
	var bizErr *errorx.BizError
	if errors.As(err, &bizErr) {
		c.JSON(http.StatusOK, Response{
			Code:      bizErr.Code.String(),
			Message:   bizErr.Message,
			RequestID: c.GetString("requestID"),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:      errorx.CodeInternalError.String(),
		Message:   errorx.CodeInternalError.Message(),
		RequestID: c.GetString("requestID"),
	})
}

func FailWithCode(c *gin.Context, code errorx.Code, message ...string) {
	msg := code.Message()
	if len(message) > 0 && message[0] != "" {
		msg = message[0]
	}
	_ = c.Error(fmt.Errorf("%s: %s", code.String(), msg)) // 注册错误
	c.JSON(http.StatusOK, Response{
		Code:      code.String(),
		Message:   msg,
		RequestID: c.GetString("requestID"),
	})
}

func FailWithStatus(c *gin.Context, httpStatus int, code errorx.Code, message ...string) {
	msg := code.Message()
	if len(message) > 0 && message[0] != "" {
		msg = message[0]
	}
	_ = c.Error(fmt.Errorf("%s: %s", code.String(), msg)) // 注册错误
	c.JSON(httpStatus, Response{
		Code:      code.String(),
		Message:   msg,
		RequestID: c.GetString("requestID"),
	})
}
