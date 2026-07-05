package app

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"

	sentrygin "github.com/getsentry/sentry-go/gin"
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
	authPkg "NetyAdmin/internal/pkg/auth"
	"NetyAdmin/internal/pkg/cache"
	"NetyAdmin/internal/pkg/captcha"
	"NetyAdmin/internal/pkg/configsync"
	"NetyAdmin/internal/pkg/database"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/jwt"
	msgPkg "NetyAdmin/internal/pkg/message"
	"NetyAdmin/internal/pkg/pubsub"
	ratelimitPkg "NetyAdmin/internal/pkg/ratelimit"
	pkgredis "NetyAdmin/internal/pkg/redis"
	"NetyAdmin/internal/pkg/response"
	pkgSentry "NetyAdmin/internal/pkg/sentry"
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
	contentAdminService "NetyAdmin/internal/service/content/admin"
	contentClientService "NetyAdmin/internal/service/content/client"
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
	// 0. Sentry 错误追踪初始化（DSN 为空时自动跳过）
	//    尽早初始化，确保后续 panic 和错误都能被捕获
	if err := pkgSentry.Init(cfg.Sentry); err != nil {
		// Sentry 初始化失败不阻断启动，仅打印警告
		slog.Warn("Sentry 初始化失败，错误追踪已禁用", "error", err)
	}

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

	// 3. JWT (RS256 非对称签名)
	//    私钥/公钥从 [jwt].private_key_file / private_key_pem / public_key_file / public_key_pem 加载，
	//    file path 优先；二者均空时 fail-closed（已在 ValidateConfig 中校验，此处再防御性校验）。
	rsaPrivateKey, err := loadRSAPrivateKey(&cfg.JWT)
	if err != nil {
		return nil, fmt.Errorf("加载 JWT RS256 私钥失败: %w", err)
	}
	rsaPublicKey, err := loadRSAPublicKey(&cfg.JWT)
	if err != nil {
		return nil, fmt.Errorf("加载 JWT RS256 公钥失败: %w", err)
	}
	jwtInstance, err := jwt.New(rsaPrivateKey, rsaPublicKey, cfg.JWT.AccessTokenTTL.Duration(), cfg.JWT.RefreshTokenTTL.Duration())
	if err != nil {
		return nil, fmt.Errorf("JWT 初始化失败: %w", err)
	}

	// 4. Repositories
	repos := initRepositories(db)

	// 4.1 TransactionManager（事务管理器，无状态可作为单例在应用生命周期内复用）
	//     Service 层通过 tm.Begin/Commit/Rollback 编排跨 Repository 的多步事务，
	//     Repository 通过 database.GetDB(ctx, fallback) 隐式复用事务句柄。
	tm := database.NewTransactionManager(db)

	// 4.2 LoginLimiter（登录端点 IP 维度限流器）
	//     仅作用于 admin /auth/login + /auth/refresh-token 与 client /user/login + /user/refresh-token，
	//     不影响其他接口。Redis 未配置（redisClient == nil）时 NewLoginLimiter 返回 noop 实现（fail-open）。
	//     算法：Redis ZSET 滑动窗口（ZADD + ZREMRANGEBYSCORE + ZCARD）。
	loginLimiter := authPkg.NewLoginLimiter(redisClient, cfg.Redis.Prefix, cfg.LoginRateLimit.Window.Duration(), cfg.LoginRateLimit.Max)

	// 5. PubSubBus
	nodeID := generateNodeID()
	var eventBus pubsub.EventBus
	busDriver := "memory" // 默认值，根据下方分支更新
	switch cfg.Bus.Driver {
	case "memory":
		busDriver = "memory"
		eventBus = pubsub.NewMemoryDriver(nodeID, cfg.PubSub.Workers, cfg.PubSub.QueueSize)
	case "redis":
		if redisClient == nil {
			return nil, fmt.Errorf("bus driver 设置为 redis 但 Redis 未启用")
		}
		busDriver = "redis"
		eventBus = pubsub.NewRedisDriver(redisClient, cfg.Redis.Prefix, nodeID, cfg.PubSub.Workers, cfg.PubSub.QueueSize)
	default:
		if cfg.Redis.Enabled && redisClient != nil {
			busDriver = "redis"
			eventBus = pubsub.NewRedisDriver(redisClient, cfg.Redis.Prefix, nodeID, cfg.PubSub.Workers, cfg.PubSub.QueueSize)
		} else {
			busDriver = "memory"
			eventBus = pubsub.NewMemoryDriver(nodeID, cfg.PubSub.Workers, cfg.PubSub.QueueSize)
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
	services := initServices(repos, jwtInstance, lazyCacheMgr, taskManager, configWatcher, cfg, captchaStore, eventBus, tm, captchaMgr)
	handlers := initHandlers(services, repos, lazyCacheMgr)

	// 7. Register PubSubBus subscribers（fail-closed：订阅失败阻断启动）
	// ConfigSync
	if err := safeSubscribe(eventBus, pubsub.TopicConfigSync, func(ctx context.Context, msg []byte) {
		_ = configWatcher.ForceReload(ctx)
	}); err != nil {
		return nil, err
	}

	// StorageSync
	if err := safeSubscribe(eventBus, pubsub.TopicStorageSync, func(ctx context.Context, msg []byte) {
		_ = services.storageConfig.LoadAllConfigs(ctx)
	}); err != nil {
		return nil, err
	}

	// CacheInvalidation
	if err := safeSubscribe(eventBus, pubsub.TopicCacheInvalidation, func(ctx context.Context, msg []byte) {
		var tags []string
		if err := json.Unmarshal(msg, &tags); err == nil {
			_ = lazyCacheMgr.InvalidateL1ByTags(ctx, tags...)
		}
	}); err != nil {
		return nil, err
	}

	// IPACReload
	if err := safeSubscribe(eventBus, pubsub.TopicIPACReload, func(ctx context.Context, msg []byte) {
		_ = services.ipac.ReloadCache(ctx)
	}); err != nil {
		return nil, err
	}

	// CacheDelete: 跨节点删 L1（payload 为 buildKey 后的完整 key）
	// 仅 L1 开启时有实际效果；当前 L1 关闭，订阅存在但 InvalidateL1ByKey 是 no-op
	if err := safeSubscribe(eventBus, pubsub.TopicCacheDelete, func(ctx context.Context, msg []byte) {
		var fullKey string
		if err := json.Unmarshal(msg, &fullKey); err == nil {
			_ = lazyCacheMgr.InvalidateL1ByKey(ctx, fullKey)
		}
	}); err != nil {
		return nil, err
	}

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
		loginLimiter,
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
		repos.user,
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
		loginLimiter,
	)

	// 10. Engine Setup
	gin.SetMode(cfg.Server.Mode)
	engine := gin.New()

	// 配置可信代理：必须在注册路由前调用（gin 限制）。
	// 空数组（默认）= 不信任任何代理，c.ClientIP() 直接回退到 RemoteAddr，
	// 防止攻击者伪造 X-Forwarded-For / X-Real-IP 头绕过 IPAC 白名单/黑名单。
	// 生产环境若部署在 Nginx/CDN 之后，需在 config.toml 配置真实代理 IP/CIDR。
	if err := engine.SetTrustedProxies(cfg.Server.TrustedProxies); err != nil {
		return nil, fmt.Errorf("设置可信代理失败: %w", err)
	}

	engine.Use(middleware.RequestID())
	engine.Use(middleware.CORS(cfg.CORS.AllowedOrigins))
	engine.Use(middleware.SecurityHeaders(cfg.SecurityHeaders.CSP))
	engine.Use(middleware.Recovery(services.errorLog))
	// sentrygin 必须在 Recovery 之后：panic 发生时 sentrygin 先捕获并上报 Sentry，
	// 然后 Repanic=true 重新 panic，由外层 Recovery 兜底记录到 DB 并返回 500。
	engine.Use(sentrygin.New(sentrygin.Options{Repanic: true}))
	// 将 requestID / path / userID 注入 Sentry Scope，实现前后端链路关联
	engine.Use(middleware.SentryTagSetter())
	engine.Use(middleware.ErrorLogger(services.errorLog))
	// 请求处理超时不再用 gin 中间件实现：原 middleware.Timeout 仅向 ctx 注入 deadline，
	// 无法主动拦截响应。改用 http.TimeoutHandler 在 app.go 中包装整个 engine，
	// 超时时由 stdlib 主动写入 503 + JSON 错误体（code:100006 / msg:"请求超时"）。
	// 配置项：[server].handler_timeout（默认 25s，应略小于 read_timeout/write_timeout）。
	engine.Use(middleware.Logger())
	engine.Use(middleware.OperationLogger(services.logBus))

	// 屏蔽路由注册时的 [GIN-debug] 日志
	gin.DefaultWriter = io.Discard

	// /health 标准健康检查端点（供 K8s liveness/readiness 探针或负载均衡探测）
	// 在 router.Register 之前注册，确保 /health 独立于任何业务路由组（含 IPACAuth）。
	// IPACAuth 现仅在 admin authGroup/permissionGroup 上注册（不再全局），
	// 公开路由（含 /health）天然豁免 IPAC 检查，Redis 故障 fail-closed 不会影响健康检查。
	engine.GET("/health", dbHealthChecker.Handler())

	router.Register(engine)
	cRouter.Register(engine)
	gin.DefaultWriter = os.Stdout

	// 静态文件服务：提供前端 SPA 页面（admin-web/dist/）
	// 使 http://localhost:8010/ 直接访问管理后台
	engine.Static("/assets", "../admin-web/dist/assets")
	engine.StaticFile("/favicon.svg", "../admin-web/dist/favicon.svg")

	// SPA 兜底：未匹配 API 的 GET 请求返回 index.html（支持 Vue Router history 模式）
	engine.NoRoute(func(c *gin.Context) {
		if c.Request.Method == http.MethodGet {
			c.File("../admin-web/dist/index.html")
			return
		}
		// 非 GET 请求返回统一 Response 格式（与 API 响应一致）
		c.JSON(http.StatusNotFound, response.Response{
			Code:      errorx.CodeNotFound.String(),
			Message:   "资源不存在",
			RequestID: c.GetString("requestID"),
		})
	})

	return NewApp(cfg, db, engine, tm, dbHealthChecker, taskManager, services.logBus, eventBus), nil
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
	admin                   systemService.AdminService
	role                    systemService.RoleService
	menu                    systemService.MenuService
	api                     systemService.APIService
	button                  systemService.ButtonService
	task                    taskServicePkg.TaskService
	sysConfig               systemService.ConfigService
	dict                    dictServicePkg.DictService
	ipac                    ipacServicePkg.IPACService
	app                     openServicePkg.AppService
	openApi                 openServicePkg.OpenApiService
	openLog                 openServicePkg.OpenLogService
	message                 msgServicePkg.MessageService
	msgSendJob              task.Task
	userAdmin               userServicePkg.UserAdminService
	userClient              userServicePkg.UserClientService
	verification            userServicePkg.VerificationService
	operationLog            logService.OperationService
	errorLog                logService.ErrorService
	storageConfig           storageService.ConfigService
	uploadRecord            storageService.RecordService
	contentCategoryAdmin    contentAdminService.CategoryService
	contentArticleAdmin     contentAdminService.ArticleService
	contentBannerGroupAdmin contentAdminService.BannerGroupService
	contentBannerItemAdmin  contentAdminService.BannerItemService

	contentCategoryClient    contentClientService.CategoryService
	contentArticleClient     contentClientService.ArticleService
	contentBannerGroupClient contentClientService.BannerGroupService
	contentBannerItemClient  contentClientService.BannerItemService
	emailDriver              msgPkg.Driver
	logBus                   logService.LogBusService
	captcha                  systemService.CaptchaService
}

func initServices(repos *repositorySet, jwtInstance *jwt.JWT, lazyCacheMgr cache.LazyCacheManager, taskManager *task.Manager, configWatcher configsync.ConfigWatcher, cfg *config.Config, captchaStore base64Captcha.Store, eventBus pubsub.EventBus, tm *database.TransactionManager, captchaMgr *captcha.Manager) *serviceSet {
	storageMgr := storagePkg.NewManager(storagePkg.NewMinioDriverFactory())
	// 限流器：复用缓存层的 Redis 连接，Redis 不可用时自动降级为进程内内存限流
	rateLimiter := ratelimitPkg.New(lazyCacheMgr.GetRedisClient(), cfg.Redis.Prefix)

	s := &serviceSet{}
	tokenStore := userServicePkg.NewTokenStore(repos.user, lazyCacheMgr)

	// ====================================================================
	// Phase 1: 创建 logBus（必须先于 taskService / openLog）
	//
	// 循环依赖处理策略（Task 21）：
	// taskService 与 openLog 通过 recordFunc 闭包将日志写入 logBus，
	// 形成「service → logBus」的单向依赖；logBus 自身仅依赖 repos
	// （operationLog / errorLog / openLog / taskLog）+ configWatcher，
	// 不反向依赖任何 service。因此本无真正的循环依赖，原 wire.go 中的
	// 「闭包捕获 s.logBus + 延迟到末尾才赋值」trick 是人为引入的初始化
	// 顺序问题——它依赖 Go 闭包对 s 指针的延迟绑定：闭包定义时 s.logBus
	// 为 nil，仅在运行时（HTTP 请求 / 任务完成回调）才被读取，那时 s 已
	// 经完整构造。该模式虽在当前调用链下安全，但隐式且脆弱——任何人在
	// initServices 内部、logBus 赋值前同步调用 recordFunc 都会 nil-panic。
	//
	// 修复方案（Option B：拆分初始化阶段）：把 logBus 的构造提前到所有
	// 依赖它的 service 之前，recordFunc 闭包改为直接捕获已初始化的 logBus
	// 局部变量，彻底消除延迟绑定 trick。这样即使闭包被同步调用也不会 panic，
	// 且初始化顺序在代码上一目了然，不再依赖隐式约定。
	// ====================================================================
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
	logBus := logService.NewLogBusService(writers, configs, configWatcher)
	s.logBus = logBus

	// ====================================================================
	// Phase 2: 创建其余 services（recordFunc 闭包直接捕获上方 logBus 局部
	// 变量，不再依赖 s.logBus 延迟绑定）
	// ====================================================================
	s.admin = systemService.NewAdminService(repos.admin, repos.role, jwtInstance, lazyCacheMgr, tokenStore, tm)
	s.role = systemService.NewRoleService(repos.role, repos.menu, repos.api, repos.button, lazyCacheMgr, tm)
	s.menu = systemService.NewMenuService(repos.menu, repos.button, repos.api, repos.role, lazyCacheMgr, tm)
	s.api = systemService.NewAPIService(repos.api, lazyCacheMgr, tm)
	s.button = systemService.NewButtonService(repos.button, lazyCacheMgr, tm)
	s.task = taskServicePkg.NewTaskService(taskManager, repos.taskLog, repos.systemConfig, configWatcher, func(ctx context.Context, logRecord *taskEntity.TaskLog) error {
		return logBus.Record(ctx, logRecord)
	}, tm)

	// Message Drivers（提前创建：sysConfig.TestEmail 依赖 emailDriver）
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

	s.sysConfig = systemService.NewConfigService(repos.systemConfig, configWatcher, eventBus, s.emailDriver)
	s.dict = dictServicePkg.NewDictService(repos.dict, lazyCacheMgr, tm)
	s.ipac = ipacServicePkg.NewIPACService(repos.ipac, eventBus, tm)
	s.app = openServicePkg.NewAppService(repos.app, lazyCacheMgr, cfg.Security.AESKey, s.ipac, repos.ipac, storageMgr, configWatcher, rateLimiter, tm)
	s.openApi = openServicePkg.NewOpenApiService(repos.openApi, repos.app, lazyCacheMgr, tm)
	s.openLog = openServicePkg.NewOpenLogService(repos.openLog, func(ctx context.Context, logRecord *openEntity.OpenPlatformLog) error {
		return logBus.Record(ctx, logRecord)
	})

	s.message = msgServicePkg.NewMessageService(repos.message, taskManager, drivers, lazyCacheMgr)
	s.msgSendJob = msgServicePkg.NewMsgSendJob(repos.message, drivers, configWatcher, tm)
	s.verification = userServicePkg.NewVerificationService(lazyCacheMgr, s.message, configWatcher, captchaStore)
	// userBase 由 admin/client 两端 service 共享：复用 repo/jwt/cache/tm 等底层依赖，
	// 避免重复构造与字段漂移。两端 service 仅 import 本端 DTO，保证 BFF 端隔离（spec D4）。
	userBase := userServicePkg.NewUserBase(repos.user, jwtInstance, s.verification, configWatcher, storageMgr, captchaStore, tokenStore, lazyCacheMgr, tm)
	s.userAdmin = userServicePkg.NewUserAdminService(userBase)
	s.userClient = userServicePkg.NewUserClientService(userBase)

	middleware.InitJWT(jwtInstance, repos.user, tokenStore, repos.admin, lazyCacheMgr)

	s.operationLog = logService.NewOperationService(repos.operationLog)
	s.errorLog = logService.NewErrorService(repos.errorLog, configWatcher, lazyCacheMgr, logBus)
	s.captcha = systemService.NewCaptchaService(captchaMgr, configWatcher)
	s.storageConfig = storageService.NewConfigService(repos.storageConfig, repos.uploadRecord, storageMgr, lazyCacheMgr, eventBus, cfg.Security.AESKey, tm)
	s.uploadRecord = storageService.NewRecordService(repos.uploadRecord, s.storageConfig, storageMgr, s.app, cfg.Security.UploadHMACKey, tm)
	// Admin content services
	s.contentCategoryAdmin = contentAdminService.NewCategoryService(repos.contentCategory, repos.contentArticle, s.storageConfig, lazyCacheMgr, configWatcher, tm)
	s.contentArticleAdmin = contentAdminService.NewArticleService(repos.contentArticle, repos.contentCategory, lazyCacheMgr, configWatcher)
	s.contentBannerGroupAdmin = contentAdminService.NewBannerGroupService(repos.contentBannerGroup, repos.contentBannerItem, s.storageConfig, lazyCacheMgr, configWatcher, tm)
	s.contentBannerItemAdmin = contentAdminService.NewBannerItemService(repos.contentBannerItem, repos.contentBannerGroup, repos.contentArticle, lazyCacheMgr)

	// Client content services（共享 Repository + cacheMgr；CategoryService 委托 admin 实现避免重复）
	s.contentArticleClient = contentClientService.NewArticleService(repos.contentArticle, repos.contentCategory, lazyCacheMgr, configWatcher)
	s.contentBannerGroupClient = contentClientService.NewBannerGroupService(repos.contentBannerGroup, lazyCacheMgr, configWatcher)
	s.contentBannerItemClient = contentClientService.NewBannerItemService(repos.contentBannerItem)
	s.contentCategoryClient = contentClientService.NewCategoryService(s.contentCategoryAdmin)

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

func initHandlers(services *serviceSet, repos *repositorySet, lazyCacheMgr cache.LazyCacheManager) *handlerSet {
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

// safeSubscribe 包装 eventBus.Subscribe：返回 error 实现 fail-closed。
// panic 恢复下沉到 pubsub/bus.go 的 dispatch 层（通过 recovery.GoSafe）。
//
// 设计说明：PubSub 消息分发是异步的，dispatch 在 goroutine 中调用 handler。
// 早期版本在 safeSubscribe 内部做 sync recover 包裹，但由于 dispatch 启动
// goroutine 时已用 recovery.GoSafe 包裹（SubTask 5.3），此处再包一层
// recover 属于冗余。简化为透传 handler，让恢复逻辑统一由 dispatch 的 GoSafe
// 负责——所有 PubSub 异步路径的 panic 都走同一条「slog.Error + Sentry
// CaptureException」管线，避免恢复逻辑分散在多处导致行为不一致。
//
// handler 接收 ctx，dispatch 已在其中通过 msg.Meta 恢复 request_id 到子 ctx
// （Task 8.4），handler 内部可用 slogutil.LoggerFromContext(ctx) 关联到原始请求。
//
// fail-closed 语义（P1-7）：订阅失败（如 topic 已注册 / 注册表写入异常）会
// 导致关键事件（ConfigSync / StorageSync / CacheInvalidation / IPACReload /
// CacheDelete）不被消费，服务"看起来正常"实则数据不一致。因此 Subscribe
// 失败必须阻断 Bootstrap 启动，调用方须 `return nil, err`。
//
// 保留此函数是为了维持 wire.go 调用点的可读性（语义化命名），并作为未来
// 若需要在 Subscribe 注册阶段做扩展（如 metrics 埋点）的扩展点。
func safeSubscribe(bus pubsub.EventBus, topic string, handler func(ctx context.Context, msg []byte)) error {
	if err := bus.Subscribe(topic, handler); err != nil {
		return fmt.Errorf("subscribe %s: %w", topic, err)
	}
	return nil
}

// loadRSAPrivateKey 从 config 加载 RSA 私钥用于 RS256 签发。
// 优先读 PrivateKeyFile；为空则用 PrivateKeyPEM 内联。两者均空返回 error（fail-closed）。
// 同时兼容 PKCS#1（RSA PRIVATE KEY）与 PKCS#8（PRIVATE KEY）两种 PEM 编码。
func loadRSAPrivateKey(cfg *config.JWTConfig) (*rsa.PrivateKey, error) {
	pemBytes, err := loadPEMSource(cfg.PrivateKeyFile, cfg.PrivateKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("读取私钥失败: %w", err)
	}
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("私钥 PEM 解码失败：不是合法的 PEM 格式")
	}

	// 优先尝试 PKCS#1（传统 RSA PRIVATE KEY 格式）
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}

	// 回退到 PKCS#8（通用 PRIVATE KEY 格式，可能含 ECDSA/RSA 等）
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("私钥解析失败（PKCS#1/PKCS#8 均不支持）: %w", err)
	}
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("私钥不是 RSA 类型（实际类型 %T），RS256 签名仅支持 RSA 私钥", key)
	}
	return rsaKey, nil
}

// loadRSAPublicKey 从 config 加载 RSA 公钥用于 RS256 验签。
// 优先读 PublicKeyFile；为空则用 PublicKeyPEM 内联。两者均空返回 error（fail-closed）。
// 公钥统一使用 PKIX 编码（PUBLIC KEY PEM block）。
func loadRSAPublicKey(cfg *config.JWTConfig) (*rsa.PublicKey, error) {
	pemBytes, err := loadPEMSource(cfg.PublicKeyFile, cfg.PublicKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("读取公钥失败: %w", err)
	}
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("公钥 PEM 解码失败：不是合法的 PEM 格式")
	}

	// PKIX 公钥解析（适用于 "PUBLIC KEY" PEM block，X.509 SubjectPublicKeyInfo）
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		// 兼容 PKCS#1 RSA PUBLIC KEY（少数工具导出格式）
		if rsaPub, perr := x509.ParsePKCS1PublicKey(block.Bytes); perr == nil {
			return rsaPub, nil
		}
		return nil, fmt.Errorf("公钥解析失败（PKIX/PKCS#1 均不支持）: %w", err)
	}
	rsaKey, ok := key.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("公钥不是 RSA 类型（实际类型 %T），RS256 验签仅支持 RSA 公钥", key)
	}
	return rsaKey, nil
}

// loadPEMSource 统一加载 PEM 内容：优先读 file path，为空则用内联 PEM 字符串。
// file path 与 inline 同时为空返回 error（fail-closed）。
func loadPEMSource(filePath, inlinePEM string) ([]byte, error) {
	if filePath != "" {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("读取文件 %q 失败: %w", filePath, err)
		}
		return data, nil
	}
	if inlinePEM == "" {
		return nil, fmt.Errorf("file path 与 inline PEM 均为空（fail-closed）")
	}
	return []byte(inlinePEM), nil
}
