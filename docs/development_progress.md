# NetyAdmin 开发进度跟踪文档 (Development Progress)

本文档用于记录 NetyAdmin 扩展模块的开发状态。无论会话中断还是重新开启，请首先查看此文档以确定当前进度。

---

## 📅 最后更新日期: 2026-04-17 (AI Assistant)

## 🚦 模块总体进度概览

| 模块名称 | 状态 | 进度 | 依赖项 | 负责人 |
| :--- | :--- | :--- | :--- | :--- |
| **IP 访问控制 (IPAC)** | 🔵 开发完成 | 100% | 基座缓存模块 | AI Assistant |
| **开放平台 (Open Platform)** | 🔵 开发完成 | 100% | IP 访问控制 | AI Assistant |
| 统一消息发送 (Message Hub) | 🔵 开发完成 | 100% | 系统任务引擎 | AI Assistant |
| 用户模块 (User Module) | 🔵 开发完成 | 100% | 开放平台 | AI Assistant |
| **用户与消息集成** | 🔵 开发完成 | 100% | 用户模块 + 消息模块 | AI Assistant |

---

## 🛠️ 详细进度详情

### 0. 核心基础设施重构 (Prerequisites)
- [x] **JWT 模块重构**: 将 `pkg/jwt` 重构为支持多实例、自定义 Claims 的通用工厂。现有的 `Claims` 结构体过于耦合 Admin 业务。
- [x] **ULID 工具集成**: 引入 `github.com/oklog/ulid/v2` 并封装工具类。基座目前缺失高性能可排序 ID 生成器。
- [x] **后端 AES 加密工具**: 实现 `pkg/utils/crypto.go`。用于对 `AppSecret` 和消息驱动凭证进行应用级加密存储。基座目前仅前端有相关逻辑。
- [x] **任务引擎优先级增强**: 扩展 `pkg/task` 的队列驱动，支持按优先级消费。已实现本地多级 Channel 与 Redis 多级 List。
- [x] **错误码与国际化对齐**: 补充业务域 (1011-1014) 的错误码定义与前端 `request.ts` 映射。
- [x] **缓存模块增强**: `LazyCacheManager` 支持分布式限流 (`RateLimit`) 与原子操作 (`SetNX`)，并统一管理缓存 Key。

### 1. IP 访问控制 (IPAC)
- [x] 数据库设计与 Schema 定义 (已确认与现有 GORM 基座兼容，AppID 统一为 ULID String)
- [x] 高性能匹配算法设计 (通过 net.IPNet 集合实现高效匹配，支持 CIDR)
- [x] 分级管理（全局/应用）逻辑设计
- [x] 数据库迁移脚本实现 (`table_ipac.sql`, `data_ipac.sql`，包含字典数据初始化)
- [x] 领域层 (Entity/Repo) 实现
- [x] 中间件拦截逻辑实现 (已集成到全局中间件链条)
- [x] **分布式同步支持**: 通过 Redis Pub/Sub 实现全网节点 IPAC 缓存同步。
- [x] Admin 后端管理接口实现
- [x] Admin-Web 前端 UI 实现 (支持 CRUD、分页、批量删除、字典联动)

### 2. 开放平台 (Open Platform)
- [x] 签名算法 (HMAC-SHA256) 设计 (依赖后端 AES 工具进行 Secret 解密)
- [x] 令牌桶分布式限流设计 (利用缓存模块 `RateLimit` 实现)
- [x] 应用生命周期与权限 Scope 设计
- [x] 数据库迁移脚本实现 (`table_open_platform.sql`, `data_open_platform.sql`，含权限与字典)
- [x] 签名验证中间件实现 (支持 Nonce 防重放与 IPAC 联动)
- [x] 领域层 (Entity/Repo/Service) 实现
- [x] Admin 后端管理接口实现
- [x] Admin-Web 前端 UI 实现 (应用管理、权限分配、导航配置)
- [x] 国际化与字典模块集成
- [x] 接口权限 (Scope) 动态管理页面实现
- [x] **客户端 API (client/v1) 入口实现**: 已实现基础路由架构、签名验证集成与示例 Echo Handler。

### 3. 统一消息发送 (Message Hub)
- [x] 驱动化架构设计 (SMS/Email/Push)
- [x] 异步任务与优先级队列设计 (强依赖“任务引擎优先级增强”)
- [x] 水位线已读状态优化设计 (依赖用户表扩展水位线字段)
- [x] 数据库迁移脚本实现 (含 RBAC 权限与字典数据)
- [x] 核心驱动实现 (SMTP/Mock SMS)
- [x] 领域层与异步任务处理器实现 (集成 `LazyCacheManager` 缓存模板)
- [x] Admin 后端管理接口实现
- [x] Admin-Web 前端 UI 实现 (模板管理、发送记录、导航配置)
- [x] **多通道发送入口 (Admin-Web)**: 已集成到发送记录详情与测试发送功能中。
- [x] 国际化与字典模块集成

### 4. 用户模块 (User Module)
- [x] 独立 JWT 认证体系设计 (强依赖“JWT 模块重构”)
- [x] Token 哈希存储方案设计
- [x] 密码加密与登录锁定设计 (使用 bcrypt 自动盐值与 Token 哈希校验)
- [x] 数据库迁移脚本实现 (包含 `users` 与 `user_token_hashes` 表及 RBAC 权限)
- [x] 客户端 API (注册/登录/资料/登出) 实现 (集成 `UserJWTAuth` 中间件)
- [x] Admin 后端管理接口实现 (用户列表、状态切换、删除)
- [x] Admin-Web 前端 UI 实现 (用户管理页面)
- [x] 国际化与字典模块集成

### 5. 用户与消息集成
- [x] 图形验证码与消息验证码协同设计
- [x] 注册/找回密码业务流程设计
- [x] 集成 API 实现
- [x] 前端业务逻辑适配

---

## 📝 开发者备忘录 (Re-entry Notes)
1. **JWT 重构**: 必须先重构 `pkg/jwt` 以支持多实例密钥与不同载荷（Admin/User）。
2. **ULID 确认为主键**: 确认为所有新业务模块（开放平台、消息记录等）采用 ULID (`varchar(26)`)，兼顾索引性能与可排序性。
3. **自适应架构原则**: 
   - 必须支持“无 Redis 自动降级”：单机部署时利用 BigCache 和本地 Channel 保证极致性能。
   - 分布式部署时：利用 Redis Lua 脚本保证限流与任务分发的绝对准确性。
4. **AES 加密**: 敏感配置项（AppSecret、驱动 Key）写入数据库前必须执行 AES 加密存储。
5. **开发规范强制要求 (必须执行)**: 
   - **前端页面与导航**: 必须完整实现 `admin-web` 页面，并完成左侧菜单导航配置。
   - **RBAC 权限体系**: 必须完善所有相关权限，包括菜单 (Menu) 和 API 权限。注意：`admin_button` 表仅有 `code` 和 `label` 字段，迁移脚本严禁写入 `name` 和 `description`。
   - **字典模块集成**: 所有具有枚举性质的字段必须对接系统字典模块 (Dictionary)，严禁硬编码。
   - **国际化 (i18n)**: 所有 UI 文本、状态码消息必须通过 i18n 语言包管理，严禁在代码中硬编码中文或英文。
   - **分布式一致性**: 所有带内存缓存的 Service (如 IPAC) 必须支持 Redis Pub/Sub 同步。
   - **ID 一致性**: 开放平台应用 ID 与 IPAC 的关联必须统一使用 ULID String。
