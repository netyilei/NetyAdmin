package redis

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/redis/go-redis/v9"

	"NetyAdmin/internal/config"
)

func NewClient(cfg *config.RedisConfig) (*redis.Client, error) {
	if cfg == nil || !cfg.Enabled {
		return nil, nil
	}

	poolSize := cfg.PoolSize
	if poolSize <= 0 {
		poolSize = 10 * runtime.GOMAXPROCS(0)
	}
	minIdle := cfg.MinIdleConns
	if minIdle <= 0 {
		minIdle = 10
	}
	poolTimeout := time.Duration(cfg.PoolTimeoutSec) * time.Second
	if poolTimeout <= 0 {
		poolTimeout = 3 * time.Second
	}

	client := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     poolSize,
		MinIdleConns: minIdle,
		PoolTimeout:  poolTimeout,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		client.Close() // 释放连接池，防止泄漏
		return nil, fmt.Errorf("连接Redis失败: %w", err)
	}

	return client, nil
}
