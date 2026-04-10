package app

import (
	"context"
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"silentorder/internal/config"
	"silentorder/internal/interface/admin/http/handler/v1/admin"
	"silentorder/internal/interface/admin/http/handler/v1/auth"
	"silentorder/internal/interface/admin/http/handler/v1/content"
	"silentorder/internal/interface/admin/http/handler/v1/error_log"
	"silentorder/internal/interface/admin/http/handler/v1/operation_log"
	"silentorder/internal/interface/admin/http/handler/v1/route"
	storageHandler "silentorder/internal/interface/admin/http/handler/v1/storage"
	"silentorder/internal/interface/admin/http/handler/v1/system"
	"silentorder/internal/middleware"
	"silentorder/internal/pkg/cache"
	"silentorder/internal/pkg/configsync"
	"silentorder/internal/pkg/database"
	"silentorder/internal/pkg/jwt"
	pkgredis "silentorder/internal/pkg/redis"
	storagePkg "silentorder/internal/pkg/storage"
	"silentorder/internal/pkg/task"

	"silentorder/internal/job"
	"silentorder/internal/pkg/migration"
	contentRepo "silentorder/internal/repository/content"
	logRepo "silentorder/internal/repository/log"
	storageRepo "silentorder/internal/repository/storage"
	sysRepo "silentorder/internal/repository/system"
	"silentorder/internal/interface/admin/http/router"
	contentService "silentorder/internal/service/content"
	logService "silentorder/internal/service/log"
	storageService "silentorder/internal/service/storage"
	systemService "silentorder/internal/service/system"
)

func Bootstrap(cfg *config.Config, db *gorm.DB) (*App, error) {
	// 0. DB Health Checker
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
	storageConfigService := storageService.NewConfigService(storageConfigRepo, uploadRecordRepo, storageMgr)
	uploadRecordService := storageService.NewRecordService(uploadRecordRepo, storageConfigRepo, storageMgr)
	contentCategoryService := contentService.NewCategoryService(contentCategoryRepo, lazyCacheMgr, configWatcher)
	contentArticleService := contentService.NewArticleService(contentArticleRepo, contentCategoryRepo)
	contentBannerGroupService := contentService.NewBannerGroupService(contentBannerGroupRepo)
	contentBannerItemService := contentService.NewBannerItemService(contentBannerItemRepo, contentBannerGroupRepo, contentArticleRepo)

	if err := storageConfigService.LoadAllConfigs(context.Background()); err != nil {
		log.Printf("加载存储配置失败: %v", err)
	}

	// 6. Handlers
	authH := auth.NewAuthHandler(adminService)
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
	r := router.NewRouter(
		authH,
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

	// 9. Migrator & Task Registration
	migrator := migration.NewMigrator(db, cfg.Migration.Dir)
	taskManager.Register(job.AllJobs(
		migrator,
		contentArticleRepo,
		taskLogRepo,
		operationLogRepo,
		errorLogRepo,
		configWatcher,
	)...)

	// 9. Engine Setup
	gin.SetMode(cfg.Server.Mode)
	engine := gin.New()

	engine.Use(middleware.RequestID())
	engine.Use(middleware.Recovery(errorLogService))
	engine.Use(middleware.ErrorLogger(errorLogService))
	engine.Use(middleware.Timeout(30 * time.Second))
	engine.Use(middleware.Logger())
	engine.Use(middleware.OperationLogger(operationLogService))

	r.Register(engine)

	return NewApp(cfg, db, engine, dbHealthChecker, taskManager), nil
}
