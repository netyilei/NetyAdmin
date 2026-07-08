package v1

import (
	handler "NetyAdmin/internal/interface/client/http/handler/v1"
	"NetyAdmin/internal/middleware"

	"github.com/gin-gonic/gin"
)

type messageRouter struct {
	handler *handler.MessageHandler
	authMW  *middleware.AuthMiddleware
}

func NewMessageRouter(h *handler.MessageHandler, authMW *middleware.AuthMiddleware) ClientModuleRouter {
	return &messageRouter{handler: h, authMW: authMW}
}

func (r *messageRouter) RegisterPublic(publicGroup *gin.RouterGroup) {}

func (r *messageRouter) RegisterAuth(authGroup *gin.RouterGroup) {
	group := authGroup.Group("/message")
	group.Use(r.authMW.UserJWTAuth())
	{
		group.GET("/internal", r.handler.ListInternalMsgs)
		group.GET("/internal/:id", r.handler.GetInternalMsg)
		group.PUT("/internal/read", r.handler.MarkRead)
		group.PUT("/internal/read-all", r.handler.MarkAllRead)
		group.GET("/internal/unread-count", r.handler.CountUnread)
	}
}
