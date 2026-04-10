# Server 基座：缓存与配置热同步（Redis / 本地降级）

本文件描述当前 `server/` 已落地的两套基础设施能力：

- **透明缓存**：对 RBAC、字典、菜单树等读多写少的数据做缓存加速
- **配置中心（sys_configs）**：支持配置 Upsert、全网热同步、并作为“开关/参数”驱动缓存与任务系统

## 1) 运行模式与开关来源

### 1.1 Redis 启用/禁用

由 `server/config.toml` 决定：

- `[redis].enabled = true`：启用 Redis（缓存用 Redis、配置热同步用 Redis Pub/Sub）
- `[redis].enabled = false`：禁用 Redis（缓存降级为本地内存 BigCache，配置热同步不启用）

配置结构见：[config.go](../../server/internal/config/config.go)

### 1.2 动态开关来源：sys_configs

系统配置表为 `sys_configs`（代码中叫 `SysConfig`），通过管理后台 API 进行 Upsert。

- 配置缓存/任务等“动态开关”统一存放在分组（group）里
- `ConfigWatcher` 会把 DB 中所有配置加载到内存 map，形成 `group:key -> value` 的快速查表

实现见：

- [watcher.go](file:///d:/SilentOrder/server/internal/pkg/configsync/watcher.go)
- [config service](../../server/internal/service/system/config.go)

## 2) 透明缓存（LazyCacheManager）

### 2.1 缓存引擎

缓存由 `LazyCacheManager` 统一封装：

- Redis 启用：缓存后端为 Redis
- Redis 禁用：缓存后端降级为 BigCache（本地内存）

实现见：[manager.go](../../server/internal/pkg/cache/manager.go)

### 2.2 Key 规范与 prefix

缓存 key 不允许业务代码手写字符串，必须由 `internal/pkg/cache/registry.go` 提供 Key 工厂函数。

- `RedisConfig.prefix` 会被自动注入到所有缓存 key 前（例如 `so:admin:1:info`）
- key 工厂函数输出的是“不含 prefix 的逻辑 key”

实现见：[registry.go](../../server/internal/pkg/cache/registry.go)

### 2.3 Tags 与批量失效

缓存写入时支持 tags（标签），用于业务数据更新后一键失效相关缓存。

当前已定义的 tags / ttl 口径见：[registry.go](file:///d:/SilentOrder/server/internal/pkg/cache/registry.go)

常见标签：

- `rbac:*`：RBAC 相关（menu/role/api）
- `dict:*`：字典相关
- `sys:config`：系统配置相关

### 2.4 模块级缓存开关（cache_switches）

`LazyCacheManager.Fetch()` 会根据 `moduleName` 做动态开关判断：

- 从 `sys_configs` 中读取 group=`cache_switches`，key=`{moduleName}` 的值
- value 为 `true/1`：允许缓存
- value 为 `false/0`：缓存直接穿透回源（不读缓存、不写缓存）

实现见：

- [watcher.go](../../server/internal/pkg/configsync/watcher.go#L103-L121)
- [manager.go](../../server/internal/pkg/cache/manager.go#L55-L106)

## 3) 配置热同步（ConfigWatcher）

### 3.1 内存结构

Watcher 内存结构为：

- `memory["{group}:{key}"] = "{value}"`

获取方式：

- `GetConfig(group, key)`
- `GetGroupConfigs(group)`

实现见：[watcher.go](../../server/internal/pkg/configsync/watcher.go)

### 3.2 触发更新与广播

当后台通过 Config API Upsert 成功后：

- 当前节点：会立即 `ForceReload()`（确保当前节点立刻生效）
- 若 Redis 启用：通过 Redis Pub/Sub 发布 `config_updated` 信号，集群其他节点收到后执行 `ForceReload()`

实现见：[config service](../../server/internal/service/system/config.go#L58-L87)

### 3.3 Redis Channel 命名规则

配置同步使用统一 channel（带 prefix 做环境隔离）：

- `ChannelConfigSync(prefix)`

实现见：[redis package](../../server/internal/pkg/redis)

