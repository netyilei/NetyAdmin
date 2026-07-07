package v1

import (
	"time"

	"github.com/gin-gonic/gin"

	v1 "NetyAdmin/internal/interface/client/dto/v1"
	"NetyAdmin/internal/pkg/auth"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/response"
)

type EchoHandler struct{}

func NewEchoHandler() *EchoHandler {
	return &EchoHandler{}
}

// Echo 示例接口：原样返回消息并附带 AppID
// @Summary      回显测试
// @Description  示例接口，原样返回请求消息并附带AppID
// @Tags         客户端-测试
// @Accept       json
// @Produce      json
// @Param        req body v1.EchoRequest true "回显请求"
// @Success      200 {object} response.Response "操作成功"
// @Router       /client/v1/echo [post]
func (h *EchoHandler) Echo(c *gin.Context) {
	var req v1.EchoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, errorx.CodeInvalidParams, "无效的请求参数")
		return
	}

	// Round 7：从 AppContext 读取 appID（原 c.GetString("appID") 遗留 key 已删除）
	appIDStr := ""
	if val, exists := c.Get("currentAppContext"); exists {
		if appCtx, ok := val.(*auth.AppContext); ok && appCtx != nil {
			appIDStr = appCtx.ID
		}
	}

	response.Success(c, v1.EchoResponse{
		Message:   req.Message,
		AppID:     appIDStr,
		Timestamp: time.Now().Unix(),
	})
}
