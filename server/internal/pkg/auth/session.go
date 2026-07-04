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
	// Incr 原子自增计数器并设置 TTL（仅在第一次自增时设置 TTL）。
	// 返回自增后的当前值。底层应基于 Redis INCR + EXPIRE 实现，
	// Redis 不可用时返回 error（fail-closed，不允许跳过原子计数）。
	// 用于登录失败计数等需防 TOCTOU 竞态的场景。
	Incr(ctx context.Context, key string, ttl time.Duration) (int64, error)
}

// LoginLockConfig 是登录锁定策略的参数。
// admin 端硬编码（maxRetry=5, lockDuration=15min, retryTTL=10min），
// user 端从 configWatcher 读取。两端各自构造此 struct 传入 HandlePasswordWrong。
type LoginLockConfig struct {
	MaxRetry     int           // 触发锁定的失败次数阈值
	LockDuration time.Duration // 锁定时长
	RetryTTL     time.Duration // 失败计数缓存 TTL
}

// HandlePasswordWrong 处理密码错误：原子递增失败计数，达到阈值则锁定账户。
//
// 抽取自 admin_auth.go / user_auth.go 中两段高度同构的代码（重构清单 B-AUTH-5）。
// 两端的差异仅在策略参数（LoginLockConfig）和 key 生成方式（lockKey/retryKey 由调用方传入）。
//
// 实现说明（B2 原子化改造）：
//   - 旧实现 Get-Set 非原子，存在 TOCTOU 竞态（并发登录失败可能丢失计数导致绕过锁定）
//   - 新实现改用 cacheMgr.Incr（底层 Redis INCR + EXPIRE 原子操作）
//   - Incr 返回值 = 当前累计失败次数（含本次）
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
	// 原子自增失败计数，Incr 内部会在首次设置时配置 TTL
	retryCount, err := cacheMgr.Incr(ctx, retryKey, cfg.RetryTTL)
	if err != nil {
		// Incr 失败（如 Redis 不可用）：fail-closed 直接锁定，避免绕过计数
		_ = cacheMgr.Set(ctx, lockKey, "1", cfg.LockDuration)
		_ = cacheMgr.Delete(ctx, retryKey)
		return true, fmt.Sprintf("密码错误次数过多，账户已被锁定 %v", cfg.LockDuration)
	}

	// Incr 返回值即为累计失败次数，达到阈值则锁定
	if int(retryCount) >= cfg.MaxRetry {
		_ = cacheMgr.Set(ctx, lockKey, "1", cfg.LockDuration)
		_ = cacheMgr.Delete(ctx, retryKey)
		return true, fmt.Sprintf("密码错误次数过多，账户已被锁定 %v", cfg.LockDuration)
	}

	remaining := cfg.MaxRetry - int(retryCount)
	return false, fmt.Sprintf("密码错误，剩余尝试次数 %d 次", remaining)
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
// Deprecated: 此方法调用 DeleteAll 会清除该用户所有 token hash，多设备登录场景下
// 刷新一个 token 会踢掉该用户其他所有设备的合法会话（P1-A BUG）。
// 新代码请使用 DeleteAndReplaceSession：仅删旧 refresh hash 不影响其他设备。
//
// 抽取自两端 RefreshToken 中同构的 DeleteAll + Create 序列。
// 不递增 TokenVersion（refresh 不应失效其他设备合法会话）。
//
// fail-closed 语义（A2）：
//   - 旧实现 `_ = tokenStore.DeleteAll(...)` 吞错，DeleteAll 失败时旧会话仍残留，
//     旧 access token 可能被继续使用（安全风险）
//   - 新实现 DeleteAll 失败时立即返回 error，整个 RefreshToken 流程失败，
//     调用方应返回错误响应，用户保留旧 refresh token 可重试
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
	// fail-closed：DeleteAll 失败时不再继续写入新会话，避免旧会话残留导致旧 token 可被继续使用
	if err := tokenStore.DeleteAll(ctx, userIDKey); err != nil {
		return err
	}
	return StoreSessionPair(ctx, tokenStore, userIDKey, access, refresh, accessExp, refreshExp)
}

// DeleteAndReplaceSession 刷新令牌场景的会话替换（不影响其他设备）：
// 仅删除当前会话的旧 refresh token hash，再写入新 access + refresh hash 对。
//
// 与 ReplaceSessionForRefresh 的区别（P1-A 修复）：
//   - ReplaceSessionForRefresh 调用 DeleteAll 清除该用户所有 token hash，多设备登录场景下
//     刷新一个 token 会踢掉该用户其他所有设备的合法会话
//   - DeleteAndReplaceSession 仅删旧 refresh hash，不调用 DeleteAll，不影响其他设备
//
// 设计说明：
//   - 旧 access token hash 不删除：当前 RefreshToken 入参仅含旧 refresh token，
//     无法定位旧 access hash；旧 access 由其自然过期或下次 Logout 时清理。
//     多设备场景下其他设备的 access token 不受影响。
//   - fail-closed 语义：Delete 失败时返回 error，整个 RefreshToken 流程失败，
//     调用方应返回错误响应，用户保留旧 refresh token 可重试。
func DeleteAndReplaceSession(
	ctx context.Context,
	tokenStore UserServiceTokenStore,
	userIDKey string,
	oldRefresh, newAccess, newRefresh string,
	accessExp, refreshExp time.Time,
) error {
	if tokenStore == nil {
		return nil
	}
	// 删旧 refresh hash（fail-closed：失败则整个刷新失败，避免旧 refresh 被继续使用）
	if err := tokenStore.Delete(ctx, userIDKey, HashToken(oldRefresh)); err != nil {
		return err
	}
	// 写新 access + refresh hash 对
	return StoreSessionPair(ctx, tokenStore, userIDKey, newAccess, newRefresh, accessExp, refreshExp)
}
