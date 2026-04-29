package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mojocn/base64Captcha"
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
	msgPkg "NetyAdmin/internal/pkg/message"
	"NetyAdmin/internal/pkg/pubsub"
	pkgredis "NetyAdmin/internal/pkg/redis"
	storagePkg "NetyAdmin/internal/pkg/storage"
	"NetyAdmin/internal/pkg/task"

	logEntity "NetyAdmin/internal/domain/entity/log"
	openEntity "NetyAdmin/internal/domain/entity/open_platform"
	taskEntity "NetyAdmin/internal/domain/entity/task"
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

	// 4. PubSubBus
	var eventBus pubsub.EventBus
	switch cfg.Bus.Driver {
	case "memory":
		eventBus = pubsub.NewMemoryDriver()
	case "redis":
		if redisClient == nil {
			return nil, fmt.Errorf("bus driver 设置为 redis 但 Redis 未启用")
		}
		eventBus = pubsub.NewRedisDriver(redisClient, cfg.Redis.Prefix)
	default:
		if cfg.Redis.Enabled && redisClient != nil {
			eventBus = pubsub.NewRedisDriver(redisClient, cfg.Redis.Prefix)
		} else {
			eventBus = pubsub.NewMemoryDriver()
		}
	}

	// 5. Config Sync & Cache Manager
	configWatcher := configsync.NewConfigWatcher(repos.systemConfig)

	lazyCacheMgr, err := cache.NewLazyCacheManager(&cfg.Redis, redisClient, configWatcher)
	if err != nil {
		return nil, err
	}

	lazyCacheMgr.SetEventBus(eventBus)

	// 5. Task Manager
	taskManager := task.NewManager(&cfg.Task, &cfg.Redis, redisClient)

	// 5.1 Captcha Manager
	captchaStore := captcha.NewDualStore(lazyCacheMgr, configWatcher, db)
	captchaMgr := captcha.NewManager(captchaStore)

	// 6. Services & Handlers
	services := initServices(repos, jwtInstance, lazyCacheMgr, taskManager, configWatcher, cfg, captchaStore, eventBus)
	handlers := initHandlers(services, captchaMgr, configWatcher, repos, lazyCacheMgr)

	// 7. Register PubSubBus subscribers
	// ConfigSync
	_ = eventBus.Subscribe(pubsub.TopicConfigSync, func(msg []byte) {
		_ = configWatcher.ForceReload(context.Background())
	})

	// StorageSync
	_ = eventBus.Subscribe(pubsub.TopicStorageSync, func(msg []byte) {
		_ = services.storageConfig.LoadAllConfigs(context.Background())
	})

	// CacheInvalidation
	_ = eventBus.Subscribe(pubsub.TopicCacheInvalidation, func(msg []byte) {
		var tags []string
		if err := json.Unmarshal(msg, &tags); err == nil {
			_ = lazyCacheMgr.InvalidateL1ByTags(context.Background(), tags...)
		}
	})

	// IPACReload
	_ = eventBus.Subscribe(pubsub.TopicIPACReload, func(msg []byte) {
		_ = services.ipac.ReloadCache(context.Background())
	})

	// 8. Router
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

	// 9. Task Registration
	taskManager.Register(job.AllJobs(
		repos.contentArticle,
		repos.taskLog,
		repos.operationLog,
		repos.errorLog,
		repos.message,
		repos.openLog,
		configWatcher,
	)...)
	taskManager.Register(services.msgSendJob)

	cRouter := clientRouter.NewClientRouter(
		handlers.client.echo,
		handlers.client.user,
		handlers.client.auth,
		handlers.client.message,
		handlers.client.content,
		handlers.client.storage,
		services.app,
		services.openApi,
		services.openLog,
		services.ipac,
	)

	// 10. Engine Setup
	gin.SetMode(cfg.Server.Mode)
	engine := gin.New()

	engine.Use(middleware.RequestID())
	engine.Use(middleware.Recovery(services.errorLog))
	engine.Use(middleware.ErrorLogger(services.errorLog))
	engine.Use(middleware.Timeout(120 * time.Second))
	engine.Use(middleware.Logger())
	engine.Use(middleware.OperationLogger(services.logBus))

	// 临时关闭标准输出以屏蔽路由注册时的 [GIN-debug] 日志
	gin.DefaultWriter = io.Discard
	router.Register(engine)
	cRouter.Register(engine)
	gin.DefaultWriter = os.Stdout

	return NewApp(cfg, db, engine, dbHealthChecker, taskManager, services.logBus, eventBus), nil
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
	emailDriver        msgPkg.Driver
	logBus             logService.LogBusService
}

func initServices(repos *repositorySet, jwtInstance *jwt.JWT, lazyCacheMgr cache.LazyCacheManager, taskManager *task.Manager, configWatcher configsync.ConfigWatcher, cfg *config.Config, captchaStore base64Captcha.Store, eventBus pubsub.EventBus) *serviceSet {
	storageMgr := storagePkg.NewManager(storagePkg.NewS3DriverFactory())

	s := &serviceSet{}
	s.admin = systemService.NewAdminService(repos.admin, repos.role, jwtInstance, lazyCacheMgr)
	s.role = systemService.NewRoleService(repos.role, repos.menu, repos.api, repos.button, lazyCacheMgr)
	s.menu = systemService.NewMenuService(repos.menu, repos.button, lazyCacheMgr)
	s.api = systemService.NewAPIService(repos.api, lazyCacheMgr)
	s.button = systemService.NewButtonService(repos.button, lazyCacheMgr)
	s.task = taskServicePkg.NewTaskService(taskManager, repos.taskLog, repos.systemConfig, configWatcher, func(ctx context.Context, logRecord *taskEntity.TaskLog) error {
		return s.logBus.Record(ctx, logRecord)
	})
	s.sysConfig = systemService.NewConfigService(repos.systemConfig, configWatcher, eventBus)
	s.dict = dictServicePkg.NewDictService(repos.dict, lazyCacheMgr)
	s.ipac = ipacServicePkg.NewIPACService(repos.ipac, eventBus)
	s.app = openServicePkg.NewAppService(repos.app, lazyCacheMgr, cfg.Security.AESKey, s.ipac, repos.ipac, storageMgr, configWatcher)
	s.openApi = openServicePkg.NewOpenApiService(repos.openApi, repos.app, lazyCacheMgr)
	s.openLog = openServicePkg.NewOpenLogService(repos.openLog, func(ctx context.Context, logRecord *openEntity.OpenPlatformLog) error {
		return s.logBus.Record(ctx, logRecord)
	})

	// Message Drivers
	configProvider := msgPkg.NewWatcherConfigProvider(configWatcher)
	drivers := make(map[string]msgPkg.Driver)
	drivers["sms"] = msgPkg.NewMockSmsDriver()
	drivers["email"] = msgPkg.NewEmailDriver(msgPkg.EmailConfig{
		Host:           cfg.Email.Host,
		Port:           cfg.Email.Port,
		User:           cfg.Email.User,
		Password:       cfg.Email.Password,
		From:           cfg.Email.From,
		SSL:            cfg.Email.SSL,
		StartTLS:       cfg.Email.StartTLS,
		AuthType:       cfg.Email.AuthType,
		ConnectTimeout: cfg.Email.ConnectTimeout,
		SendTimeout:    cfg.Email.SendTimeout,
	}, configProvider)
	s.emailDriver = drivers["email"]

	s.message = msgServicePkg.NewMessageService(repos.message, taskManager, drivers, lazyCacheMgr)
	s.msgSendJob = msgServicePkg.NewMsgSendJob(repos.message, drivers, configWatcher)
	s.verification = userServicePkg.NewVerificationService(lazyCacheMgr, s.message, configWatcher, captchaStore)
	tokenStore := userServicePkg.NewTokenStoreFromConfig(configWatcher, repos.user, lazyCacheMgr)
	s.user = userServicePkg.NewUserService(repos.user, jwtInstance, s.verification, configWatcher, storageMgr, captchaStore, tokenStore, lazyCacheMgr)

	middleware.InitJWT(jwtInstance, repos.user, tokenStore)

	writers := map[logEntity.LogType]logService.LogBatchWriter{
		logEntity.LogTypeOperation: logService.NewOperationLogWriter(repos.operationLog),
		logEntity.LogTypeError:     logService.NewErrorLogWriter(repos.errorLog),
		logEntity.LogTypeOpen:      logService.NewOpenLogWriter(repos.openLog),
		logEntity.LogTypeTask:      logService.NewTaskLogWriter(repos.taskLog),
	}

	configs := map[logEntity.LogType]logService.BucketConfig{
		logEntity.LogTypeOperation: {Priority: logEntity.PriorityP1},
		logEntity.LogTypeError:     {Priority: logEntity.PriorityP0},
		logEntity.LogTypeOpen:      {Priority: logEntity.PriorityP2},
		logEntity.LogTypeTask:      {Priority: logEntity.PriorityP2},
	}

	s.logBus = logService.NewLogBusService(writers, configs, configWatcher)

	s.operationLog = logService.NewOperationService(repos.operationLog)
	s.errorLog = logService.NewErrorService(repos.errorLog, configWatcher, lazyCacheMgr, s.logBus)
	s.storageConfig = storageService.NewConfigService(repos.storageConfig, repos.uploadRecord, storageMgr, lazyCacheMgr, eventBus)
	s.uploadRecord = storageService.NewRecordService(repos.uploadRecord, s.storageConfig, storageMgr, s.app)
	s.contentCategory = contentService.NewCategoryService(repos.contentCategory, s.storageConfig, lazyCacheMgr, configWatcher)
	s.contentArticle = contentService.NewArticleService(repos.contentArticle, repos.contentCategory, lazyCacheMgr, configWatcher)
	s.contentBannerGroup = contentService.NewBannerGroupService(repos.contentBannerGroup, s.storageConfig, lazyCacheMgr, configWatcher)
	s.contentBannerItem = contentService.NewBannerItemService(repos.contentBannerItem, repos.contentBannerGroup, repos.contentArticle, lazyCacheMgr)

	_ = s.storageConfig.LoadAllConfigs(context.Background())

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
		content *clientHandler.ContentHandler
		storage *clientHandler.ClientStorageHandler
	}
}

func initHandlers(services *serviceSet, captchaMgr *captcha.Manager, configWatcher configsync.ConfigWatcher, repos *repositorySet, lazyCacheMgr cache.LazyCacheManager) *handlerSet {
	h := &handlerSet{}
	h.auth = auth.NewAuthHandler(services.admin, captchaMgr, configWatcher)
	h.common = common.NewCommonHandler(captchaMgr, configWatcher)
	h.admin = admin.NewAdminHandler(services.admin)
	h.operationLog = operation_log.NewOperationLogHandler(services.operationLog)
	h.errorLog = error_log.NewErrorLogHandler(services.errorLog)
	h.system = system.NewSystemHandler(services.role, services.menu, services.api, services.button, services.sysConfig, services.emailDriver)
	h.storage = storageHandler.NewStorageHandler(services.storageConfig, services.uploadRecord)
	h.ipac = ipacHandler.NewIPACHandler(services.ipac)
	h.app = openHandler.NewAppHandler(services.app)
	h.openApi = openHandler.NewOpenApiHandler(services.openApi)
	h.openLog = openHandler.NewOpenLogHandler(services.openLog)
	h.message = msgHandler.NewMessageHandler(services.message)
	h.dict = dictHandler.NewDictHandler(services.dict)
	h.task = taskHandler.NewTaskHandler(services.task)
	h.userAdmin = userHandler.NewUserHandler(services.user, lazyCacheMgr)
	h.content.Category = content.NewContentCategoryHandler(services.contentCategory)
	h.content.Article = content.NewContentArticleHandler(services.contentArticle)
	h.content.BannerGroup = content.NewContentBannerGroupHandler(services.contentBannerGroup)
	h.content.BannerItem = content.NewContentBannerItemHandler(services.contentBannerItem)
	h.route = route.NewRouteHandler(services.menu, services.admin)

	h.client.echo = clientHandler.NewEchoHandler()
	h.client.user = clientHandler.NewUserHandler(services.user, services.uploadRecord)
	h.client.auth = clientHandler.NewAuthHandler(services.verification, captchaMgr, configWatcher, repos.user)
	h.client.message = clientHandler.NewMessageHandler(services.message)
	h.client.content = clientHandler.NewContentHandler(services.contentArticle, services.contentCategory, services.contentBannerGroup, services.contentBannerItem)
	h.client.storage = clientHandler.NewClientStorageHandler(services.uploadRecord)

	return h
}
