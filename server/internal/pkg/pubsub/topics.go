package pubsub

// Topic 注册表
// 所有 PubSubBus 的 Topic 必须在此注册，严禁在业务代码中硬编码 Topic 字符串。
const (
	TopicConfigSync        = "config_sync"
	TopicStorageSync       = "storage_sync"
	TopicCacheInvalidation = "cache_invalidation"
	TopicIPACReload        = "ipac_reload"
	TopicCacheDelete       = "cache_delete" // payload: 完整缓存 key（已带 prefix），用于跨节点删 L1
)
