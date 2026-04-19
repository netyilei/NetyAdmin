package v1

import (
	"github.com/gin-gonic/gin"

	msgHandler "NetyAdmin/internal/interface/admin/http/handler/v1/message"
)

type MessageRouter struct {
	msg *msgHandler.MessageHandler
}

func NewMessageRouter(msg *msgHandler.MessageHandler) *MessageRouter {
	return &MessageRouter{msg: msg}
}

func (r *MessageRouter) RegisterPublic(group *gin.RouterGroup) {}

func (r *MessageRouter) RegisterAuth(group *gin.RouterGroup) {}

func (r *MessageRouter) RegisterPermission(group *gin.RouterGroup) {
	msgGroup := group.Group("/message")
	{
		msgGroup.GET("/templates", r.msg.ListTemplates)
		msgGroup.POST("/templates", r.msg.CreateTemplate)
		msgGroup.PUT("/templates", r.msg.UpdateTemplate)
		msgGroup.DELETE("/templates/:id", r.msg.DeleteTemplate)
		msgGroup.GET("/records", r.msg.ListRecords)
		msgGroup.POST("/records/:id/retry", r.msg.RetryRecord)
		msgGroup.POST("/send", r.msg.SendDirect)
	}
}
