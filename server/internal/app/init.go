package app

import (
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"NetyAdmin/internal/config"
)

func InitDB(cfg *config.Config) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.DBName,
		cfg.Database.SSLMode,
	)

	// 设置 GORM 日志级别，生产级别默认只记录错误，开发调试模式记录警告和错误
	newLogger := logger.Default.LogMode(logger.Error)
	if cfg.Server.Mode == "debug" {
		newLogger = logger.Default.LogMode(logger.Warn) // Warn 级别不会打印正常执行的 SQL 源码，只会打印慢查询和错误
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: newLogger,
	})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxIdleConns(cfg.Database.MaxIdle)
	sqlDB.SetMaxOpenConns(cfg.Database.MaxOpen)
	sqlDB.SetConnMaxLifetime(time.Hour)

	return db, nil
}
