// Package auth 提供 BFF 多端共享的会话令牌工具与登录/刷新流程的可复用单元。
//
// 设计原则（RULES.md §0.1 + §0.3）：
// admin 与 user 两端登录/刷新流程存在同构片段（密码失败计数、token 签发、tokenStore 写入），
// 本包只抽取这些**真正同构**的片段，不强行抽取整个流程（避免过度抽象引入隐式状态传递）。
// 流程编排仍由各端 service 自行控制，差异点（验证码、查找方式、claims 类型）留在 service 层。
package auth

import (
	"context"
	"fmt"
	"strconv"
	"time"

	userEntity "NetyAdmin/internal/domain/entity/user"
)

// UserServiceTokenStore 是 service/user.TokenStore 的镜像接口，
// 避免 pkg/auth 反向依赖 service 层（保持依赖方向：service → pkg）。
type UserServiceTokenStore interface {
	Create(ctx context.Context, hash *userEntity.UserTokenHash) error
	Get(ctx context.Context, userID, tokenHash string) (*userEntity.UserTokenHash, error)
	Delete(ctx context.Context, userID, tokenHash string) error
	DeleteAll(ctx context.Context, userID string) error
}

// CacheManager 是 pkg/cache.LazyCacheManager 的镜像接口。
// 仅声明本包用到的方法，避免暴露完整 cache 接口。
type CacheManager interface {
	Get(ctx context.Context, key string, dest interface{}) error
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
}

// LoginLockConfig 是登录锁定策略的参数。
// admin 端硬编码（maxRetry=5, lockDuration=15min, retryTTL=10min），
// user 端从 configWatcher 读取。两端各自构造此 struct 传入 HandlePasswordWrong。
type LoginLockConfig struct {
	MaxRetry     int           // 触发锁定的失败次数阈值
	LockDuration time.Duration // 锁定时长
	RetryTTL     time.Duration // 失败计数缓存 TTL
}

// HandlePasswordWrong 处理密码错误：递增失败计数，达到阈值则锁定账户。
//
// 抽取自 admin_auth.go / user_auth.go 中两段高度同构的代码（重构清单 B-AUTH-5）。
// 两端的差异仅在策略参数（LoginLockConfig）和 key 生成方式（lockKey/retryKey 由调用方传入）。
//
// 返回值：
//   - locked=true 表示本次错误触发了锁定，msg 为锁定提示
//   - locked=false 表示未触发锁定，msg 为剩余次数提示
//
// 调用方应将 msg 包装为 errorx.CodePasswordWrong 或 errorx.CodeUserLocked 返回。
func HandlePasswordWrong(
	ctx context.Context,
	cacheMgr CacheManager,
	lockKey, retryKey string,
	cfg LoginLockConfig,
) (locked bool, msg string) {
	var retryVal string
	retryCount := 0
	if err := cacheMgr.Get(ctx, retryKey, &retryVal); err == nil && retryVal != "" {
		retryCount, _ = strconv.Atoi(retryVal)
	}
	retryCount++

	if retryCount >= cfg.MaxRetry {
		_ = cacheMgr.Set(ctx, lockKey, "1", cfg.LockDuration)
		_ = cacheMgr.Delete(ctx, retryKey)
		return true, fmt.Sprintf("密码错误次数过多，账户已被锁定 %v", cfg.LockDuration)
	}

	_ = cacheMgr.Set(ctx, retryKey, strconv.Itoa(retryCount), cfg.RetryTTL)
	return false, fmt.Sprintf("密码错误，剩余尝试次数 %d 次", cfg.MaxRetry-retryCount)
}

// ClearLoginRetry 清理登录失败计数（登录成功路径）。
// 抽取自两端 Login 中的 `_ = s.cacheMgr.Delete(ctx, retryKey)`。
func ClearLoginRetry(ctx context.Context, cacheMgr CacheManager, retryKey string) {
	_ = cacheMgr.Delete(ctx, retryKey)
}

// StoreSessionPair 写入 access + refresh 两个 token 哈希到 tokenStore。
//
// 抽取自两端 Login/RefreshToken 中的复制粘贴（重构清单 B-AUTH-5）。
// tokenStore 为 nil 时跳过（设计允许关闭会话存储）。
// userIDKey 由调用方决定（admin 用 AdminTokenKey，user 用 ULID）。
func StoreSessionPair(
	ctx context.Context,
	tokenStore UserServiceTokenStore,
	userIDKey string,
	access, refresh string,
	accessExp, refreshExp time.Time,
) error {
	if tokenStore == nil {
		return nil
	}
	if err := tokenStore.Create(ctx, &userEntity.UserTokenHash{
		UserID:    userIDKey,
		TokenHash: HashToken(access),
		ExpiredAt: accessExp,
	}); err != nil {
		return err
	}
	return tokenStore.Create(ctx, &userEntity.UserTokenHash{
		UserID:    userIDKey,
		TokenHash: HashToken(refresh),
		ExpiredAt: refreshExp,
	})
}

// ReplaceSessionForRefresh 刷新令牌场景的会话替换：
// 先 DeleteAll 清旧会话哈希（含旧 access token，防泄露被继续使用），再写入新对。
//
// 抽取自两端 RefreshToken 中同构的 DeleteAll + Create 序列。
// 不递增 TokenVersion（refresh 不应失效其他设备合法会话）。
func ReplaceSessionForRefresh(
	ctx context.Context,
	tokenStore UserServiceTokenStore,
	userIDKey string,
	access, refresh string,
	accessExp, refreshExp time.Time,
) error {
	if tokenStore == nil {
		return nil
	}
	_ = tokenStore.DeleteAll(ctx, userIDKey)
	return StoreSessionPair(ctx, tokenStore, userIDKey, access, refresh, accessExp, refreshExp)
}
