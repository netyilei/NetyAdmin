package database

import (
	"context"
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"
)

type HealthChecker struct {
	db            *gorm.DB
	checkInterval time.Duration
	retryInterval time.Duration
	maxRetries    int
	stopChan      chan struct{}
	onReconnect   func()
	onDisconnect  func(error)
}

type HealthCheckerOption func(*HealthChecker)

func WithCheckInterval(d time.Duration) HealthCheckerOption {
	return func(h *HealthChecker) {
		h.checkInterval = d
	}
}

func WithRetryInterval(d time.Duration) HealthCheckerOption {
	return func(h *HealthChecker) {
		h.retryInterval = d
	}
}

func WithMaxRetries(n int) HealthCheckerOption {
	return func(h *HealthChecker) {
		h.maxRetries = n
	}
}

func WithOnReconnect(fn func()) HealthCheckerOption {
	return func(h *HealthChecker) {
		h.onReconnect = fn
	}
}

func WithOnDisconnect(fn func(error)) HealthCheckerOption {
	return func(h *HealthChecker) {
		h.onDisconnect = fn
	}
}

func NewHealthChecker(db *gorm.DB, opts ...HealthCheckerOption) *HealthChecker {
	h := &HealthChecker{
		db:            db,
		checkInterval: 30 * time.Second,
		retryInterval: 5 * time.Second,
		maxRetries:    5,
		stopChan:      make(chan struct{}),
	}

	for _, opt := range opts {
		opt(h)
	}

	return h
}

func (h *HealthChecker) Start() {
	go h.run()
}

func (h *HealthChecker) Stop() {
	close(h.stopChan)
}

func (h *HealthChecker) run() {
	ticker := time.NewTicker(h.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-h.stopChan:
			return
		case <-ticker.C:
			if err := h.check(); err != nil {
				log.Printf("[DB健康检查] 数据库连接异常: %v", err)
				if h.onDisconnect != nil {
					h.onDisconnect(err)
				}
				h.reconnect()
			}
		}
	}
}

func (h *HealthChecker) check() error {
	sqlDB, err := h.db.DB()
	if err != nil {
		return fmt.Errorf("获取数据库连接失败: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("数据库 ping 失败: %w", err)
	}

	return nil
}

func (h *HealthChecker) reconnect() {
	for i := 0; i < h.maxRetries; i++ {
		log.Printf("[DB重连] 尝试重连数据库 (%d/%d)...", i+1, h.maxRetries)

		if err := h.check(); err == nil {
			log.Printf("[DB重连] 数据库连接恢复")
			if h.onReconnect != nil {
				h.onReconnect()
			}
			return
		}

		time.Sleep(h.retryInterval)
	}

	log.Printf("[DB重连] 重连失败，已达到最大重试次数")
}

func (h *HealthChecker) IsHealthy() bool {
	return h.check() == nil
}

func Ping(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	return sqlDB.PingContext(ctx)
}
