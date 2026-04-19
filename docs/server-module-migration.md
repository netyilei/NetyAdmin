# 数据迁移模块详解

本文档详细介绍 NetyAdmin 数据迁移模块的架构设计、目录结构和最佳实践。

---

## 一、模块概述

数据迁移模块是 NetyAdmin 的核心基础设施之一，负责在系统启动阶段自动执行数据库初始化与结构更新。它确保了不同环境（开发、测试、生产）下的数据库一致性。

### 1.1 核心特性

- **启动即同步**：在后端服务 Bootstrap 阶段自动触发，无需手动维护 SQL 脚本执行顺序。
- **目录化管理**：通过物理文件夹区分脚本类型，结构清晰。
- **阶段化执行**：严格按照 `结构 -> 约束 -> 数据 -> 其他` 的顺序执行，完美解决外键依赖问题。
- **幂等性保证**：采用“全量扫描+幂等执行”策略，支持脚本重复执行而不报错。
- **智能识别**：支持通过文件夹名称或文件前缀两种方式自动识别脚本类型。

---

## 二、目录结构

迁移脚本存放在 [server/migrations/](file:///d:/NetyAdmin/server/migrations) 目录下，按以下子目录组织，并使用 **3 位数字前缀** 严格控制执行顺序：

```text
server/migrations/
├── table/                  # 1. 表结构定义阶段 (001-999)
│   ├── 001_sys_dict_type.sql
│   ├── 011_admin_user.sql
│   └── 031_users.sql
├── constraint/             # 2. 约束与关联阶段 (001-999)
│   ├── 001_storage.sql
│   └── 002_open_platform.sql
├── data/                   # 3. 基础数据填充阶段 (001-999)
│   ├── 001_sys_dict_type.sql
│   ├── 021_admin_menu.sql
│   └── 901_admin_auth.sql
└── (root)                  # 4. 其他/兜底阶段
    └── custom_patch.sql
```

---

## 三、执行顺序逻辑

迁移引擎 [migrator.go](file:///d:/NetyAdmin/server/internal/pkg/migration/migrator.go) 会递归扫描目录并按以下顺序排序执行：

1.  **Table 阶段**：执行 `table/` 目录或以 `table_` 开头的文件。
2.  **Constraint 阶段**：执行 `constraint/` 目录或以 `constraint_` / `fk_` 开头的文件。
3.  **Data 阶段**：执行 `data/` 目录或以 `data_` 开头的文件。
4.  **Other 阶段**：执行补丁、兼容性补丁和其他所有 `.sql` 文件。

> **核心规则**：在同一阶段内部，文件按**文件名的数字前缀**顺序执行。因此所有脚本必须以 `NNN_` 格式开头。

---

## 四、脚本编写规范

为了确保迁移模块正常工作，开发者必须遵循以下规范：

### 4.1 命名规范

所有脚本文件必须遵循以下命名格式：
`数字前缀_描述.sql`

- **数字前缀**：3 位数字（如 `001`, `010`, `100`），决定了同一目录下的执行先后。
- **描述**：简短的英文描述（如 `sys_user`, `add_column_age`）。

**推荐的数字分配**：
- `001-099`: 核心基础模块
- `100-499`: 业务模块
- `900-999`: 权限分配、数据同步、清理脚本

脚本必须支持多次执行。
- **创建表**：使用 `CREATE TABLE IF NOT EXISTS`。
- **插入数据**：使用 `INSERT INTO ... ON CONFLICT DO NOTHING` 或 `ON CONFLICT (...) DO UPDATE`。
- **修改字段**：使用 `DO` 块检查列是否存在。

### 4.2 约束分离

**严禁**在 `table/` 脚本中使用内联外键（Inline Foreign Keys）。所有的外键关联必须写在 `constraint/` 目录下，使用 `ALTER TABLE` 语句添加。这样可以避免“循环依赖”导致的表创建失败。

### 4.3 示例：添加外键

```sql
-- server/migrations/constraint/example.sql
BEGIN;
DO $$ 
BEGIN 
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.table_constraints 
        WHERE constraint_name = 'fk_user_profile_user'
    ) THEN
        ALTER TABLE user_profiles 
        ADD CONSTRAINT fk_user_profile_user 
        FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
    END IF;
END $$;
COMMIT;
```

---

## 五、系统集成

### 5.1 初始化执行

在 [wire.go](file:///d:/NetyAdmin/server/internal/app/wire.go) 的 `Bootstrap` 函数中，系统会自动调用迁移引擎：

```go
// Run 执行迁移
if err := migrator.Run(); err != nil {
    return fmt.Errorf("database migration failed: %w", err)
}
```

### 5.2 事务保证

迁移引擎会将所有待执行的 SQL 文件包裹在一个大的数据库事务中。如果其中任何一个脚本执行失败，整个迁移过程将回滚，确保数据库状态的一致性。

---

## 六、最佳实践

1.  **一表一文件**：为每个表创建独立的 `table/xxx.sql`，便于管理和追踪变更。
2.  **数据与结构分离**：不要在建表脚本里写 `INSERT` 语句。
3.  **事务控制**：建议在 SQL 脚本内部也使用 `BEGIN;` 和 `COMMIT;` 包裹（虽然引擎已有外层事务）。
4.  **日志观察**：服务启动时，通过控制台日志确认每个迁移脚本的执行状态。

---

## 七、相关参考

- [Server 架构设计](./server-architecture.md)
- [API 管理指南](./api-management.md)
