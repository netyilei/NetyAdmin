package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
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
	ratelimitPkg "NetyAdmin/internal/pkg/ratelimit"
	storagePkg "NetyAdmin/internal/pkg/storage"
	"NetyAdmin/internal/pkg/task"
	"NetyAdmin/internal/pkg/utils"

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
	// 0. DB Migration（基于 golang-migrate，SQL 文件 embed 进二进制）
	//    使用独立连接执行迁移，避免与 GORM 连接池的 advisory lock 冲突。
	if cfg.Migration.Enabled {
		if err := migration.Run(cfg.Database.DSNURL()); err != nil {
			return nil, fmt.Errorf("数据库迁移失败: %w", err)
		}
	}

	// 1. Redis & Cache（先于健康检查器：后者需复用 Redis 连接做探活）
	redisClient, err := pkgredis.NewClient(&cfg.Redis)
	if err != nil {
		return nil, err
	}

	// 2. DB & Redis Health Checker（基于 hellofresh/health-go v5）
	//    复用已建立的 DB / Redis 连接池，仅在 Redis 启用时注册其探活检查。
	var redisHealthOpt database.HealthCheckerOption
	if redisClient != nil {
		redisHealthOpt = database.WithRedis(redisClient)
	}
	dbHealthChecker, err := database.NewHealthChecker(db, redisHealthOpt)
	if err != nil {
		return nil, fmt.Errorf("健康检查器初始化失败: %w", err)
	}

	// 3. JWT
	jwtInstance, err := jwt.New(cfg.JWT.Secret, cfg.JWT.Expiration)
	if err != nil {
		return nil, fmt.Errorf("JWT 初始化失败: %w", err)
	}

	// 4. Repositories
	repos := initRepositories(db)

	// 5. PubSubBus
	nodeID := generateNodeID()
	var eventBus pubsub.EventBus
	busDriver := "memory" // 默认值，根据下方分支更新
	switch cfg.Bus.Driver {
	case "memory":
		busDriver = "memory"
		eventBus = pubsub.NewMemoryDriver(nodeID)
	case "redis":
		if redisClient == nil {
			return nil, fmt.Errorf("bus driver 设置为 redis 但 Redis 未启用")
		}
		busDriver = "redis"
		eventBus = pubsub.NewRedisDriver(redisClient, cfg.Redis.Prefix, nodeID)
	default:
		if cfg.Redis.Enabled && redisClient != nil {
			busDriver = "redis"
			eventBus = pubsub.NewRedisDriver(redisClient, cfg.Redis.Prefix, nodeID)
		} else {
			busDriver = "memory"
			eventBus = pubsub.NewMemoryDriver(nodeID)
		}
	}

	// 多机部署校验：multi_node=true 但 bus 为 memory 模式时告警（缓存/IPAC/配置失效不会跨节点同步）
	if cfg.Server.MultiNode && busDriver == "memory" {
		slog.Warn("检测到多节点部署(multi_node=true)但事件总线为 memory 模式，" +
			"缓存/IPAC/配置失效不会跨节点同步。请设置 [bus] driver = \"redis\"")
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

	// CacheDelete: 跨节点删 L1（payload 为 buildKey 后的完整 key）
	// 仅 L1 开启时有实际效果；当前 L1 关闭，订阅存在但 InvalidateL1ByKey 是 no-op
	_ = eventBus.Subscribe(pubsub.TopicCacheDelete, func(msg []byte) {
		var fullKey string
		if err := json.Unmarshal(msg, &fullKey); err == nil {
			_ = lazyCacheMgr.InvalidateL1ByKey(context.Background(), fullKey)
		}
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
		repos.uploadRecord,
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
	engine.Use(middleware.CORS())
	engine.Use(middleware.SecurityHeaders())
	engine.Use(middleware.Recovery(services.errorLog))
	engine.Use(middleware.ErrorLogger(services.errorLog))
	// 中间件超时与 HTTP server 保持一致（取 read_timeout 和 write_timeout 的较大值）
	middlewareTimeout := time.Duration(cfg.Server.ReadTimeout) * time.Second
	if wt := time.Duration(cfg.Server.WriteTimeout) * time.Second; wt > middlewareTimeout {
		middlewareTimeout = wt
	}
	if middlewareTimeout <= 0 {
		middlewareTimeout = 120 * time.Second
	}
	engine.Use(middleware.Timeout(middlewareTimeout))
	engine.Use(middleware.Logger())
	engine.Use(middleware.OperationLogger(services.logBus))

	// 临时关闭标准输出以屏蔽路由注册时的 [GIN-debug] 日志
	gin.DefaultWriter = io.Discard
	router.Register(engine)
	cRouter.Register(engine)
	gin.DefaultWriter = os.Stdout

	// /health 标准健康检查端点（供 K8s liveness/readiness 探针或负载均衡探测）
	// 不走鉴权与限流，直接返回 DB/Redis 探活结果。
	engine.GET("/health", dbHealthChecker.Handler())

	// 静态文件服务：提供前端 SPA 页面（admin-web/dist/）
	// 使 http://localhost:8010/ 直接访问管理后台
	engine.Static("/assets", "../admin-web/dist/assets")
	engine.StaticFile("/favicon.svg", "../admin-web/dist/favicon.svg")

	// SPA 兜底：未匹配 API 的 GET 请求返回 index.html（支持 Vue Router history 模式）
	engine.NoRoute(func(c *gin.Context) {
		if c.Request.Method == "GET" {
			c.File("../admin-web/dist/index.html")
			return
		}
		c.Status(http.StatusNotFound)
	})

	return NewApp(cfg, db, engine, dbHealthChecker, taskManager, services.logBus, eventBus), nil
}

// generateNodeID 生成进程级唯一节点标识，用于事件总线过滤本节点回环消息
// 格式: hostname-ULID后8位（hostname 提供主机维度，ULID 后缀提供进程维度）
func generateNodeID() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "unknown"
	}
	ulid := utils.NewULID()
	if len(ulid) > 8 {
		ulid = ulid[len(ulid)-8:]
	}
	return fmt.Sprintf("%s-%s", host, ulid)
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
	storageMgr := storagePkg.NewManager(storagePkg.NewMinioDriverFactory())
	// 限流器：复用缓存层的 Redis 连接，Redis 不可用时自动降级为进程内内存限流
	rateLimiter := ratelimitPkg.New(lazyCacheMgr.GetRedisClient(), cfg.Redis.Prefix)

	s := &serviceSet{}
	tokenStore := userServicePkg.NewTokenStoreFromConfig(configWatcher, repos.user, lazyCacheMgr)
	s.admin = systemService.NewAdminService(repos.admin, repos.role, jwtInstance, lazyCacheMgr, tokenStore)
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
	s.app = openServicePkg.NewAppService(repos.app, lazyCacheMgr, cfg.Security.AESKey, s.ipac, repos.ipac, storageMgr, configWatcher, rateLimiter)
	s.openApi = openServicePkg.NewOpenApiService(repos.openApi, repos.app, lazyCacheMgr)
	s.openLog = openServicePkg.NewOpenLogService(repos.openLog, func(ctx context.Context, logRecord *openEntity.OpenPlatformLog) error {
		return s.logBus.Record(ctx, logRecord)
	})

	// Message Drivers
	configProvider := msgPkg.NewWatcherConfigProvider(configWatcher)
	drivers := make(map[string]msgPkg.Driver)
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
	s.user = userServicePkg.NewUserService(repos.user, jwtInstance, s.verification, configWatcher, storageMgr, captchaStore, tokenStore, lazyCacheMgr)

	middleware.InitJWT(jwtInstance, repos.user, tokenStore, repos.admin)

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
	s.storageConfig = storageService.NewConfigService(repos.storageConfig, repos.uploadRecord, storageMgr, lazyCacheMgr, eventBus, cfg.Security.AESKey)
	s.uploadRecord = storageService.NewRecordService(repos.uploadRecord, s.storageConfig, storageMgr, s.app, cfg.JWT.Secret)
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
