package ratelimit

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisRateLimiter 基于 Redis Lua 的令牌桶限流器
type RedisRateLimiter struct {
	client *redis.Client
}

func NewRedisRateLimiter(client *redis.Client) *RedisRateLimiter {
	return &RedisRateLimiter{client: client}
}

// Allow 判断是否允许请求
// key: 限流标识 (如 app_key)
// rate: 填充速率 (每秒生成的令牌数)
// capacity: 桶容量 (最大突发量)
func (r *RedisRateLimiter) Allow(ctx context.Context, key string, rate int, capacity int) (bool, error) {
	if r.client == nil {
		return true, nil // Redis 未启用，默认放行 (或使用本地限流降级)
	}

	// Lua 脚本实现令牌桶
	// KEYS[1]: 令牌桶 Key
	// ARGV[1]: 填充速率 (Tokens per second)
	// ARGV[2]: 桶容量 (Bucket capacity)
	// ARGV[3]: 当前时间戳 (Unix seconds)
	// ARGV[4]: 请求令牌数 (Requested tokens, 默认为 1)
	script := `
local bucket_key = KEYS[1]
local rate = tonumber(ARGV[1])
local capacity = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local requested = tonumber(ARGV[4])

local last_tokens = tonumber(redis.call("HGET", bucket_key, "tokens"))
local last_time = tonumber(redis.call("HGET", bucket_key, "last_time"))

if last_tokens == nil then
    last_tokens = capacity
    last_time = now
end

-- 计算自上次请求以来生成的令牌数
local delta = math.max(0, now - last_time)
local generated = delta * rate
local current_tokens = math.min(capacity, last_tokens + generated)

local allowed = false
if current_tokens >= requested then
    current_tokens = current_tokens - requested
    allowed = true
end

redis.call("HSET", bucket_key, "tokens", current_tokens, "last_time", now)
redis.call("EXPIRE", bucket_key, 86400) -- 设置 1 天过期，防止 Key 堆积

return allowed and 1 or 0
`
	res, err := r.client.Eval(ctx, script, []string{key}, rate, capacity, time.Now().Unix(), 1).Result()
	if err != nil {
		return false, err
	}

	return res.(int64) == 1, nil
}
