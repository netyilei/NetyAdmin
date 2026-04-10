package response

import (
	"fmt"
	"net/http"

	"netyadmin/internal/pkg/errorx"

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
		Code:      "200",
		Message:   "",
		Data:      data,
		RequestID: c.GetString("requestID"),
	})
}

func SuccessWithMsg(c *gin.Context, message string, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:      "200",
		Message:   "",
		Data:      data,
		RequestID: c.GetString("requestID"),
	})
}

func SuccessWithPage(c *gin.Context, current, size int, total int64, list interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    "200",
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

func Fail(c *gin.Context, err error) {
	_ = c.Error(err) // 注册错误到 Gin 上下文
	if bizErr, ok := err.(*errorx.BizError); ok {
		c.JSON(http.StatusOK, Response{
			Code:      bizErr.Code.String(),
			Message:   "",
			RequestID: c.GetString("requestID"),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:      errorx.CodeInternalError.String(),
		Message:   "",
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
		Message:   "",
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
		Message:   "",
		RequestID: c.GetString("requestID"),
	})
}
