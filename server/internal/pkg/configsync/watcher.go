package configsync

import (
	"context"
	"log"
	"strings"
	"sync"

	"NetyAdmin/internal/config"
	"NetyAdmin/internal/pkg/redis"
	systemRepo "NetyAdmin/internal/repository/system"

	goRedis "github.com/redis/go-redis/v9"
)

// ConfigWatcher 负责全量加载数据库配置至内存，并监听集群配置热更通知
type ConfigWatcher interface {
	// IsCacheEnabled 实现了 Cache Manager 所需的接口
	IsCacheEnabled(moduleName string) bool

	// GetConfig 获取全局任意配置
	GetConfig(groupName, key string) (string, bool)

	// GetGroupConfigs 获取某个分组下的所有配置
	GetGroupConfigs(groupName string) map[string]string

	// WatchBlocking 阻塞式监听变更频道
	WatchBlocking(ctx context.Context)

	// ForceReload 强制触发重新从数据库拉取
	ForceReload(ctx context.Context) error
}

type configWatcher struct {
	repo        systemRepo.ConfigRepository
	redisClient *goRedis.Client
	redisCfg    *config.RedisConfig

	sync.RWMutex
	memory map[string]string // 存储格式为 "group:key" -> value
}

func NewConfigWatcher(repo systemRepo.ConfigRepository, redisClient *goRedis.Client, redisCfg *config.RedisConfig) ConfigWatcher {
	w := &configWatcher{
		repo:        repo,
		redisClient: redisClient,
		redisCfg:    redisCfg,
		memory:      make(map[string]string),
	}

	// 初始化立刻加载一次
	if err := w.ForceReload(context.Background()); err != nil {
		log.Printf("[Config Watcher] 首次加载全局配置失败: %v", err)
	}
	return w
}

func (w *configWatcher) buildMemKey(group, key string) string {
	return group + ":" + key
}

func (w *configWatcher) ForceReload(ctx context.Context) error {
	configs, err := w.repo.GetAll(ctx)
	if err != nil {
		return err
	}

	newMem := make(map[string]string, len(configs))
	for _, config := range configs {
		newMem[w.buildMemKey(config.GroupName, config.ConfigKey)] = config.ConfigValue
	}

	w.Lock()
	w.memory = newMem
	w.Unlock()

	log.Printf("[Config Watcher] 成功从 DB 同步 %d 条配置记录", len(configs))
	return nil
}

func (w *configWatcher) GetConfig(groupName, key string) (string, bool) {
	w.RLock()
	defer w.RUnlock()

	val, exists := w.memory[w.buildMemKey(groupName, key)]
	return val, exists
}

func (w *configWatcher) GetGroupConfigs(groupName string) map[string]string {
	w.RLock()
	defer w.RUnlock()

	result := make(map[string]string)
	prefix := groupName + ":"
	for k, v := range w.memory {
		if strings.HasPrefix(k, prefix) {
			key := strings.TrimPrefix(k, prefix)
			result[key] = v
		}
	}
	return result
}

// IsCacheEnabled 实现 SwitchChecker 接口
func (w *configWatcher) IsCacheEnabled(moduleName string) bool {
	val, exists := w.GetConfig("cache_switches", moduleName)
	if !exists {
		// 默认保险策略：查不到配置默认允许缓存（以保持旧逻辑兼容性）
		return true
	}
	return strings.ToLower(val) == "true" || val == "1"
}

// WatchBlocking 如果启用了 Redis，则订阅配置修改通知，实现热发布
func (w *configWatcher) WatchBlocking(ctx context.Context) {
	if err := w.ForceReload(ctx); err != nil {
		log.Printf("[Config Watcher] 首次加载全局配置失败: %v", err)
	}

	if w.redisCfg == nil || !w.redisCfg.Enabled || w.redisClient == nil {
		log.Println("[Config Watcher] 当前为无 Redis 模式，配置热更通道(Pub/Sub)未开启。")
		return
	}

	// 使用统一的 Redis Channel 定义
	// 这里的 redis 指的是 "NetyAdmin/internal/pkg/redis"
	channel := redis.ChannelConfigSync(w.redisCfg.Prefix)
	sub := w.redisClient.Subscribe(ctx, channel)
	defer sub.Close()

	ch := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-ch:
			log.Printf("[Config Watcher] 收到 Redis Pub/Sub 配置热更信号: %v, 即将刷新内存...", msg.Payload)
			_ = w.ForceReload(ctx)
		}
	}
}
