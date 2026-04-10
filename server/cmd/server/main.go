package main

import (
	"log"

	"NetyAdmin/internal/app"
	"NetyAdmin/internal/config"
	"NetyAdmin/internal/pkg/recovery"
)

func main() {
	// 1. 全局 Panic 恢复
	defer recovery.GlobalRecovery()

	// 2. 加载配置
	cfg, err := config.Load("config.toml")
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 3. 初始化数据库 (GORM)
	db, err := app.InitDB(cfg)
	if err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}

	// 4. 引导启动整个应用依赖 (DI)
	application, err := app.Bootstrap(cfg, db)
	if err != nil {
		log.Fatalf("应用引导失败: %v", err)
	}

	// 5. 运行服务器
	if err := application.Run(); err != nil {
		log.Fatalf("服务器运行错误: %v", err)
	}
}
