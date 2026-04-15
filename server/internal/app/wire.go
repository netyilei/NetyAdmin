package app

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"NetyAdmin/internal/config"
	"NetyAdmin/internal/interface/admin/http/handler/v1/admin"
	"NetyAdmin/internal/interface/admin/http/handler/v1/auth"
	"NetyAdmin/internal/interface/admin/http/handler/v1/common"
	"NetyAdmin/internal/interface/admin/http/handler/v1/content"
	"NetyAdmin/internal/interface/admin/http/handler/v1/error_log"
	"NetyAdmin/internal/interface/admin/http/handler/v1/operation_log"
	"NetyAdmin/internal/interface/admin/http/handler/v1/route"
	storageHandler "NetyAdmin/internal/interface/admin/http/handler/v1/storage"
	"NetyAdmin/internal/interface/admin/http/handler/v1/system"
	"NetyAdmin/internal/middleware"
	"NetyAdmin/internal/pkg/cache"
	"NetyAdmin/internal/pkg/captcha"
	"NetyAdmin/internal/pkg/configsync"
	"NetyAdmin/internal/pkg/database"
	"NetyAdmin/internal/pkg/jwt"
	pkgredis "NetyAdmin/internal/pkg/redis"
	storagePkg "NetyAdmin/internal/pkg/storage"
	"NetyAdmin/internal/pkg/task"

	"NetyAdmin/internal/interface/admin/http/router"
	"NetyAdmin/internal/job"
	"NetyAdmin/internal/pkg/migration"
	contentRepo "NetyAdmin/internal/repository/content"
	logRepo "NetyAdmin/internal/repository/log"
	storageRepo "NetyAdmin/internal/repository/storage"
	sysRepo "NetyAdmin/internal/repository/system"
	contentService "NetyAdmin/internal/service/content"
	logService "NetyAdmin/internal/service/log"
	storageService "NetyAdmin/internal/service/storage"
	systemService "NetyAdmin/internal/service/system"
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
	middleware.InitJWT(jwtInstance)

	// 3. Repositories
	systemConfigRepo := sysRepo.NewConfigRepository(db)
	adminRepo := sysRepo.NewAdminRepository(db)
	roleRepo := sysRepo.NewRoleRepository(db)
	menuRepo := sysRepo.NewMenuRepository(db)
	apiRepo := sysRepo.NewAPIRepository(db)
	buttonRepo := sysRepo.NewButtonRepository(db)
	operationLogRepo := logRepo.NewOperationRepository(db)
	errorLogRepo := logRepo.NewErrorRepository(db)
	storageConfigRepo := storageRepo.NewConfigRepository(db)
	uploadRecordRepo := storageRepo.NewRecordRepository(db)
	contentCategoryRepo := contentRepo.NewContentCategoryRepository(db)
	contentArticleRepo := contentRepo.NewContentArticleRepository(db)
	contentBannerGroupRepo := contentRepo.NewContentBannerGroupRepository(db)
	contentBannerItemRepo := contentRepo.NewContentBannerItemRepository(db)
	taskLogRepo := sysRepo.NewTaskLogRepository(db)
	dictRepo := sysRepo.NewDictRepository(db)

	// 4. Config Sync & Cache Manager
	configWatcher := configsync.NewConfigWatcher(systemConfigRepo, redisClient, &cfg.Redis)
	go configWatcher.WatchBlocking(context.Background())

	lazyCacheMgr, err := cache.NewLazyCacheManager(&cfg.Redis, redisClient, configWatcher)
	if err != nil {
		return nil, err
	}

	// 5. Task Manager (Moved up for TaskService dependency)
	taskManager := task.NewManager(&cfg.Task, &cfg.Redis, redisClient)

	// 5.1 Captcha Manager
	captchaStore := captcha.NewDualStore(lazyCacheMgr, configWatcher, db)
	captchaMgr := captcha.NewManager(captchaStore)

	// 6. Services
	storageMgr := storagePkg.NewManager(storagePkg.NewS3DriverFactory())

	adminService := systemService.NewAdminService(adminRepo, roleRepo, jwtInstance, lazyCacheMgr)
	roleService := systemService.NewRoleService(roleRepo, menuRepo, apiRepo, buttonRepo, lazyCacheMgr)
	menuService := systemService.NewMenuService(menuRepo, buttonRepo, lazyCacheMgr)
	apiService := systemService.NewAPIService(apiRepo, lazyCacheMgr)
	buttonService := systemService.NewButtonService(buttonRepo, lazyCacheMgr)
	taskService := systemService.NewTaskService(taskManager, taskLogRepo, systemConfigRepo, configWatcher)
	sysConfigService := systemService.NewConfigService(systemConfigRepo, redisClient, &cfg.Redis, configWatcher)
	dictService := systemService.NewDictService(dictRepo, lazyCacheMgr)
	operationLogService := logService.NewOperationService(operationLogRepo)
	errorLogService := logService.NewErrorService(errorLogRepo, configWatcher, redisClient)
	storageConfigService := storageService.NewConfigService(storageConfigRepo, uploadRecordRepo, storageMgr, lazyCacheMgr)
	uploadRecordService := storageService.NewRecordService(uploadRecordRepo, storageConfigRepo, storageMgr)
	contentCategoryService := contentService.NewCategoryService(contentCategoryRepo, storageConfigService, lazyCacheMgr, configWatcher)
	contentArticleService := contentService.NewArticleService(contentArticleRepo, contentCategoryRepo)
	contentBannerGroupService := contentService.NewBannerGroupService(contentBannerGroupRepo, storageConfigService)
	contentBannerItemService := contentService.NewBannerItemService(contentBannerItemRepo, contentBannerGroupRepo, contentArticleRepo)

	if err := storageConfigService.LoadAllConfigs(context.Background()); err != nil {
		log.Printf("加载存储配置失败: %v", err)
	}

	// 6. Handlers
	authH := auth.NewAuthHandler(adminService, captchaMgr, configWatcher)
	commonH := common.NewCommonHandler(captchaMgr, configWatcher)
	adminH := admin.NewAdminHandler(adminService)
	operationLogH := operation_log.NewOperationLogHandler(operationLogService)
	errorLogH := error_log.NewErrorLogHandler(errorLogService)
	systemH := system.NewSystemHandler(roleService, menuService, apiService, buttonService, adminService, sysConfigService, taskService, dictService)
	storageH := storageHandler.NewStorageHandler(storageConfigService, uploadRecordService)
	contentH := content.NewContentHandler(
		contentCategoryService,
		contentArticleService,
		contentBannerGroupService,
		contentBannerItemService,
	)
	routeH := route.NewRouteHandler(menuService)

	// 7. Router
	router := router.NewRouter(
		authH,
		commonH,
		adminH,
		systemH,
		storageH,
		contentH.Category,
		contentH.Article,
		contentH.BannerGroup,
		contentH.BannerItem,
		operationLogH,
		errorLogH,
		routeH,
		roleService, // Inject as AuthVerifier
	)

	// 9. Task Registration
	taskManager.Register(job.AllJobs(
		contentArticleRepo,
		taskLogRepo,
		operationLogRepo,
		errorLogRepo,
		configWatcher,
	)...)

	// 9. Engine Setup
	gin.SetMode(cfg.Server.Mode)
	// 如果是 debug 模式，默认会打印路由注册信息。通过指定新的 engine 而不使用 gin.Default()，
	// 并配合自定义的 Logger 中件间，可以在保持调试能力的同时，让启动输出更清爽。
	engine := gin.New()

	engine.Use(middleware.RequestID())
	engine.Use(middleware.Recovery(errorLogService))
	engine.Use(middleware.ErrorLogger(errorLogService))
	engine.Use(middleware.Timeout(30 * time.Second))
	engine.Use(middleware.Logger())
	engine.Use(middleware.OperationLogger(operationLogService))

	// 临时关闭标准输出以屏蔽路由注册时的 [GIN-debug] 日志
	gin.DefaultWriter = io.Discard
	router.Register(engine)
	// 注册完成后恢复标准输出，确保后续请求日志正常打印
	gin.DefaultWriter = os.Stdout

	return NewApp(cfg, db, engine, dbHealthChecker, taskManager), nil
}
