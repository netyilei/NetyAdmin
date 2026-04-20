package redis

import "fmt"

// 该文件定义低级分布式原语（锁、Pub/Sub 频道、消息队列等）的 Key。
// 这些键通常不通过 LazyCacheManager 访问，需要手动处理前缀逻辑。
// 全局前缀由配置注入，这里使用常量或方法生成。

const (
	// DefaultPrefix 全局默认前缀
	DefaultPrefix = "netyadmin"
)

// ChannelConfigSync 配置/缓存热重载广播频道
func ChannelConfigSync(prefix string) string {
	if prefix == "" {
		prefix = DefaultPrefix
	}
	return fmt.Sprintf("%s:channel:config_sync", prefix)
}

// ChannelStorageSync 存储配置热重载广播频道
func ChannelStorageSync(prefix string) string {
	if prefix == "" {
		prefix = DefaultPrefix
	}
	return fmt.Sprintf("%s:channel:storage_sync", prefix)
}

// ChannelCacheInvalidation 缓存失效广播频道
func ChannelCacheInvalidation(prefix string) string {
	if prefix == "" {
		prefix = DefaultPrefix
	}
	return fmt.Sprintf("%s:channel:cache_invalidation", prefix)
}
