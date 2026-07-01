# 数据迁移模块详解

本文档详细介绍 NetyAdmin 数据迁移模块的架构设计、目录结构和最佳实践。

---

## 一、模块概述

数据迁移模块是 NetyAdmin 的核心基础设施之一，负责在系统启动阶段自动执行数据库初始化与结构更新，确保不同环境（开发、测试、生产）下的数据库一致性。

### 1.1 核心特性

- **基于 golang-migrate v4.18.2**：采用社区主流迁移框架，自带版本追踪、脏锁检测、增量迁移能力。
- **编译期嵌入**：迁移 SQL 通过 `go:embed` 编译进二进制，部署时无需携带外部迁移文件。
- **扁平化版本管理**：采用 golang-migrate 标准的 `NNNN_name.up.sql` / `NNNN_name.down.sql` 格式，全局唯一递增版本号。
- **历史压扁**：将历史增量变更（ALTER、补充数据）合并为"建表即最终形态、种子即最终数据"，避免冗余的回放历史。
- **幂等安全**：全量脚本保留 `IF NOT EXISTS` / `ON CONFLICT DO NOTHING`，重复执行不报错。

---

## 二、目录结构

迁移脚本存放在 [server/internal/pkg/migration/migrations/](file:///d:/NetyAdmin/server/internal/pkg/migration/migrations) 目录下（与迁移器代码同包，便于 `go:embed`），采用扁平化的 4 位全局序号：

```text
server/internal/pkg/migration/
├── migration.go                 # 迁移器代码（基于 golang-migrate）
└── migrations/                  # 迁移 SQL 文件（go:embed 编译进二进制）
    ├── 0001_sys_dict_type.up.sql
    ├── 0001_sys_dict_type.down.sql
    ├── 0002_sys_dict_data.up.sql
    ├── 0002_sys_dict_data.down.sql
    ├── ...
    ├── 0036_upload_record.up.sql       # 含历史 ALTER 合并后的最终表结构
    ├── 0037_fk_storage.up.sql          # 外键约束阶段
    ├── ...
    ├── 0040_seed_sys_dict_type.up.sql   # 种子数据阶段（无 down 文件）
    ├── ...
    └── 0057_seed_admin_auth.up.sql      # 必须最后执行（依赖前置 menu/api/button）

server/scripts/
└── sequence_sync.sql            # 运维工具：dump 导入后修复主键序列（不计入迁移版本）
```

### 文件编号规则

| 序号范围 | 阶段 | 说明 |
|---------|------|------|
| 0001-0036 | 表结构 | `CREATE TABLE`，每个表一个文件 |
| 0037-0039 | 外键约束 | `ALTER TABLE ADD CONSTRAINT`，跨表依赖独立成文件 |
| 0040-0057 | 种子数据 | `INSERT`，无 down 文件（清库重建即恢复） |

> **关键约束**：`0057_seed_admin_auth` 必须排在所有种子数据最后，因为它依赖前置的 menu/api/button 全部就绪后做超级管理员的全量授权。

---

## 三、执行顺序逻辑

迁移器 [migration.go](file:///d:/NetyAdmin/server/internal/pkg/migration/migration.go) 的工作原理：

1. **嵌入加载**：`go:embed migrations/*.sql` 将所有 SQL 编译进二进制。
2. **iofs source**：用 `iofs.New(embedFS, "migrations")` 创建内存文件源。
3. **版本追踪**：golang-migrate 在数据库中创建 `schema_migrations` 表，记录当前版本号。
4. **增量迁移**：`m.Up()` 自动比对数据库当前版本与文件最新版本，仅执行缺失的迁移。
5. **字典序执行**：文件按文件名字典序执行，4 位数字零填充确保数值序 = 字典序。

### ErrNoChange 处理

当数据库已是最新版本时，`m.Up()` 返回 `migrate.ErrNoChange`，迁移器将其视为成功（无变更需要执行）。

---

## 四、脚本编写规范

### 4.1 命名规范

新增迁移文件必须遵循 golang-migrate 标准：

```
NNNN_descriptive_name.up.sql    # 正向迁移
NNNN_descriptive_name.down.sql  # 回滚迁移（表/约束必须提供，种子数据可不提供）
```

- **NNNN**：4 位全局唯一递增数字，紧接当前最大序号（当前最大为 0057，下一个用 0001 不对，用 0058）。
- **descriptive_name**：简短英文描述，蛇形命名。

推荐用 golang-migrate CLI 生成（自动分配序号）：

```bash
migrate create -ext sql -dir internal/pkg/migration/migrations -seq add_user_avatar_column
```

### 4.2 事务管理

**不要**在 SQL 文件内写 `BEGIN;` / `COMMIT;`。golang-migrate 默认每个文件作为一个事务执行，内部事务包裹会导致嵌套事务警告。

`DO $$ ... $$` 的 PL/pgSQL 块内部的 `BEGIN/END` 是语法必需，保留不动。

### 4.3 up 文件规范

```sql
-- 0058_add_user_avatar.up.sql
ALTER TABLE users ADD COLUMN IF NOT EXISTS avatar VARCHAR(500);
CREATE INDEX IF NOT EXISTS idx_users_avatar ON users(avatar) WHERE avatar IS NOT NULL;
```

### 4.4 down 文件规范

down 文件用于回滚，必须可安全执行：

```sql
-- 0058_add_user_avatar.down.sql
ALTER TABLE users DROP COLUMN IF EXISTS avatar;
```

> **生产环境警告**：down 文件会删除数据/结构，生产环境慎用。主要用于开发环境重置。

### 4.5 种子数据规范

种子数据文件（0040-0057）**不需要 down 文件**。原因：清空数据库后 `m.Up()` 会重新建表并插入种子数据，无需回滚。如需修改种子数据，直接修改对应的 up 文件（用 `ON CONFLICT DO UPDATE` 保证幂等）。

---

## 五、系统集成

### 5.1 配置

```toml
# config.toml
[migration]
enabled = true
# 迁移文件已通过 go:embed 编译进二进制，无需配置 dir。
```

### 5.2 启动执行

在 [wire.go](file:///d:/NetyAdmin/server/internal/app/wire.go) 的 `Bootstrap` 函数中：

```go
if cfg.Migration.Enabled {
    if err := migration.Run(cfg.Database.DSN()); err != nil {
        return nil, fmt.Errorf("数据库迁移失败: %w", err)
    }
}
```

迁移器使用**独立的数据库连接**（不复用 GORM 连接池），以避免 golang-migrate 的 advisory lock 与业务查询相互阻塞。

---

## 六、生产库基线建立（重要）

### 全新部署（空数据库）

直接启动服务，`m.Up()` 从版本 0001 顺序执行全部 57 个迁移，自动建表 + 种子数据。

### 已有数据库升级到 golang-migrate

现有生产库已有全部表和数据，但无 `schema_migrations` 表。**必须在部署新版二进制前**，在生产库手动建立版本基线：

```sql
-- 告诉 golang-migrate 当前库已是版本 57 的状态
CREATE TABLE IF NOT EXISTS schema_migrations (version bigint PRIMARY KEY, dirty boolean NOT NULL DEFAULT false);
INSERT INTO schema_migrations (version, dirty) VALUES (57, false);
```

建立基线后，新版二进制启动时 `m.Up()` 检测到已是最新版本，跳过所有迁移，**零数据变更**。

> **不丢数据保证**：up 文件全部用 `IF NOT EXISTS` / `ON CONFLICT DO NOTHING`；基线 force 到 57 后 Up 不会执行任何 DDL。

---

## 七、相关参考

- [Server 架构设计](./server-architecture.md)
- [API 管理指南](./api-management.md)
- golang-migrate 官方文档：https://github.com/golang-migrate/migrate
