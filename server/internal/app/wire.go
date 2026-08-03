package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"

	sentrygin "github.com/getsentry/sentry-go/gin"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"NetyAdmin/internal/config"
	"NetyAdmin/internal/interface/admin/http/router"
	clientRouter "NetyAdmin/internal/interface/client/http/router"
	"NetyAdmin/internal/middleware"
	authPkg "NetyAdmin/internal/pkg/auth"
	"NetyAdmin/internal/pkg/cache"
	"NetyAdmin/internal/pkg/captcha"
	"NetyAdmin/internal/pkg/configsync"
	"NetyAdmin/internal/pkg/database"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/jwt"
	"NetyAdmin/internal/pkg/pubsub"
	pkgredis "NetyAdmin/internal/pkg/redis"
	"NetyAdmin/internal/pkg/response"
	pkgSentry "NetyAdmin/internal/pkg/sentry"
	"NetyAdmin/internal/pkg/task"

	"NetyAdmin/internal/job"
	"NetyAdmin/internal/pkg/migration"
	userServicePkg "NetyAdmin/internal/service/user"
)

// Bootstrap 是应用主初始化函数（依赖注入组合根）。
//
// 接收解析好的 *config.Config 和 GORM *gorm.DB 连接池，
// 依次初始化 Sentry、DB Migration、Redis、Cache、JWT、Repositories、
// Services、Handlers、PubSub 订阅、Router、Task Registration，
// 最后组装为 *App 返回。main.go 拿到 *App 后调用 Run() 启动 HTTP 服务。
//
// 初始化顺序由依赖关系决定（如：logBus 必须在 taskService 之前创建，
// storageConfig 必须在 uploadRecord 之前创建等）。
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
	tokenStore := userServicePkg.NewTokenStore(repos.user, lazyCacheMgr)
	services := initServices(repos, jwtInstance, lazyCacheMgr, redisClient, taskManager, configWatcher, cfg, captchaStore, eventBus, tm, captchaMgr, tokenStore)
	handlers := initHandlers(services)

	// 7. Register PubSubBus subscribers（fail-closed：订阅失败阻断启动）
	// ConfigSync
	if err := safeSubscribe(eventBus, pubsub.TopicConfigSync, func(ctx context.Context, msg []byte) {
		if err := configWatcher.ForceReload(ctx); err != nil {
			slog.Error("configsync reload failed", "err", err)
		}
	}); err != nil {
		return nil, err
	}

	// StorageSync
	if err := safeSubscribe(eventBus, pubsub.TopicStorageSync, func(ctx context.Context, msg []byte) {
		if err := services.storageConfig.LoadAllConfigs(ctx); err != nil {
			slog.Error("storagesync reload failed", "err", err)
		}
	}); err != nil {
		return nil, err
	}

	// CacheInvalidation
	if err := safeSubscribe(eventBus, pubsub.TopicCacheInvalidation, func(ctx context.Context, msg []byte) {
		var tags []string
		if err := json.Unmarshal(msg, &tags); err != nil {
			slog.Warn("cacheinvalidation: unmarshal tags failed",
				"msg", string(msg), "err", err)
		} else if err := lazyCacheMgr.InvalidateL1ByTags(ctx, tags...); err != nil {
			slog.Error("cacheinvalidation: invalidate L1 by tags failed",
				"tags", tags, "err", err)
		}
	}); err != nil {
		return nil, err
	}

	// IPACReload
	if err := safeSubscribe(eventBus, pubsub.TopicIPACReload, func(ctx context.Context, msg []byte) {
		if err := services.ipac.ReloadCache(ctx); err != nil {
			slog.Error("ipac reload failed", "err", err)
		}
	}); err != nil {
		return nil, err
	}

	// 8. Router
	authMW := middleware.NewAuthMiddleware(jwtInstance, repos.user, repos.userToken, tokenStore, repos.admin, lazyCacheMgr)
	r := router.NewRouter(
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
		authMW,
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
		repos.userToken,
		configWatcher,
	)...)
	taskManager.Register(services.msgSendJob)

	cr := clientRouter.NewClientRouter(
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
		authMW,
	)

	// 10. Engine Setup
	gin.SetMode(cfg.Server.Mode)
	engine := gin.New()

	// 配置可信代理：必须在注册路由前调用（gin 限制）。
	// 空数组（默认）= 不信任任何代理，c.ClientIP() 直接回退到 RemoteAddr，
	// 防止攻击者伪造 X-Forwarded-For / X-Real-IP 头绕过 IPAC 白名单/黑名单。
	// 生产环境若部署在 Nginx/CDN 之后，需在 config.yaml 配置真实代理 IP/CIDR。
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

	// 路由注册
	r.Register(engine)
	cr.Register(engine)
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

	return NewApp(cfg, db, engine, tm, dbHealthChecker, taskManager, services.logBus, eventBus,
		jwtInstance, tokenStore, services.verification, repos.user, lazyCacheMgr, services.oauthBinding), nil
}
