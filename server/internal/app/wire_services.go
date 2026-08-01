package app

import (
	"context"
	"log/slog"

	"github.com/mojocn/base64Captcha"
	"github.com/redis/go-redis/v9"

	"NetyAdmin/internal/config"
	"NetyAdmin/internal/pkg/cache"
	"NetyAdmin/internal/pkg/captcha"
	"NetyAdmin/internal/pkg/configsync"
	"NetyAdmin/internal/pkg/database"
	"NetyAdmin/internal/pkg/jwt"
	msgPkg "NetyAdmin/internal/pkg/message"
	"NetyAdmin/internal/pkg/pubsub"
	ratelimitPkg "NetyAdmin/internal/pkg/ratelimit"
	storagePkg "NetyAdmin/internal/pkg/storage"
	"NetyAdmin/internal/pkg/task"

	logEntity "NetyAdmin/internal/domain/entity/log"
	openEntity "NetyAdmin/internal/domain/entity/open_platform"
	taskEntity "NetyAdmin/internal/domain/entity/task"
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
	oauthBinding             userServicePkg.OAuthBindingService
}

func initServices(repos *repositorySet, jwtInstance *jwt.JWT, lazyCacheMgr *cache.LazyCacheManager, redisClient *redis.Client, taskManager *task.Manager, configWatcher configsync.ConfigWatcher, cfg *config.Config, captchaStore base64Captcha.Store, eventBus pubsub.EventBus, tm database.TxManager, captchaMgr *captcha.Manager, tokenStore userServicePkg.TokenStore) *serviceSet {
	storageMgr := storagePkg.NewManager(storagePkg.NewMinioDriverFactory())
	// 限流器：复用 Bootstrap 已建立的 Redis 连接，Redis 不可用时自动降级为进程内内存限流
	rateLimiter := ratelimitPkg.New(redisClient, cfg.Redis.Prefix)

	s := &serviceSet{}

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
	s.admin = systemService.NewAdminService(repos.admin, repos.role, jwtInstance, lazyCacheMgr, lazyCacheMgr, tokenStore, tm)
	s.role = systemService.NewRoleService(repos.role, repos.menu, repos.api, repos.button, lazyCacheMgr, tm)
	s.menu = systemService.NewMenuService(repos.menu, repos.button, repos.api, repos.role, lazyCacheMgr, tm)
	s.api = systemService.NewAPIService(repos.api, lazyCacheMgr, tm)
	s.button = systemService.NewButtonService(repos.button, lazyCacheMgr, tm)
	s.task = taskServicePkg.NewTaskService(taskManager, repos.taskLog, repos.systemConfig, configWatcher, func(ctx context.Context, logRecord *taskEntity.TaskLog) error {
		return logBus.Record(ctx, logRecord)
	}, tm)

	// Message Drivers：仅在对应 [email]/[sms].enabled = true 时注入 driver。
	// 未启用的 channel 不在 drivers map 中，发送该 channel 的消息会在 job 层返回
	// "no driver found for channel: xxx"（message_job.go 的既有失败路径）。
	configProvider := msgPkg.NewWatcherConfigProvider(configWatcher)
	drivers := make(map[string]msgPkg.Driver)
	if cfg.Email.Enabled {
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
		slog.Info("Email driver 已启用", "host", cfg.Email.Host, "port", cfg.Email.Port)
	} else {
		slog.Info("Email driver 未启用（[email].enabled = false）")
	}

	if cfg.Sms.Enabled {
		drivers["sms"] = msgPkg.NewTencentSmsDriver(msgPkg.SmsConfig{
			SecretID:  cfg.Sms.SecretID,
			SecretKey: cfg.Sms.SecretKey,
			AppID:     cfg.Sms.AppID,
			SignName:  cfg.Sms.SignName,
			Region:    cfg.Sms.Region,
		}, configProvider)
		slog.Info("SMS driver 已启用 (tencent)", "region", cfg.Sms.Region, "sign_name", cfg.Sms.SignName)
	} else {
		slog.Info("SMS driver 未启用（[sms].enabled = false）")
	}

	s.sysConfig = systemService.NewConfigService(repos.systemConfig, configWatcher, eventBus, s.emailDriver)
	s.dict = dictServicePkg.NewDictService(repos.dict, lazyCacheMgr, tm)
	s.ipac = ipacServicePkg.NewIPACService(repos.ipac, eventBus, tm)
	s.app = openServicePkg.NewAppService(repos.app, lazyCacheMgr, lazyCacheMgr, cfg.Security.AESKey, s.ipac, repos.ipac, storageMgr, configWatcher, rateLimiter, tm)
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

	// Client content services（共享 Repository + cacheFast；CategoryService 委托 admin 实现避免重复）
	s.contentArticleClient = contentClientService.NewArticleService(repos.contentArticle, repos.contentCategory, lazyCacheMgr, configWatcher)
	s.contentBannerGroupClient = contentClientService.NewBannerGroupService(repos.contentBannerGroup, lazyCacheMgr, configWatcher)
	s.contentBannerItemClient = contentClientService.NewBannerItemService(repos.contentBannerItem)
	s.contentCategoryClient = contentClientService.NewCategoryService(s.contentCategoryAdmin)

	if err := s.storageConfig.LoadAllConfigs(context.Background()); err != nil {
		slog.Error("load storage configs failed", "err", err)
	}

	s.oauthBinding = userServicePkg.NewOAuthBindingService(repos.user, tm, lazyCacheMgr)

	return s
}
