package v1

import (
	"time"

	"github.com/gin-gonic/gin"

	v1 "NetyAdmin/internal/interface/client/dto/v1"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/response"
)

type EchoHandler struct{}

func NewEchoHandler() *EchoHandler {
	return &EchoHandler{}
}

// Echo 示例接口：原样返回消息并附带 AppID
func (h *EchoHandler) Echo(c *gin.Context) {
	var req v1.EchoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "无效的请求参数")
		return
	}

	appID, _ := c.Get("appID")
	appIDStr := ""
	if appID != nil {
		appIDStr = appID.(string)
	}

	response.Success(c, v1.EchoResponse{
		Message:   req.Message,
		AppID:     appIDStr,
		Timestamp: time.Now().Unix(),
	})
}
