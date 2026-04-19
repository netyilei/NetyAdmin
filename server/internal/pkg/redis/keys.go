package redis

import "fmt"

// 该文件定义低级分布式原语（锁、Pub/Sub 频道、消息队列等）的 Key。
// 这些键通常不通过 LazyCacheManager 访问，需要手动处理前缀逻辑。
// 全局前缀由配置注入，这里使用常量或方法生成。

const (
	// DefaultPrefix 全局默认前缀
	DefaultPrefix = "so"
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

// KeyLockAccount 账号抢占/操作锁
func KeyLockAccount(prefix string, accountID uint) string {
	if prefix == "" {
		prefix = DefaultPrefix
	}
	return fmt.Sprintf("%s:lock:account:%d", prefix, accountID)
}

// KeyLockOrder 订单处理排他锁
func KeyLockOrder(prefix string, orderID uint) string {
	if prefix == "" {
		prefix = DefaultPrefix
	}
	return fmt.Sprintf("%s:lock:order:%d", prefix, orderID)
}

// KeyIdemLock 接口幂等执行锁
func KeyIdemLock(prefix string, userID uint, method, route, requestID string) string {
	if prefix == "" {
		prefix = DefaultPrefix
	}
	return fmt.Sprintf("%s:idem:lock:%d:%s:%s:%s", prefix, userID, method, route, requestID)
}
