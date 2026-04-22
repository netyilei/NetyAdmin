package app

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"NetyAdmin/internal/config"
	"NetyAdmin/internal/pkg/database"
	"NetyAdmin/internal/pkg/pubsub"
	"NetyAdmin/internal/pkg/task"
	logService "NetyAdmin/internal/service/log"
)

type App struct {
	cfg             *config.Config
	db              *gorm.DB
	engine          *gin.Engine
	dbHealthChecker *database.HealthChecker
	taskManager     *task.Manager
	logBus          logService.LogBusService
	eventBus        pubsub.EventBus
}

func NewApp(cfg *config.Config, db *gorm.DB, engine *gin.Engine, dbHealthChecker *database.HealthChecker, taskManager *task.Manager, logBus logService.LogBusService, eventBus pubsub.EventBus) *App {
	return &App{
		cfg:             cfg,
		db:              db,
		engine:          engine,
		dbHealthChecker: dbHealthChecker,
		taskManager:     taskManager,
		logBus:          logBus,
		eventBus:        eventBus,
	}
}

func (a *App) Run() error {
	addr := fmt.Sprintf(":%d", a.cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      a.engine,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// 1. Start task manager (Execute startup tasks like DB migration)
	if a.taskManager != nil {
		a.taskManager.Start(context.Background())
	}

	// 2. Start Web Server
	go func() {
		log.Printf("服务器启动在 %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("启动服务器失败: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("正在关闭服务器...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("服务器强制关闭: %v", err)
	}

	// Stop DB health checker
	if a.dbHealthChecker != nil {
		a.dbHealthChecker.Stop()
	}

	// Stop task manager
	if a.taskManager != nil {
		a.taskManager.Stop()
	}

	// Stop LogBus (flush all buckets)
	if a.logBus != nil {
		a.logBus.Stop()
	}

	// Stop PubSubBus (close Redis subscription goroutine)
	if a.eventBus != nil {
		_ = a.eventBus.Close()
	}

	log.Println("服务器已安全关闭")
	return nil
}
