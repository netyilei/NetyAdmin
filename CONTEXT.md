# CONTEXT.md — NetyAdmin Domain Glossary

> 本文件是项目领域词汇表，为架构 review、AI 协作、新人 onboarding 提供统一的领域语言。
> 新增模块概念时必须在此登记；模糊术语在对话中澄清后必须立即更新此文件。

---

## 缓存领域 (Cache Domain)

### L1 / L2 / L3 三级缓存

| 层级 | 实现 | 角色 | 一致性策略 |
|:-----|:-----|:-----|:-----------|
| **L1** | BigCache (本地内存) | 配置类数据加速，进程内 | InvalidateByTags 广播失效，窗口期 <100ms |
| **L2** | Redis (共享) | 跨节点一致性，所有缓存数据 | Redis 删除一次全集群立即生效 |
| **L3** | DB (回源) | 真相源 | 无缓存时直接读取 |

### ConfigCache（配置缓存接口）

配置类数据的缓存接口，对应 **Fast 方法系列**（L1+L2 chain）。

- **持有者**：app_service、dict_service、menu_service、role_service、api_service、message_service 等配置类服务
- **铁律**：持有此接口的服务只能操作 L1+L2 chain，数据容忍秒级延迟
- **失效**：配置变更时通过 `InvalidateByTags` 广播清 L1
- **方法**：FetchFast、SetFast、GetFast、DeleteFast、InvalidateByTags、IsCacheEnabled

### SecurityCache（安全缓存接口）

安全类数据的缓存接口，对应 **非 Fast 方法系列**（L2 only）。

- **持有者**：token_store、verification、admin_auth、user_auth、pkg/auth/session 等安全类服务
- **铁律**：持有此接口的服务只能操作 L2 (Redis)，数据永不进入 L1 (BigCache)
- **失效**：Redis 删除一次即对整个集群立即生效，无 PubSub 窗口期
- **方法**：Fetch、Set、Get、Delete、Exists、SetNX、Incr、InvalidateByTags、IsCacheEnabled

### CacheLifecycle（缓存生命周期接口）

仅 `wire.go` 和 PubSub 订阅者使用的缓存基础设施接口。

- **持有者**：wire.go（启动装配）、TopicCacheInvalidation 订阅者
- **铁律**：服务层永不持有此接口，杜绝 SetEventBus / InvalidateL1ByTags 的越层访问
- **方法**：SetEventBus、InvalidateL1ByTags
- **已移除**：GetRedisClient（wire.go 直接传 redisClient 给消费者）

### 数据分类判定

| 数据类型 | 分类 | 接口 | 判定依据 |
|:---------|:-----|:-----|:---------|
| token 哈希、refresh token 黑名单 | 安全类 | SecurityCache | 登出/踢人需立即全集群生效 |
| 登录锁、重试计数器 | 安全类 | SecurityCache | 限流需跨节点一致 |
| 验证码、Nonce | 安全类 | SecurityCache | 防重放需原子性 |
| App 信息、Scopes、API Keys | 配置类 | ConfigCache | 变更少，容忍秒级延迟 |
| 字典数据、菜单树、角色权限 | 配置类 | ConfigCache | 变更少，容忍秒级延迟 |
| 消息模板、内容分类 | 配置类 | ConfigCache | 变更少，容忍秒级延迟 |
| Admin 信息（Profile） | 配置类 | ConfigCache | 变更少，容忍秒级延迟 |

---

## 认证领域 (Auth Domain)

### TokenStore（令牌存储）

会话令牌哈希的存储抽象。DB 是唯一真相源，Redis 缓存作为加速层在 store 内部组合，调用方不感知缓存存在。

- **接口位置**：`service/user.TokenStore`
- **实现**：单一实现 `tokenStore`（DB + Redis 缓存），无 dbTokenStore / cacheTokenStore 双实现
- **隔离**：admin 与 user 共用同一 store，通过 userID 自然隔离

### AppContext（应用上下文）

开放平台应用上下文，通过 `gin.Context` 在中间件与 handler 间传递。

- **字段**：ID、AppKey、StorageID（3 个字段，已清除 5 个死字段）
- **注入点**：`OpenPlatformAuth` 中间件
- **消费点**：storage_handler、user_handler

### OpenPlatformAuthPipeline（开放平台认证管线）

开放平台签名认证的管线模块（设计中的深化模块）。

- **职责**：验证签名请求，返回解析后的 AppContext 或类型化失败
- **步骤**：SignatureVerify → NonceGuard → RateLimiter → ScopeAuthorizer → AppResolver
- **测试面**：构造 SignedRequest，断言 AppContext，无需 gin 引擎

---

## 事务领域 (Transaction Domain)

### TransactionManager（事务管理器）

多步原子操作的事务边界管理器。Service 层通过 TM 控制事务边界，Repository 通过 `getDB(ctx)` 处理事务上下文。

- **API**：`WithTransaction(ctx, fn)`（线性多步写）、`Begin(ctx)` + `Commit/Rollback`（循环内独立事务）
- **铁律**：Repository 禁止自管事务；缓存失效在 Commit 之后
- **Tx 字段**：`Tx.DB` 应为包内可见（当前错误导出，待修复）

### FailClosedBatchDelete（fail-closed 批量删除）

批量删除的统一模式：逐条独立事务，业务规则拒绝跳过，DB 错误立即返回。

- **配方**：`for _, id := range ids { check exists → Begin → steps → Commit → invalidate cache }`
- **语义**：fail-closed（事务失败立即返回错误，已成功的不回滚）
- **抽取目标**：从 SHARED.md §七 的文字配方深化为可测试的泛型 helper

---

## 架构词汇 (Architecture Vocabulary)

> 以下词汇来自 `/codebase-design` 技能，用于架构 review 时的统一语言。

| 词汇 | 含义 |
|:-----|:-----|
| **Module（模块）** | 有明确边界的代码单元，对外暴露 interface |
| **Interface（接口）** | 模块对外的契约，是测试面 |
| **Depth（深度）** | interface 简单但实现丰富 = 深模块；interface ≈ 实现 = 浅模块 |
| **Seam（接缝）** | 模块间的可替换边界，一个 adapter = 假想接缝，两个 = 真实接缝 |
| **Adapter（适配器）** | 在接缝处转换协议的薄层 |
| **Leverage（杠杆）** | 修改一处影响多处的程度 |
| **Locality（局部性）** | 相关逻辑物理集中的程度 |
| **Deletion test（删除测试）** | 删除模块后复杂度是集中还是仅移动？集中 = 值得深化 |
