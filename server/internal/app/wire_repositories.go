package app

import (
	"gorm.io/gorm"

	contentRepo "NetyAdmin/internal/repository/content"
	dictRepoPkg "NetyAdmin/internal/repository/dict"
	ipacRepo "NetyAdmin/internal/repository/ipac"
	logRepo "NetyAdmin/internal/repository/log"
	msgRepo "NetyAdmin/internal/repository/message"
	openRepo "NetyAdmin/internal/repository/open_platform"
	storageRepo "NetyAdmin/internal/repository/storage"
	sysRepo "NetyAdmin/internal/repository/system"
	taskRepoPkg "NetyAdmin/internal/repository/task"
	userRepoPkg "NetyAdmin/internal/repository/user"
)

type repositorySet struct {
	systemConfig       sysRepo.ConfigRepository
	admin              sysRepo.AdminRepository
	role               sysRepo.RoleRepository
	menu               sysRepo.MenuRepository
	api                sysRepo.APIRepository
	button             sysRepo.ButtonRepository
	operationLog       *logRepo.OperationRepository
	errorLog           *logRepo.ErrorRepository
	storageConfig      storageRepo.ConfigRepository
	uploadRecord       storageRepo.RecordRepository
	contentCategory    contentRepo.ContentCategoryRepository
	contentArticle     contentRepo.ContentArticleRepository
	contentBannerGroup contentRepo.ContentBannerGroupRepository
	contentBannerItem  contentRepo.ContentBannerItemRepository
	taskLog            taskRepoPkg.TaskLogRepository
	dict               dictRepoPkg.DictRepository
	ipac               ipacRepo.IPACRepository
	app                openRepo.AppRepository
	openApi            openRepo.OpenApiRepository
	openLog            openRepo.OpenLogRepository
	message            msgRepo.MsgRepository
	user               userRepoPkg.UserRepository
}

func initRepositories(db *gorm.DB) *repositorySet {
	return &repositorySet{
		systemConfig:       sysRepo.NewConfigRepository(db),
		admin:              sysRepo.NewAdminRepository(db),
		role:               sysRepo.NewRoleRepository(db),
		menu:               sysRepo.NewMenuRepository(db),
		api:                sysRepo.NewAPIRepository(db),
		button:             sysRepo.NewButtonRepository(db),
		operationLog:       logRepo.NewOperationRepository(db),
		errorLog:           logRepo.NewErrorRepository(db),
		storageConfig:      storageRepo.NewConfigRepository(db),
		uploadRecord:       storageRepo.NewRecordRepository(db),
		contentCategory:    contentRepo.NewContentCategoryRepository(db),
		contentArticle:     contentRepo.NewContentArticleRepository(db),
		contentBannerGroup: contentRepo.NewContentBannerGroupRepository(db),
		contentBannerItem:  contentRepo.NewContentBannerItemRepository(db),
		taskLog:            taskRepoPkg.NewTaskLogRepository(db),
		dict:               dictRepoPkg.NewDictRepository(db),
		ipac:               ipacRepo.NewIPACRepository(db),
		app:                openRepo.NewAppRepository(db),
		openApi:            openRepo.NewOpenApiRepository(db),
		openLog:            openRepo.NewOpenLogRepository(db),
		message:            msgRepo.NewMsgRepository(db),
		user:               userRepoPkg.NewUserRepository(db),
	}
}
