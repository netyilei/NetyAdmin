package cache

import (
	"fmt"
	"time"
)

// 定义系统统一的缓存 Tag 集合
// 使用 Tag 可以在进行数据更新时，一次性批量失效相关联的所有缓存 Key。
const (
	// TTL 默认时间
	TTL_Default = 24 * time.Hour
	// TTL_RBAC RBAC 相关缓存时长：3小时
	TTL_RBAC = 3 * time.Hour

	// Tags
	TagSystemConfig = "sys:config"
	TagRBACMenu     = "rbac:menu"
	TagRBACRole     = "rbac:role"
	TagRBACAPI      = "rbac:api"
	TagAdminInfo    = "admin:info"
	// TagDict 字典缓存标签前缀
	TagDictPrefix = "dict:"
	// TagStorageConfig 存储配置缓存标签
	TagStorageConfig = "storage:config"
	// TagContentCategoryTree 内容分类树缓存标签
	TagContentCategoryTree = "content:category:tree"
	// TagMsgTemplate 消息模板缓存标签
	TagMsgTemplate = "msg:template"
	// TagIPAC IPAC 缓存标签
	TagIPAC = "ipac"
	// TagApp 开放平台应用缓存标签
	TagApp = "open:app"
)

// 定义系统统一的缓存 Key 生成函数
// 严禁在业务代码中硬编码 Key 字符串。
// 【注意】这里的 Key 不需要也不应该包含全局前缀 (如 "so:")。
// 全局前缀由 LazyCacheManager.buildKey() 在最终读写时自动拼接。
// 我们只需要维护业务相关的 key 结构即可。

// KeyDictData 字典数据缓存 Key
func KeyDictData(dictCode string) string {
	return fmt.Sprintf("dict:data:%s", dictCode)
}

// TagDict 字典缓存 Tag
func TagDict(dictCode string) string {
	return TagDictPrefix + dictCode
}

// KeyAllApis 全量 API 列表缓存
func KeyAllApis() string {
	return "rbac:all:apis"
}

// KeyMenuTree 全量菜单树缓存
func KeyMenuTree() string {
	return "rbac:tree:menu"
}

// KeyMenuButtonTree 菜单按钮混合授权树
func KeyMenuButtonTree() string {
	return "rbac:tree:button"
}

// KeyMenuApiTree 菜单API混合授权树
func KeyMenuApiTree() string {
	return "rbac:tree:api"
}

// KeyRoleMenus 角色拥有的菜单 ID 集合
func KeyRoleMenus(roleID uint) string {
	return fmt.Sprintf("rbac:role:%d:menus", roleID)
}

// KeyRoleButtons 角色拥有的按钮 ID 集合
func KeyRoleButtons(roleID uint) string {
	return fmt.Sprintf("rbac:role:%d:buttons", roleID)
}

// KeyRoleApis 角色拥有的 API 对象集合 (用于鉴权)
func KeyRoleApis(roleCode string) string {
	return fmt.Sprintf("rbac:role:%s:apis", roleCode)
}

// KeyAdminInfo 管理员基础信息与权限快照
func KeyAdminInfo(adminID uint) string {
	return fmt.Sprintf("admin:%d:info", adminID)
}

// KeySystemConfigs 按组获取配置
func KeySystemConfigs(group string) string {
	return fmt.Sprintf("sys:config:group:%s", group)
}

// KeySystemConfig 系统配置缓存 Key
func KeySystemConfig() string {
	return "cache:sys:config"
}

// KeyVerificationCode 验证码缓存 Key
// type: register, reset_password, etc.
// target: phone or email
func KeyVerificationCode(scene, target string) string {
	return fmt.Sprintf("auth:verify:%s:%s", scene, target)
}

// KeyRBACMenu 路由菜单树缓存 Key
func KeyRBACMenu(roleID uint) string {
	return fmt.Sprintf("cache:rbac:menu:%d", roleID)
}

// KeyRBACAPI API 权限白名单缓存 Key
func KeyRBACAPI(roleID uint) string {
	return fmt.Sprintf("cache:rbac:api:%d", roleID)
}

// KeyRBACRole 角色信息缓存 Key
func KeyRBACRole(roleID uint) string {
	return fmt.Sprintf("cache:rbac:role:%d", roleID)
}

// KeyAuthBlacklistRefreshToken RefreshToken 黑名单 Key
func KeyAuthBlacklistRefreshToken(token string) string {
	return fmt.Sprintf("auth:blacklist:refresh:%s", token)
}

// KeyRoleApiIDs 角色拥有的 API ID 列表 (用于编辑回显)
func KeyRoleApiIDs(roleID uint) string {
	return fmt.Sprintf("rbac:role:%d:api_ids", roleID)
}

// KeyErrorLogSuppress 错误日志防抖静默 Key
func KeyErrorLogSuppress(fingerprint string) string {
	return fmt.Sprintf("err_log:suppress:%s", fingerprint)
}

// KeyStorageConfigByID 存储配置单条缓存 Key
func KeyStorageConfigByID(id uint) string {
	return fmt.Sprintf("storage:config:id:%d", id)
}

// KeyStorageConfigDefault 默认存储配置缓存 Key
func KeyStorageConfigDefault() string {
	return "storage:config:default"
}

// KeyStorageConfigAllEnabled 所有启用的存储配置缓存 Key
func KeyStorageConfigAllEnabled() string {
	return "storage:config:all_enabled"
}

// KeyContentCategoryTree 内容分类全量树
func KeyContentCategoryTree() string {
	return "content:category:tree:all"
}

// KeyTaskEnabled 任务是否启用配置
func KeyTaskEnabled(taskName string) string {
	return fmt.Sprintf("task:%s:enabled", taskName)
}

// KeyTaskSpec 任务 Cron 表达式配置
func KeyTaskSpec(taskName string) string {
	return fmt.Sprintf("task:%s:spec", taskName)
}

// KeyTaskLock 任务分布式锁 Key
func KeyTaskLock(prefix, taskName string) string {
	if prefix != "" {
		return fmt.Sprintf("%s:task:lock:%s", prefix, taskName)
	}
	return fmt.Sprintf("task:lock:%s", taskName)
}

// KeyMsgTemplate 消息模板缓存 Key
func KeyMsgTemplate(code string) string {
	return fmt.Sprintf("msg:template:%s", code)
}

// KeyIPACReload IPAC 重载通知 Channel
func KeyIPACReload() string {
	return "netyadmin:ipac:reload"
}

// KeyAppRateLimit 开放平台限流 Key
func KeyAppRateLimit(appKey string) string {
	return fmt.Sprintf("open:ratelimit:%s", appKey)
}

// KeyAppNonce 开放平台 Nonce 防重放 Key
func KeyAppNonce(appKey, nonce string) string {
	return fmt.Sprintf("open:nonce:%s:%s", appKey, nonce)
}

// KeyAppInfo 应用信息缓存 Key
func KeyAppInfo(appKey string) string {
	return fmt.Sprintf("open:app:info:%s", appKey)
}

// KeyAppScopes 应用权限范围缓存 Key
func KeyAppScopes(appID string) string {
	return fmt.Sprintf("open:app:%s:scopes", appID)
}

// KeyAppAvailableScopes 可用权限列表缓存 Key
func KeyAppAvailableScopes() string {
	return "open:app:available_scopes"
}

// TagAppKey 应用 Key 标签
func TagAppKey(appKey string) string {
	return "app:" + appKey
}

// TagAppID 应用 ID 标签
func TagAppID(appID string) string {
	return "app:" + appID
}
