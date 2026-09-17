package cache

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"NetyAdmin/internal/config"
)

// TestGetAndDeleteLocalMode 本地降级模式（无 Redis）下 GetAndDelete 的语义：
// 缺失返回 redis.Nil（与 Redis GETDEL 统一）、命中返回值并删除、二次取值为 Nil。
// 验证码原子消费（VerifyAndClearCode）依赖该语义区分"不存在"与"故障"。
func TestGetAndDeleteLocalMode(t *testing.T) {
	mgr, err := NewLazyCacheManager(&config.RedisConfig{Enabled: false}, nil, nil)
	if err != nil {
		t.Fatalf("NewLazyCacheManager: %v", err)
	}
	ctx := context.Background()

	// 1. 缺失 → redis.Nil（而非任意错误）
	err = mgr.GetAndDelete(ctx, "test:gad:missing", new(string))
	if !errors.Is(err, redis.Nil) {
		t.Fatalf("missing key: err = %v, want redis.Nil", err)
	}

	// 2. 写入 → 原子取出并删除
	if err := mgr.Set(ctx, "test:gad:hit", "secret", time.Minute); err != nil {
		t.Fatalf("Set: %v", err)
	}
	var got string
	if err := mgr.GetAndDelete(ctx, "test:gad:hit", &got); err != nil {
		t.Fatalf("GetAndDelete: %v", err)
	}
	if got != "secret" {
		t.Fatalf("got = %q, want %q", got, "secret")
	}

	// 3. 二次取值 → 已被删除
	if err := mgr.GetAndDelete(ctx, "test:gad:hit", new(string)); !errors.Is(err, redis.Nil) {
		t.Fatalf("second GetAndDelete: err = %v, want redis.Nil", err)
	}
}
