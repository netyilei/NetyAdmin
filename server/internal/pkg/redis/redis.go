package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"NetyAdmin/internal/config"
)

func NewClient(cfg *config.RedisConfig) (*redis.Client, error) {
	if cfg == nil || !cfg.Enabled {
		return nil, nil
	}
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("连接Redis失败: %w", err)
	}

	return client, nil
}
