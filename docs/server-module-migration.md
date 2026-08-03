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
    ├── 0035_upload_record.up.sql       # 含历史 ALTER/索引/外键合并后的最终表结构
    ├── ...
    ├── 0036_seed_sys_dict_type.up.sql   # 种子数据阶段（含 down 文件，见 §4.5）
    ├── 0036_seed_sys_dict_type.down.sql
    ├── ...
    ├── 0053_seed_admin_auth.up.sql      # 必须最后执行（依赖前置 menu/api/button）
    ├── 0053_seed_admin_auth.down.sql
    └── 0055_user_tokens.up.sql  # 客户端多端会话表（platform 维度，顶号）

server/scripts/
└── sequence_sync.sql            # 运维工具：dump 导入后修复主键序列（独立脚本，不入迁移链，见 §2.3 编号规划）
```

### 文件编号规则

| 序号范围 | 阶段 | 说明 |
|---------|------|------|
| 0001-0035 | 表结构 | `CREATE TABLE`，每个表一个文件，含最终列/联合索引/外键（历史补丁已压扁） |
| 0036-0053 | 种子数据 | `INSERT`，必须配对 up/down 文件（见 §4.5） |
| 0055+ | 新功能 | 新功能建表等 |

> **关键约束**：`0053_seed_admin_auth` 必须排在所有种子数据最后，因为它依赖前置的 menu/api/button 全部就绪后做超级管理员的全量授权。

### 2.3 编号规划（基座 / 下游协作，重要）

迁移文件为**全局递增序号**，基座与基于基座开发的下游项目共用同一个 `migrations/` 目录，必须划分号段避免编号冲突：

| 号段 | 归属 | 说明 |
|------|------|------|
| `0001-0499` | 基座业务迁移 | 建表 / 种子 / 新功能。基座新增迁移继续使用段内下一个连续号（当前最新为 0055，下一个用 0056） |
| `1000-9999` | 下游项目迁移 | 基于基座开发的项目，自建迁移从 `1001` 起编号，**绝不使用** `0001-0499` 段 |
| 不入链 | 运维/兜底脚本 | 如 `server/scripts/sequence_sync.sql`，**不放入迁移链**（原因见下） |

> **为什么运维脚本不入迁移链**：golang-migrate 的 `Up()` 只执行编号**大于**当前库版本的迁移。若兜底脚本占用大号段（如 8001），任何已部署库执行过它后版本号即停在 8001，基座之后新增的业务迁移（0056+，编号更小）将**永不执行**。故运维/兜底一律放 `server/scripts/` 独立脚本，迁移链只保留业务迁移。

**下游项目新增迁移的正确做法**：在 `migrations/` 下创建 `1001_xxx.up.sql` / `1001_xxx.down.sql`（以此类推）。基座更新新增 `0056+` 迁移时，与下游 `1001+` 编号永不冲突，`git merge` 后直接可用。

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

- **NNNN**：4 位全局唯一递增数字。基座段 `0001-0499`（当前最新为 0055，下一个用 0056）；下游项目段 `1001-9999`（见 §2.3）。
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
-- 0065_add_user_avatar.up.sql
ALTER TABLE users ADD COLUMN IF NOT EXISTS avatar VARCHAR(500);
CREATE INDEX IF NOT EXISTS idx_users_avatar ON users(avatar) WHERE avatar IS NOT NULL;
```

### 4.4 down 文件规范

down 文件用于回滚，必须可安全执行：

```sql
-- 0065_add_user_avatar.down.sql
ALTER TABLE users DROP COLUMN IF EXISTS avatar;
```

> **生产环境警告**：down 文件会删除数据/结构，生产环境慎用。主要用于开发环境重置。

### 4.5 种子数据规范

种子数据文件（0036-0053）**必须配对 up/down 文件**。原因：

1. **CI 可逆性测试**：CI 流水线执行 `migrate up && migrate down 1 && migrate up` 验证迁移可逆性，缺失 down 文件会导致测试失败。
2. **开发环境回滚**：开发者误执行 up 后可通过 down 回滚到迁移前状态，避免清库重建。
3. **审计与文档**：down 文件即「up 操作的精确回滚清单」，便于审计种子数据范围。

#### 种子 down 文件编写范式

down 文件必须是 up 文件的**精确反向操作**，使用与 up 相同的 WHERE 条件删除被插入的行：

```sql
-- up：INSERT INTO sys_dict_type (name, code, ...) VALUES (..., 'sys_status', ...) ON CONFLICT (code) DO NOTHING;
-- down：DELETE FROM sys_dict_type WHERE code IN ('sys_status', ...);
```

| up 文件特征 | down 文件应做 |
|------------|--------------|
| `INSERT ... ON CONFLICT (code) DO NOTHING` | `DELETE WHERE code IN (...)`（按业务唯一键删除） |
| `INSERT ... ON CONFLICT (method, path) DO NOTHING` | `DELETE WHERE (method, path) IN (...)` |
| `INSERT ... ON CONFLICT (id) DO NOTHING` | `DELETE WHERE id IN (...)`（按主键删除） |
| `INSERT ... SELECT ... WHERE NOT EXISTS` | `DELETE WHERE`（按 up 文件的 SELECT 条件删除） |
| 动态查找关联表 ID 插入（如 `admin_role_menus`） | 用相同的子查询定位 ID 后 DELETE |

> **删除顺序**：多表 down 时按 up 的**逆序**删除（先删关联表，再删主表），避免外键约束冲突。

> **不要用 `TRUNCATE`**：down 文件必须只删除 up 文件插入的精确行，不能用 `TRUNCATE` 清表（会误删用户数据）。

### 4.6 不可逆迁移的 down 文件规范

部分迁移操作本质上是**幂等修复**而非数据/结构变更，无有意义的回退路径。这类迁移的 down 文件应明确标注「不可逆」并说明原因，文件内容可为 `SELECT 1;` 占位：

```sql
-- 0055_xxx_irreversible.down.sql
-- 不可逆：本迁移是幂等修复操作，对数据无破坏性影响，down 不需要回退
-- 说明：
--   1. 修复操作只对齐/修正数据，不删除数据、不修改表结构；
--   2. 回退没有意义——下一次同类修复仍会重新对齐；
--   3. 若强制回退，需要知道修复前的原值（不可知），无安全回退路径。
SELECT 1;
```

典型场景：
- 序列同步（setval）
- 统计信息刷新（ANALYZE）
- 索引重建（REINDEX）
- 数据修复脚本（UPDATE 修正脏数据）

> **关键约束**：down 文件必须存在（满足可逆性配对），但内容可以是 no-op。注释必须说明「为什么不可逆」与「为什么安全」。

---

## 五、系统集成

### 5.1 配置

```yaml
# config.yaml
migration:
  enabled: true
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

直接启动服务，`m.Up()` 从版本 0001 顺序执行全部迁移（当前最新版本为 0055），自动建表 + 种子数据。

### 已有数据库升级到 golang-migrate

现有生产库已有全部表和数据，但无 `schema_migrations` 表。**必须在部署新版二进制前**，在生产库手动建立版本基线：

```sql
-- 告诉 golang-migrate 当前库已是最新版本（当前为 0055）的状态
CREATE TABLE IF NOT EXISTS schema_migrations (version bigint PRIMARY KEY, dirty boolean NOT NULL DEFAULT false);
INSERT INTO schema_migrations (version, dirty) VALUES (55, false);
```

建立基线后，新版二进制启动时 `m.Up()` 检测到已是最新版本，跳过所有迁移，**零数据变更**。

> **不丢数据保证**：up 文件全部用 `IF NOT EXISTS` / `ON CONFLICT DO NOTHING`；基线 force 到 54 后 Up 不会执行任何 DDL。
>
> **基线版本同步**：每次新增迁移文件后，上述 INSERT 语句的版本号需相应更新为最新版本号。可用 `ls internal/pkg/migration/migrations/*.up.sql | tail -1` 查看最新版本。

### 历史补丁压扁后的已有库升级指引（重要）

迁移链曾将历史补丁（token_version / request_id 列、23 个联合索引、5 个外键、msg_blacklist 删表）压扁进建表文件，并将迁移链重编号为连续序号（当前 `0001-0055`）。因此：

- **全新库 / 按上节 force 基线到 0054 的库**：编号与文件一一对应，后续 `m.Up()` 正常增量。
- **任何已跟踪的旧编号库（`schema_migrations` 版本 ≠ 0055）**：新旧文件编号已错位，版本对照不可靠，**必须删库重建**（本项目开发期惯例）。
- **无 schema_migrations 表的既有库**：按上节「已有数据库升级到 golang-migrate」force 基线到 0054 即可（schema 需自行与最新建表文件对齐）。

---

## 七、相关参考

- [Server 架构设计](./server-architecture.md)
- [API 管理指南](./api-management.md)
- golang-migrate 官方文档：https://github.com/golang-migrate/migrate
