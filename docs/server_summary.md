# Server（Go）现状总结

## 技术栈与关键依赖

- Web 框架：Gin
- ORM：GORM（PostgreSQL Driver）
- 配置：TOML（`server/config.toml`），使用 `go-toml/v2` 加载
- Redis：可开关（开：Redis + 本地内存双层缓存；关：降级为本地内存）
- JWT：`github.com/golang-jwt/jwt/v5`
- 存储：对象存储 S3 驱动（AWS SDK v2），支持上传凭证下发与上传记录
- 任务：内置 Task Manager（once / interval / cron 形态），支持后台管理与日志落库
- 基础设施说明：
  - 缓存与配置热同步：`server_cache_configsync.md`


## 服务启动入口

- 入口：`server/cmd/server/main.go`
- 配置文件：默认读取 `server/config.toml`
- 监听端口：由 `config.toml` 的 `[server].port` 决定（当前默认 8010）

## 当前已实现的业务模块（按后端代码）

### 1) 管理员认证（JWT）

- 登录、获取用户信息、个人资料查看/更新、修改密码
- 登录结果由后端返回 Token（以及用户信息对象，具体字段以实现为准）

### 2) RBAC（角色-菜单-按钮-API）

- 管理员管理（Admin CRUD）
- 角色管理（Role CRUD）
- 菜单管理（Menu CRUD + 菜单树）
- API 管理（API CRUD + API 树）
- 按钮管理（Button CRUD + Button 树）
- 角色授权：
  - 角色-菜单绑定
  - 角色-按钮绑定
  - 角色-API 绑定（用于权限中间件判定）
- 权限中间件：
  - 所有 `/admin/v1` 路由默认要求 JWT
  - 大多数管理接口要求 RBAC（少量 L1 接口仅 JWT）
  - 超级角色码存在“全放行”语义（具体常量以代码为准）

### 3) 动态路由（给前端渲染菜单）

- 基于后端菜单数据输出“用户可用路由树”
- 提供“路由是否存在”校验接口（用于前端路由守卫）

### 4) 系统配置（sys_configs）与热同步

- sys_configs：支持按组查询、Upsert 更新
- 配置变更支持全网热同步：
  - Redis Pub/Sub 广播
  - 节点本地缓存失效/重载
- 该能力与缓存系统/权限系统存在联动（例如开关缓存、控制 task 行为等）

### 5) 动态字典（sys_dict_type / sys_dict_data）

- 字典类型 CRUD
- 字典数据 CRUD
- 支持按 code 获取启用的字典数据（用于前端渲染 Tag/Select 等）
- 具备缓存策略设计（与 LazyCacheManager 与配置同步机制配合）

### 6) 内容管理（CMS）

- 分类（Category）：列表、树、CRUD
- 文章（Article）：列表、CRUD、发布/取消发布、置顶
- Banner 分组（BannerGroup）：列表、CRUD
- Banner 项（BannerItem）：列表、CRUD
- 存在文章定时发布相关 Job（具体策略以 job 实现为准）

### 7) 存储管理（对象存储）

- 存储配置（Storage Config）：列表/详情/创建/更新/删除、设为默认、测试上传
- 上传凭证：为前端上传提供临时凭证（通常用于直传对象存储）
- 上传记录（Upload Record）：创建、列表/详情、删除、批量删除

### 8) 日志体系（面向后台可观测与审计）

- 操作日志（Operation Log）：列表、删除、批量删除
- 错误日志（Error Log）：列表、标记已解决、删除、批量删除
- 中间件会对请求体中的敏感字段做脱敏（password 等）

### 10) 迁移体系（migrations）

- 自动执行：启动期（Bootstrap 阶段）自动扫描 `server/migrations/` 的 SQL 文件并同步执行。
- 可控性：迁移功能受 `config.toml` 中的 `[migration].enabled` 开关控制。
- 引导依赖：作为服务启动的硬性前置条件，确保数据库表结构和基础数据在业务逻辑运行前就绪。


## 当前 HTTP 路由前缀与命名空间

- 管理后台 API：`/admin/v1/*`
- 现有代码未注册 `/api/*` 前缀的业务接口（与 `docs/openapi.yaml` 中的 `/api` 体系不一致，详见 gap 分析）
