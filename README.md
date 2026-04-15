[English](README.en-US.md)

# NetyAdmin - 通用型后台管理系统基座

NetyAdmin 是一个基于 Go 后端和 Vue 3 前端构建的通用型后台管理系统基座。它旨在为企业级应用提供一套快速开发、高性能、高可用且功能丰富的管理后台解决方案。项目结合了最新的前端技术栈和稳健的后端架构，致力于成为您构建各类管理系统的坚实基础。

## 核心特性

### 🚀 现代化技术栈

* **前端**: Vue 3, TypeScript, Vite, Naive UI, UnoCSS, Pinia, Vue Router, vue-i18n
* **后端**: Go, Gin, GORM (PostgreSQL), Redis (可选), JWT

### 💡 清晰的架构设计

* **前端**: 采用 pnpm monorepo 架构，目录结构清晰，遵循严格的页面层架构，组件高内聚，API 版本隔离，易于维护和扩展。
* **后端**: 采用 BFF (Backend For Frontend) 模式，实现多端物理隔离，确保业务逻辑和权限安全。遵循清晰的分层架构 (`router -> handler -> service -> repository -> entity`)，模块以业务域拆分。

### 🔒 完善的认证与权限管理

* **JWT 认证**: 提供安全可靠的用户认证机制。
* **RBAC 权限体系**: 基于角色、菜单、按钮和 API 的精细化权限控制，支持管理员、角色、菜单、API、按钮的 CRUD 操作及授权管理。
* **动态路由**: 后端动态生成路由树，前端根据用户权限渲染菜单和控制页面访问。

### ⚡ 高性能与高可用

* **Go 语言后端**: 提供高性能的 Web 服务能力。
* **透明缓存**: 支持 Redis 和本地内存 (BigCache) 双层缓存，通过 `LazyCacheManager` 统一管理，支持 Key 规范、Prefix 和 Tags 批量失效，有效提升数据访问性能。
* **配置热同步**: 系统配置支持 Upsert 更新，并通过 Redis Pub/Sub 实现全网热同步，支持缓存和任务系统的动态开关。
* **数据库自动迁移**: 启动期自动执行 `migrations/` 下的 SQL 脚本，支持在 `config.toml` 中动态开启/关闭。

### ✨ 丰富的功能模块

* **用户管理**: 管理员、角色、权限等。
* **内容管理**: 分类、文章、Banner 的 CRUD，支持文章发布、置顶、定时发布等。
* **系统配置**: 动态字典、系统参数配置与热同步、任务系统。
* **验证码模块**: 集成 `base64Captcha`，支持 Admin 端登录验证、多种验证码类型（数字/字符/算术）、自定义图形参数及缓存/数据库双轨存储。
* **存储管理**: 对象存储配置、上传凭证下发、上传记录管理。
* **日志审计**: 操作日志、错误日志的记录、查询和管理，支持敏感字段脱敏。

## 适用场景

NetyAdmin 作为一个功能全面、架构先进的后台管理系统基座，特别适用于以下场景：

* **企业级后台管理系统**: 适用于各种需要管理用户、内容、权限、系统配置等业务的后台管理系统。
* **快速开发平台**: 可作为新项目的快速启动基座，帮助团队高效构建企业级应用，减少重复开发工作。
* **多客户端支持的业务**: 后端 BFF 架构为未来支持多种前端（如 Admin、移动端、Web 端）提供了良好的扩展基础。
* **需要精细权限控制的系统**: 完善的 RBAC 权限体系能够满足企业对用户操作进行精细化控制的需求。
* **需要国际化支持的项目**: 前后端均支持国际化，方便部署到全球不同语言环境。
* **微服务架构中的基础服务**: 可作为微服务架构中的认证授权、配置管理、内容管理等基础服务。

## 快速开始

* **无 User 版本（不包含 user 模块）**:
  * 维护分支: <https://github.com/netyilei/NetyAdmin/tree/maint-nouser>
* **默认账号**: `admin`
* **默认密码**: `admin123`

（此处将放置部署和开发环境搭建的简要说明，详细内容请参考部署文档）

## 文档

* [状态码全量文档（编码规则 + 全量码表 + 新增流程）](docs/status_codes.md)
* [Admin-Web 目录结构与架构规范](docs/admin_web_directory.md)
* [Admin-Web 页面模块与路由侧行为](docs/admin_web_modules.md)
* [Admin-Web（Vue）现状总结](docs/admin_web_summary.md)
* [Server（Go）HTTP 路由清单](docs/server_api.md)
* [项目变更与修复记录 (CHANGELOG)](CHANGELOG.md)
* [Server 多端接入架构 (BFF Pattern) 规范](docs/server_arch_bff.md)
* [Server 基座：缓存与配置热同步](docs/server_cache_configsync.md)
* [Server（Go）目录结构与分层](docs/server_directory.md)
* [Server（Go）现状总结](docs/server_summary.md)

---

**注意**: `NetyAdmin` 基于 `soybean-admin` 进行重构和精简。感谢 `soybean-admin` 团队的贡献。
