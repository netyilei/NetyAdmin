package app

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"netyadmin/internal/config"
	"netyadmin/internal/interface/admin/http/handler/v1/admin"
	"netyadmin/internal/interface/admin/http/handler/v1/auth"
	"netyadmin/internal/interface/admin/http/handler/v1/content"
	"netyadmin/internal/interface/admin/http/handler/v1/error_log"
	"netyadmin/internal/interface/admin/http/handler/v1/operation_log"
	"netyadmin/internal/interface/admin/http/handler/v1/route"
	storageHandler "netyadmin/internal/interface/admin/http/handler/v1/storage"
	"netyadmin/internal/interface/admin/http/handler/v1/system"
	"netyadmin/internal/middleware"
	"netyadmin/internal/pkg/cache"
	"netyadmin/internal/pkg/configsync"
	"netyadmin/internal/pkg/database"
	"netyadmin/internal/pkg/jwt"
	pkgredis "netyadmin/internal/pkg/redis"
	storagePkg "netyadmin/internal/pkg/storage"
	"netyadmin/internal/pkg/task"

	"netyadmin/internal/interface/admin/http/router"
	"netyadmin/internal/job"
	"netyadmin/internal/pkg/migration"
	contentRepo "netyadmin/internal/repository/content"
	logRepo "netyadmin/internal/repository/log"
	storageRepo "netyadmin/internal/repository/storage"
	sysRepo "netyadmin/internal/repository/system"
	contentService "netyadmin/internal/service/content"
	logService "netyadmin/internal/service/log"
	storageService "netyadmin/internal/service/storage"
	systemService "netyadmin/internal/service/system"
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

	// 0.1 DB Migration (Run once before services initialization)
	migrator := migration.NewMigrator(db, cfg.Migration.Dir)
	if err := migrator.Run(); err != nil {
		return nil, fmt.Errorf("数据库迁移失败: %w", err)
	}

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
