[English](README.en-US.md)

# NetyAdmin - 企业级后台管理系统基座

NetyAdmin 是一个基于 **Go + Gin** 后端和 **Vue 3 + TypeScript** 前端构建的企业级后台管理系统基座。采用现代化的 BFF (Backend For Frontend) 多端隔离架构，提供高性能、高可用、功能丰富的管理后台解决方案。

---

## ✨ 核心特性

### 🚀 现代化技术栈

- **前端**: Vue 3, TypeScript, Vite, Naive UI, UnoCSS, Pinia, Vue Router, vue-i18n
- **后端**: Go 1.21+, Gin, GORM (PostgreSQL), Redis (可选), JWT

### 🏗️ 清晰的架构设计

- **BFF 多端隔离**: Admin/Client 端物理隔离，避免业务逻辑混杂
- **分层架构**: 严格遵循 `router -> handler -> service -> repository -> entity` 调用链
- **API 版本控制**: 显式版本管理，支持平滑演进
- **依赖注入**: 使用 Wire 进行依赖装配，便于测试和替换

### 🔒 完善的认证与权限

- **JWT 认证**: 安全可靠的用户认证机制
- **RBAC 权限体系**: 基于角色、菜单、按钮、API 的精细化权限控制
- **动态路由**: 后端动态生成路由树，前端根据权限渲染菜单

### ⚡ 高性能与高可用

- **透明缓存**: Redis + BigCache 双引擎，支持动态开关和批量失效
- **统一消息总线**: PubSubBus 统一收口 Redis Pub/Sub，驱动化设计支持单机/集群切换
- **统一日志缓冲**: LogBus 异步聚合全系统日志，分级背压(P0/P1/P2)，双触发批量写入
- **配置热同步**: 基于 PubSubBus 实现全网配置实时同步
- **任务调度**: 内置任务引擎，支持定时任务、后台管理和日志持久化
- **数据库迁移**: 启动期自动执行 SQL 迁移脚本
- **Email 驱动**: 基于 go-simple-mail，支持 SSL/TLS、STARTTLS、多种 SMTP 认证方式

### 📦 丰富的功能模块

| 模块 | 功能 |
|------|------|
| **RBAC** | 管理员、角色、菜单、按钮、API 管理 |
| **用户模块** | C端用户体系、多端登录、TokenStore 抽象（缓存/数据库）、账户锁定与自动解锁、安全加密 |
| **消息中心** | 统一发送入口（SMS/Email/站内信）、模板渲染、异步重试、STARTTLS 支持 |
| **开放平台** | AppKey 认证、安全签名、分布式限流、Scope 权限、应用级存储绑定 |
| **IP 访问控制** | 全局/应用级 IP 治理、CIDR 匹配、高性能内存过滤 |
| **内容管理** | 分类、文章、Banner，支持富文本、定时发布、C端公开API |
| **存储管理** | 多存储源配置、上传凭证、上传记录、应用级存储隔离、Client端上传接口 |
| **日志审计** | 操作日志、错误日志、开放平台日志、任务日志，LogBus 统一缓冲，敏感字段脱敏 |
| **系统配置** | 动态字典、系统参数、任务调度 |
| **统一消息总线** | PubSubBus、驱动化设计、Topic 注册表、分布式缓存同步 |
| **验证码** | 图形验证码，支持多种类型和存储方案，场景化验证配置（登录/注册/找回密码） |

---

## 🛠️ 项目维护

### 维护分支(No User):** <https://github.com/netyilei/NetyAdmin/tree/maint-nouser>

## 📚 文档索引

### 架构设计文档

| 文档 | 说明 |
|------|------|
| [Server 架构设计与目录结构](docs/server-architecture.md) | 后端架构理念、分层设计、二次开发指南 |
| [Admin-Web 架构设计与目录结构](docs/admin-web-architecture.md) | 前端架构理念、目录规范、开发规范 |

### 模块详解文档

| 文档 | 说明 |
|------|------|
| [用户模块详解](docs/server-module-user.md) | C端用户体系、多端登录、TokenStore 抽象、账户锁定机制、登录存储介质 |
| [统一消息模块详解](docs/server-module-message.md) | 统一发送入口(SMS/Email/站内信)、驱动扩展、异步任务 |
| [开放平台详解](docs/server-module-open-platform.md) | AppKey认证、签名验证、分布式限流、Scope权限 |
| [IP访问控制详解](docs/server-module-ipac.md) | 高性能内存匹配、CIDR网段、分级治理 |
| [验证码模块详解](docs/server-module-captcha.md) | 验证码类型、存储方案、动态配置、场景化验证 |
| [缓存模块详解](docs/server-module-cache.md) | 双引擎缓存、Tags 批量失效、动态开关 |
| [统一消息总线详解](docs/server-module-pubsub.md) | PubSubBus 架构、驱动机制、Topic 注册、二次开发 |
| [任务系统详解](docs/server-module-task.md) | 任务调度、队列机制、后台管理 |
| [字典模块详解](docs/server-module-dict.md) | 动态字典、缓存策略、使用示例 |
| [内容管理模块详解](docs/server-module-content.md) | 文章、分类、Banner、定时发布 |
| [存储模块详解](docs/server-module-storage.md) | 对象存储、上传凭证、驱动扩展 |
| [日志模块详解](docs/server-module-log.md) | 操作日志、错误日志、LogBus 统一缓冲、敏感脱敏 |
| [数据迁移详解](docs/server-module-migration.md) | 迁移脚本、版本控制、幂等执行 |

### 开发规范文档

| 文档 | 说明 |
|------|------|
| [状态码规范](docs/status-codes.md) | 错误码编码规则、全量码表、新增流程 |
| [API 管理指南](docs/api-management.md) | 前后端 API 定义、新增流程、最佳实践 |
| [快速部署指南](docs/quick-deployment.md) | 环境准备、配置说明、部署步骤 |

### 客户端 API 文档

| 文档 | 说明 |
|------|------|
| [认证与签名指南](docs/client-api/00-authentication.md) | 开放平台签名、JWT Token、统一响应格式、错误码 |
| [登录/注册/找回密码](docs/client-api/01-auth.md) | 场景验证配置、验证码、登录注册流程 |
| [用户资料管理](docs/client-api/02-user.md) | 个人资料、修改密码、注销、上传凭证 |
| [内容管理](docs/client-api/03-content.md) | 分类树、文章列表/详情/点赞、Banner |
| [文件上传](docs/client-api/04-storage.md) | 上传凭证、直传流程、上传记录回调 |
| [站内信](docs/client-api/05-message.md) | 消息列表、详情、已读、未读数 |
| [Echo 测试](docs/client-api/06-echo.md) | 连通性测试接口 |

---

## 🚀 快速开始

### 环境要求

- **Go** >= 1.21
- **Node.js** >= 18
- **PostgreSQL** >= 14
- **Redis** >= 6.0（可选）

### 一键启动

```bash
# 1. 克隆代码
git clone https://github.com/netyilei/NetyAdmin.git
cd NetyAdmin

# 2. 启动服务端
cd server
# 编辑 config.toml 配置数据库
go mod download
go run cmd/server/main.go

# 3. 启动前端（新终端）
cd ../admin-web
pnpm install
pnpm dev
```

### 默认账号

- **账号**: `admin`
- **密码**: `admin123`

> ⚠️ **安全提示**: 部署后请立即修改默认密码！

---

## 🏗️ 项目结构

```
NetyAdmin/
├── server/                    # 后端服务（Go + Gin）
│   ├── cmd/server/           # 进程入口
│   ├── internal/             # 业务代码
│   │   ├── app/              # 应用启动
│   │   ├── domain/           # 领域模型
│   │   ├── interface/        # 接入层（BFF）
│   │   ├── pkg/              # 基础设施
│   │   ├── repository/       # 数据访问
│   │   └── service/          # 业务服务
│   ├── migrations/           # 数据库迁移
│   └── config.toml           # 配置文件
│
├── admin-web/                 # 管理后台（Vue 3）
│   ├── src/
│   │   ├── components/       # 组件
│   │   ├── service/api/      # API 封装
│   │   ├── store/            # 状态管理
│   │   ├── views/            # 页面
│   │   └── locales/          # 国际化
│   └── package.json
│
└── docs/                      # 文档
```

---

## 🛠️ 二次开发

### 新增后端模块

参考 [Server 架构设计](docs/server-architecture.md) 中的二次开发示例：

1. 定义实体（`domain/entity`）
2. 创建仓储（`repository`）
3. 创建 DTO（`interface/admin/dto`）
4. 创建服务（`service`）
5. 创建 Handler（`interface/admin/http/handler`）
6. 注册路由（`interface/admin/http/router`）
7. Wire 注入（`app/wire.go`）

### 新增前端页面

参考 [Admin-Web 架构设计](docs/admin-web-architecture.md) 中的二次开发示例：

1. 定义类型（`typings/api/v1`）
2. 封装 API（`service/api/v1`）
3. 创建页面（`views/xxx/index.vue`）
4. 添加国际化（`locales/langs`）

---

## 📖 适用场景

- **企业级后台管理系统**: 用户、内容、权限、配置管理
- **快速开发平台**: 新项目快速启动基座，减少重复开发
- **多客户端支持**: BFF 架构支持 Admin、移动端、Web 端
- **精细权限控制**: RBAC 体系满足企业精细化控制需求
- **国际化项目**: 前后端均支持国际化
- **微服务基础服务**: 认证授权、配置管理、内容管理

---

## 🤝 参与贡献

欢迎提交 Issue 和 Pull Request。

---

## 📄 开源协议

本项目基于 MIT 协议开源。

---

**注意**: NetyAdmin 基于 soybean-admin 进行重构和精简。感谢 soybean-admin 团队的贡献。
