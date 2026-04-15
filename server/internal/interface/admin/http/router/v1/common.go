package v1

import (
	"NetyAdmin/internal/interface/admin/http/handler/v1/common"

	"github.com/gin-gonic/gin"
)

type CommonRouter struct {
	handler *common.CommonHandler
}

func NewCommonRouter(handler *common.CommonHandler) *CommonRouter {
	return &CommonRouter{handler: handler}
}

func (r *CommonRouter) RegisterPublic(group *gin.RouterGroup) {
	group.GET("/common/captcha", r.handler.GetCaptcha)
}

func (r *CommonRouter) RegisterAuth(group *gin.RouterGroup)       {}
func (r *CommonRouter) RegisterPermission(group *gin.RouterGroup) {}
