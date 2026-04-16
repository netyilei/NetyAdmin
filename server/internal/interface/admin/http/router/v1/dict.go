package v1

import (
	"github.com/gin-gonic/gin"

	dictHandler "NetyAdmin/internal/interface/admin/http/handler/v1/dict"
)

type DictRouter struct {
	dict *dictHandler.DictHandler
}

func NewDictRouter(dict *dictHandler.DictHandler) *DictRouter {
	return &DictRouter{dict: dict}
}

func (r *DictRouter) RegisterPublic(group *gin.RouterGroup) {
	// 字典数据获取通常也是公开的 (例如前端根据 code 获取枚举显示)
	group.GET("/system/dict/data/:code", r.dict.GetDictData)
}

func (r *DictRouter) RegisterAuth(group *gin.RouterGroup) {}

func (r *DictRouter) RegisterPermission(group *gin.RouterGroup) {
	dictGroup := group.Group("/system/dict")
	{
		dictGroup.GET("/types", r.dict.ListType)
		dictGroup.POST("/types", r.dict.CreateType)
		dictGroup.PUT("/types", r.dict.UpdateType)
		dictGroup.DELETE("/types/:id", r.dict.DeleteType)

		dictGroup.GET("/data", r.dict.ListDataFull)
		dictGroup.POST("/data", r.dict.CreateData)
		dictGroup.PUT("/data", r.dict.UpdateData)
		dictGroup.DELETE("/data/:id", r.dict.DeleteData)
	}
}
