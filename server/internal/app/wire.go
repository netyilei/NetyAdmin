package app

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mojocn/base64Captcha"
	goRedis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"NetyAdmin/internal/config"
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
	"NetyAdmin/internal/middleware"
	"NetyAdmin/internal/pkg/cache"
	"NetyAdmin/internal/pkg/captcha"
	"NetyAdmin/internal/pkg/configsync"
	"NetyAdmin/internal/pkg/database"
	"NetyAdmin/internal/pkg/jwt"
	"NetyAdmin/internal/pkg/message"
	pkgredis "NetyAdmin/internal/pkg/redis"
	storagePkg "NetyAdmin/internal/pkg/storage"
	"NetyAdmin/internal/pkg/task"

	"NetyAdmin/internal/interface/admin/http/router"
	clientRouter "NetyAdmin/internal/interface/client/http/router"
	"NetyAdmin/internal/job"
	"NetyAdmin/internal/pkg/migration"
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
	contentService "NetyAdmin/internal/service/content"
	dictServicePkg "NetyAdmin/internal/service/dict"
	ipacServicePkg "NetyAdmin/internal/service/ipac"
	logService "NetyAdmin/internal/service/log"
	msgServicePkg "NetyAdmin/internal/service/message"
	openServicePkg "NetyAdmin/internal/service/open_platform"
	storageService "NetyAdmin/internal/service/storage"
	systemService "NetyAdmin/internal/service/system"
	taskServicePkg "NetyAdmin/internal/service/task"
	userServicePkg "NetyAdmin/internal/service/user"
)

func Bootstrap(cfg *config.Config, db *gorm.DB) (*App, error) {
	// 0. DB Migration (Separate startup step, independent of Task Manager)
	if cfg.Migration.Enabled {
		migrator := migration.NewMigrator(db, cfg.Migration.Dir)
		if err := migrator.Run(); err != nil {
			return nil, fmt.Errorf("数据库同步迁移失败: %w", err)
		}
	}

	// 1. DB Health Checker
	dbHealthChecker := database.NewHealthChecker(db,
		database.WithCheckInterval(30*time.Second),
		database.WithRetryInterval(5*time.Second),
		database.WithMaxRetries(5),
		database.WithOnReconnect(func() {
			log.Println("[数据库] 连接已恢复")
		}),
		database.WithOnDisconnect(func(err error) {
			log.Printf("[数据库] 连接断开: %v", err)
		}),
	)

	// 1. Redis & Cache
	redisClient, err := pkgredis.NewClient(&cfg.Redis)
	if err != nil {
		return nil, err
	}

	// 2. JWT
	jwtInstance := jwt.New(cfg.JWT.Secret, cfg.JWT.Expiration)

	// 3. Repositories
	repos := initRepositories(db)

	// 4. Config Sync & Cache Manager
	configWatcher := configsync.NewConfigWatcher(repos.systemConfig, redisClient, &cfg.Redis)
	go configWatcher.WatchBlocking(context.Background())

	lazyCacheMgr, err := cache.NewLazyCacheManager(&cfg.Redis, redisClient, configWatcher)
	if err != nil {
		return nil, err
	}

	// 5. Task Manager
	taskManager := task.NewManager(&cfg.Task, &cfg.Redis, redisClient)

	// 5.1 Captcha Manager
	captchaStore := captcha.NewDualStore(lazyCacheMgr, configWatcher, db)
	captchaMgr := captcha.NewManager(captchaStore)

	// 6. Services & Handlers
	services := initServices(repos, jwtInstance, lazyCacheMgr, taskManager, configWatcher, cfg, captchaStore, redisClient)
	handlers := initHandlers(services, captchaMgr, configWatcher)

	// 7. Router
	router := router.NewRouter(
		handlers.auth,
		handlers.common,
		handlers.admin,
		handlers.system,
		handlers.storage,
		handlers.content.Category,
		handlers.content.Article,
		handlers.content.BannerGroup,
		handlers.content.BannerItem,
		handlers.operationLog,
		handlers.errorLog,
		handlers.route,
		handlers.ipac,
		handlers.app,
		handlers.openApi,
		handlers.openLog,
		handlers.message,
		handlers.dict,
		handlers.task,
		handlers.userAdmin,
		services.ipac,
		services.role,
	)

	cRouter := clientRouter.NewClientRouter(
		handlers.client.echo,
		handlers.client.user,
		handlers.client.auth,
		handlers.client.message,
		services.app,
		services.openApi,
		services.openLog,
		services.ipac,
	)

	// 9. Task Registration
	taskManager.Register(job.AllJobs(
		repos.contentArticle,
		repos.taskLog,
		repos.operationLog,
		repos.errorLog,
		repos.message,
		configWatcher,
	)...)
	taskManager.Register(services.msgSendJob)

	// 9. Engine Setup
	gin.SetMode(cfg.Server.Mode)
	engine := gin.New()

	engine.Use(middleware.RequestID())
	engine.Use(middleware.Recovery(services.errorLog))
	engine.Use(middleware.ErrorLogger(services.errorLog))
	engine.Use(middleware.Timeout(30 * time.Second))
	engine.Use(middleware.Logger())
	engine.Use(middleware.OperationLogger(services.operationLog))

	// 临时关闭标准输出以屏蔽路由注册时的 [GIN-debug] 日志
	gin.DefaultWriter = io.Discard
	router.Register(engine)
	cRouter.Register(engine)
	gin.DefaultWriter = os.Stdout

	return NewApp(cfg, db, engine, dbHealthChecker, taskManager), nil
}

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

type serviceSet struct {
	admin              systemService.AdminService
	role               systemService.RoleService
	menu               systemService.MenuService
	api                systemService.APIService
	button             systemService.ButtonService
	task               taskServicePkg.TaskService
	sysConfig          systemService.ConfigService
	dict               dictServicePkg.DictService
	ipac               ipacServicePkg.IPACService
	app                openServicePkg.AppService
	openApi            openServicePkg.OpenApiService
	openLog            openServicePkg.OpenLogService
	message            msgServicePkg.MessageService
	msgSendJob         task.Task
	user               userServicePkg.UserService
	verification       userServicePkg.VerificationService
	operationLog       logService.OperationService
	errorLog           logService.ErrorService
	storageConfig      storageService.ConfigService
	uploadRecord       storageService.RecordService
	contentCategory    contentService.CategoryService
	contentArticle     contentService.ArticleService
	contentBannerGroup contentService.BannerGroupService
	contentBannerItem  contentService.BannerItemService
}

func initServices(repos *repositorySet, jwtInstance *jwt.JWT, lazyCacheMgr cache.LazyCacheManager, taskManager *task.Manager, configWatcher configsync.ConfigWatcher, cfg *config.Config, captchaStore base64Captcha.Store, redisClient *goRedis.Client) *serviceSet {
	storageMgr := storagePkg.NewManager(storagePkg.NewS3DriverFactory())

	s := &serviceSet{}
	s.admin = systemService.NewAdminService(repos.admin, repos.role, jwtInstance, lazyCacheMgr)
	s.role = systemService.NewRoleService(repos.role, repos.menu, repos.api, repos.button, lazyCacheMgr)
	s.menu = systemService.NewMenuService(repos.menu, repos.button, lazyCacheMgr)
	s.api = systemService.NewAPIService(repos.api, lazyCacheMgr)
	s.button = systemService.NewButtonService(repos.button, lazyCacheMgr)
	s.task = taskServicePkg.NewTaskService(taskManager, repos.taskLog, repos.systemConfig, configWatcher)
	s.sysConfig = systemService.NewConfigService(repos.systemConfig, nil, &cfg.Redis, configWatcher) // Redis client passed later if needed
	s.dict = dictServicePkg.NewDictService(repos.dict, lazyCacheMgr)
	s.ipac = ipacServicePkg.NewIPACService(repos.ipac, lazyCacheMgr)
	s.app = openServicePkg.NewAppService(repos.app, lazyCacheMgr, cfg.Security.AESKey, s.ipac, repos.ipac)
	s.openApi = openServicePkg.NewOpenApiService(repos.openApi, repos.app, repos.app, lazyCacheMgr)
	s.openLog = openServicePkg.NewOpenLogService(repos.openLog)

	// Message Drivers
	configProvider := message.NewDbConfigProvider(repos.systemConfig)
	drivers := make(map[string]message.Driver)
	drivers["sms"] = message.NewMockSmsDriver()
	drivers["email"] = message.NewEmailDriver(message.EmailConfig{
		Host:     cfg.Email.Host,
		Port:     cfg.Email.Port,
		User:     cfg.Email.User,
		Password: cfg.Email.Password,
		From:     cfg.Email.From,
	}, configProvider)

	s.message = msgServicePkg.NewMessageService(repos.message, taskManager, drivers, lazyCacheMgr)
	s.msgSendJob = msgServicePkg.NewMsgSendJob(repos.message, drivers)
	s.verification = userServicePkg.NewVerificationService(lazyCacheMgr, s.message, configWatcher, captchaStore)
	s.user = userServicePkg.NewUserService(repos.user, jwtInstance, s.verification, configWatcher, storageMgr)

	middleware.InitJWT(jwtInstance, repos.user)

	s.operationLog = logService.NewOperationService(repos.operationLog)
	s.errorLog = logService.NewErrorService(repos.errorLog, configWatcher, nil)
	s.storageConfig = storageService.NewConfigService(repos.storageConfig, repos.uploadRecord, storageMgr, lazyCacheMgr, redisClient, &cfg.Redis)
	s.uploadRecord = storageService.NewRecordService(repos.uploadRecord, repos.storageConfig, storageMgr)
	s.contentCategory = contentService.NewCategoryService(repos.contentCategory, s.storageConfig, lazyCacheMgr, configWatcher)
	s.contentArticle = contentService.NewArticleService(repos.contentArticle, repos.contentCategory)
	s.contentBannerGroup = contentService.NewBannerGroupService(repos.contentBannerGroup, s.storageConfig)
	s.contentBannerItem = contentService.NewBannerItemService(repos.contentBannerItem, repos.contentBannerGroup, repos.contentArticle)

	_ = s.storageConfig.LoadAllConfigs(context.Background())

	go storageMgr.WatchConfigChanges(context.Background(), redisClient, pkgredis.ChannelStorageSync(cfg.Redis.Prefix), func(ctx context.Context) error {
		return s.storageConfig.LoadAllConfigs(ctx)
	})

	return s
}

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
	}
}

func initHandlers(services *serviceSet, captchaMgr *captcha.Manager, configWatcher configsync.ConfigWatcher) *handlerSet {
	h := &handlerSet{}
	h.auth = auth.NewAuthHandler(services.admin, captchaMgr, configWatcher)
	h.common = common.NewCommonHandler(captchaMgr, configWatcher)
	h.admin = admin.NewAdminHandler(services.admin)
	h.operationLog = operation_log.NewOperationLogHandler(services.operationLog)
	h.errorLog = error_log.NewErrorLogHandler(services.errorLog)
	h.system = system.NewSystemHandler(services.role, services.menu, services.api, services.button, services.admin, services.sysConfig)
	h.storage = storageHandler.NewStorageHandler(services.storageConfig, services.uploadRecord)
	h.ipac = ipacHandler.NewIPACHandler(services.ipac)
	h.app = openHandler.NewAppHandler(services.app)
	h.openApi = openHandler.NewOpenApiHandler(services.openApi)
	h.openLog = openHandler.NewOpenLogHandler(services.openLog)
	h.message = msgHandler.NewMessageHandler(services.message)
	h.dict = dictHandler.NewDictHandler(services.dict)
	h.task = taskHandler.NewTaskHandler(services.task)
	h.userAdmin = userHandler.NewUserHandler(services.user)
	h.content.Category = content.NewContentCategoryHandler(services.contentCategory)
	h.content.Article = content.NewContentArticleHandler(services.contentArticle)
	h.content.BannerGroup = content.NewContentBannerGroupHandler(services.contentBannerGroup)
	h.content.BannerItem = content.NewContentBannerItemHandler(services.contentBannerItem)
	h.route = route.NewRouteHandler(services.menu, services.admin)

	h.client.echo = clientHandler.NewEchoHandler()
	h.client.user = clientHandler.NewUserHandler(services.user)
	h.client.auth = clientHandler.NewAuthHandler(services.verification, captchaMgr)
	h.client.message = clientHandler.NewMessageHandler(services.message)

	return h
}
