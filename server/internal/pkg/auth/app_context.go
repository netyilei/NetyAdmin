// Package auth provides shared authentication context types used to pass
// identity / authorization state across middleware and handler layers
// without leaking entity references into gin.Context.
package auth

// AppContext 是开放平台应用上下文，仅包含 handler 实际需要的基础类型字段，
// 用于在中间件与 handler 之间传递应用信息，避免将 *open_platform.App entity
// 直接注入 gin.Context（违反「Handler 禁止类型断言 entity」红线）。
//
// 字段选择原则（Round 7 修正）：
// 只保留 handler 真正读取的字段。原 Task 15 把 Status / QuotaConfig / CacheTTL /
// IPFilterEnabled / RateLimitEnabled 5 个字段镜像进来，意图是「把验证逻辑下沉到
// handler」，但实际验证逻辑全部在中间件 + service 层用 entity 直接读，
// handler 从不消费这 5 个字段，属于过度设计的死字段，已删除。
//
// 中间件从 service 返回的 *open_platform.App 构造 AppContext 后通过
// c.Set("currentAppContext", appCtx) 注入 gin.Context，handler 统一通过
// c.Get("currentAppContext") 读取。
type AppContext struct {
	ID        string // ULID 应用 ID（与 AppKey 相同，业务唯一标识）
	AppKey    string // 应用 AppKey（与 ID 相同）
	StorageID uint   // 绑定的存储配置 ID
}

// 开放平台应用状态常量（与 entity open_platform.AppStatusDisabled / AppStatusEnabled
// 镜像）。中间件 / handler 通过本常量判断应用状态，避免直接 import entity 包
// （Task 15：中间件 / handler 不再 import openEntity）。
const (
	AppStatusDisabled = 0
	AppStatusEnabled  = 1
)
