# 项目变更与修复记录 (CHANGELOG)

本文档用于统一记录 NetyAdmin 项目的功能变更、架构重构以及 Bug 修复内容。

## [2026-04-10] 项目全量更名与架构优化

### 🔄 功能变更与重构

- **角色管理优化**：移除了编辑角色菜单权限弹窗中的“首页”下拉选择框，简化了权限分配流程。
- **项目彻底更名**：
  - 全量替换项目标识：`NetyAdmin` -> `NetyAdmin`。
  - 前端包名变更：`NetyAdmin-admin` -> `netyadmin-web`。
  - 后端模块名变更：`NetyAdmin` -> `netyadmin`。
  - 清理了所有文档、配置、代码注释中的旧项目名称。
- **数据库迁移机制重构**：
  - **解耦任务模块**：将 `db_migration` 从异步任务模块中移除，不再作为后台 Job 运行。
  - **同步启动机制**：重构为 `Bootstrap` 引导阶段的同步阻塞步骤，确保业务服务初始化前数据库表结构已就绪。
  - **独立开关控制**：在 `config.toml` 中新增 `[migration].enabled` 配置项。
- **文档规范化**：
  - 更新了 `README.md` 中的核心特性描述。
  - 修正了项目归属说明，明确基于 `NetyAdminAdmin` 进行重构。

### 🐞 Bug 修复

- **数据库约束匹配修复**：
  - 修复了 `admin_api` 和 `admin_button` 在执行 `INSERT ... ON CONFLICT` 时因缺少 `WHERE deleted_at = 0` 导致无法匹配部分唯一索引的错误。
- **前端环境损坏修复**：
  - 修正了误替换 `package.json` 中外部依赖组织名（`@NetyAdminjs/`）导致 `pnpm install` 失败的问题。
  - 修复了因 `node_modules` 损坏导致的 `vite` 命令找不到（`MODULE_NOT_FOUND`）的报错。
- **国际化映射修复**：
  - 修正了菜单管理中“目录”类型翻译 Key 不一致的问题（`directory` -> `dir`），使其与后端字典定义匹配。
  - 修复了内容管理模块（内容分类、文章管理、Banner组）菜单缺失 `i18nKey` 导致标签页标题显示不正确的问题。
- **角色管理逻辑修复**：
  - 修复了更新角色权限时 `homeRouteName` 被硬编码为空字符串的问题，现在会保留从后端获取的原始值，防止角色配置丢失。
- **初始化顺序修复**：
  - 解决了 `ConfigWatcher` 和 `StorageService` 在表结构创建前尝试查询数据库导致的 `relation "xxx" does not exist` 崩溃问题。

### 🔧 资源调整

- **日志输出优化**：在服务启动阶段禁用了 Gin 框架默认打印的路由映射调试日志（`[GIN-debug] GET ...`），使启动控制台更加清爽。
- **文件重命名**：
  - 前端组件：`NetyAdmin-avatar.vue` -> `netyadmin-avatar.vue`。
  - 静态资源：`NetyAdmin.jpg` -> `netyadmin.jpg`。
- **清理残留**：
  - 物理删除了 `internal/job/db_migration.go`。
  - 清理了 `data_storage.sql` 中旧版残留的无效 API 定义。

