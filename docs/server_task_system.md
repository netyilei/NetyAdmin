# Server 基座：任务系统（Task Manager）

本文件描述当前 `server/` 已落地的任务系统：启动期任务、定时任务、后台可控化（启停/重载/日志）以及配置覆盖规则。

## 1) 任务引擎概览

任务引擎位于 `internal/pkg/task`，由 `task.Manager` 统一调度。

支持三种类型：

- `once`：启动执行一次
- `interval`：固定间隔循环
- `cron`：Cron 表达式

任务接口：

- `Task`：`Name()` / `DisplayName()` / `Run(ctx)`
- `TaskWithMetadata`：允许任务提供默认元数据（type/spec/weight/enabled）

实现见：

- [task.go]../../server/internal/pkg/task/task.go)
- [manager.go](../../server/internal/pkg/task/manager.go)

## 2) 配置来源与覆盖规则（两层覆盖）

### 2.1 第一层：代码默认值（任务自带 DefaultMetadata）

系统任务（例如 DB migration）通过实现 `DefaultMetadata()` 给出默认值，并按 weight 决定启动期的执行顺序。

示例见：[db_migration.go](../../server/internal/job/db_migration.go)

### 2.2 第二层：config.toml 覆盖（task.jobs）

`server/config.toml` 支持对任务进行覆盖（只覆盖显式写出的字段）：

- `task.enabled`：任务系统总开关
- `task.jobs.{taskName}.enabled/type/spec/weight`：单任务覆盖项

配置结构见：[config.go](../../server/internal/config/config.go#L10-L33)

合并规则见：[manager.go](../../server/internal/pkg/task/manager.go#L338-L374)

### 2.3 运行态覆盖：sys_configs（task_config 分组）

任务管理后台的“更新任务配置”并不会写回 `config.toml`，而是把覆盖项写入数据库 `sys_configs`：

- group：`task_config`
- key：
  - `task:{name}:enabled`
  - `task:{name}:spec`

这些覆盖值会在任务列表接口中生效，并在更新后触发 `ForceReload()` + 重启任务。

实现见：[task service](../../server/internal/service/system/task.go#L186-L217)

## 3) 启动执行链路

应用启动时：

- `taskManager.Register(...)` 注册所有任务（包含系统级与业务级）
- `taskManager.Start(...)` 在启动期执行 `once` 任务，并启动 cron/interval 的调度循环

注册点见：[wire.go](../../server/internal/app/wire.go#L120-L170)

## 4) 内置任务（当前已落地）

当前注册的任务集合在 `internal/job` 聚合：

- `db_migration`：数据库 SQL 迁移任务（once，系统级）
- `article_publish`：文章定时发布（interval/cron，业务级）
- `system_log_cleanup`：日志清理任务（interval/cron，运维级）

聚合入口见：[job/init.go](../../server/internal/job/init.go)

## 5) 管理后台 API（RBAC）

任务管理 API 位于 `/admin/v1/system/tasks*`（需要登录 + RBAC）：

- `GET /admin/v1/system/tasks`：任务列表（包含 enabled/spec 的 DB 覆盖、生效状态、最近执行信息）
- `POST /admin/v1/system/tasks/:name/run`：立即执行一次
- `POST /admin/v1/system/tasks/:name/start`：启动任务
- `POST /admin/v1/system/tasks/:name/stop`：停止任务
- `POST /admin/v1/system/tasks/:name/reload`：重载任务（停止后按当前配置决定是否启动）
- `PUT /admin/v1/system/tasks/:name`：更新任务（写入 sys_configs(task_config) 并立即生效）
- `GET /admin/v1/system/tasks/logs`：查询任务执行日志

实现见：[task_handler.go](../../server/internal/handler/v1/system/task_handler.go)

## 6) 任务日志（sys_task_logs）

### 6.1 持久化方式

任务执行完成后，会通过 `task.Manager.SetOnFinish()` 回调写入 `sys_task_logs`。

- 可通过 sys_configs 动态关闭日志落库：
  - group=`task_config`
  - key=`log_enabled`
  - value=`false/0`：不落库

实现见：[task service](../../server/internal/service/system/task.go#L29-L60)

### 6.2 清理策略

`system_log_cleanup` 任务会读取以下配置来决定保留天数：

- `task_config.retention_days`：任务日志保留天数
- `ops_config.retention_days`：操作日志保留天数
- `error_config.retention_days`：错误日志保留天数

实现见：[system_log_cleanup.go](../../server/internal/job/system_log_cleanup.go)

