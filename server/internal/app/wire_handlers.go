package app

import (
	"NetyAdmin/internal/interface/admin/http/handler/v1/admin"
	"NetyAdmin/internal/interface/admin/http/handler/v1/auth"
	"NetyAdmin/internal/interface/admin/http/handler/v1/common"
	"NetyAdmin/internal/interface/admin/http/handler/v1/content"
	dictHandler "NetyAdmin/internal/interface/admin/http/handler/v1/dict"
	"NetyAdmin/internal/interface/admin/http/handler/v1/error_log"
	ipacHandler "NetyAdmin/internal/interface/admin/http/handler/v1/ipac"
	msgHandler "NetyAdmin/internal/interface/admin/http/handler/v1/message"
	openHandler "NetyAdmin/internal/interface/admin/http/handler/v1/open_platform"
	"NetyAdmin/internal/interface/admin/http/handler/v1/operation_log"
	"NetyAdmin/internal/interface/admin/http/handler/v1/route"
	storageHandler "NetyAdmin/internal/interface/admin/http/handler/v1/storage"
	"NetyAdmin/internal/interface/admin/http/handler/v1/system"
	taskHandler "NetyAdmin/internal/interface/admin/http/handler/v1/task"
	userHandler "NetyAdmin/internal/interface/admin/http/handler/v1/user"
	clientHandler "NetyAdmin/internal/interface/client/http/handler/v1"
)

type handlerSet struct {
	auth         *auth.AuthHandler
	common       *common.CommonHandler
	admin        *admin.AdminHandler
	operationLog *operation_log.OperationLogHandler
	errorLog     *error_log.ErrorLogHandler
	system       *system.SystemHandler
	storage      *storageHandler.StorageHandler
	ipac         *ipacHandler.IPACHandler
	app          *openHandler.AppHandler
	openApi      *openHandler.OpenApiHandler
	openLog      *openHandler.OpenLogHandler
	message      *msgHandler.MessageHandler
	dict         *dictHandler.DictHandler
	task         *taskHandler.TaskHandler
	userAdmin    *userHandler.UserHandler
	content      struct {
		Category    *content.ContentCategoryHandler
		Article     *content.ContentArticleHandler
		BannerGroup *content.ContentBannerGroupHandler
		BannerItem  *content.ContentBannerItemHandler
	}
	route  *route.RouteHandler
	client struct {
		echo    *clientHandler.EchoHandler
		user    *clientHandler.UserHandler
		auth    *clientHandler.AuthHandler
		message *clientHandler.MessageHandler
		content *clientHandler.ContentHandler
		storage *clientHandler.ClientStorageHandler
	}
}

func initHandlers(services *serviceSet, repos *repositorySet) *handlerSet {
	h := &handlerSet{}
	h.auth = auth.NewAuthHandler(services.admin, services.captcha)
	h.common = common.NewCommonHandler(services.captcha)
	h.admin = admin.NewAdminHandler(services.admin)
	h.operationLog = operation_log.NewOperationLogHandler(services.operationLog)
	h.errorLog = error_log.NewErrorLogHandler(services.errorLog)
	h.system = system.NewSystemHandler(services.role, services.menu, services.api, services.button, services.sysConfig)
	h.storage = storageHandler.NewStorageHandler(services.storageConfig, services.uploadRecord)
	h.ipac = ipacHandler.NewIPACHandler(services.ipac)
	h.app = openHandler.NewAppHandler(services.app)
	h.openApi = openHandler.NewOpenApiHandler(services.openApi)
	h.openLog = openHandler.NewOpenLogHandler(services.openLog)
	h.message = msgHandler.NewMessageHandler(services.message)
	h.dict = dictHandler.NewDictHandler(services.dict)
	h.task = taskHandler.NewTaskHandler(services.task)
	h.userAdmin = userHandler.NewUserHandler(services.userAdmin)
	h.content.Category = content.NewContentCategoryHandler(services.contentCategoryAdmin)
	h.content.Article = content.NewContentArticleHandler(services.contentArticleAdmin)
	h.content.BannerGroup = content.NewContentBannerGroupHandler(services.contentBannerGroupAdmin)
	h.content.BannerItem = content.NewContentBannerItemHandler(services.contentBannerItemAdmin)
	h.route = route.NewRouteHandler(services.menu, services.admin)

	h.client.echo = clientHandler.NewEchoHandler()
	h.client.user = clientHandler.NewUserHandler(services.userClient, services.uploadRecord)
	h.client.auth = clientHandler.NewAuthHandler(services.verification, services.captcha, services.userClient)
	h.client.message = clientHandler.NewMessageHandler(services.message)
	h.client.content = clientHandler.NewContentHandler(services.contentArticleClient, services.contentCategoryClient, services.contentBannerGroupClient, services.contentBannerItemClient)
	h.client.storage = clientHandler.NewClientStorageHandler(services.uploadRecord)

	return h
}
