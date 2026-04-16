# 数据迁移模块详解

本文档详细介绍 NetyAdmin 数据迁移模块的架构设计、使用方式和二次开发指南。

---

## 一、模块概述

数据迁移模块提供数据库结构和基础数据的版本化管理，支持启动期自动执行迁移脚本。

### 1.1 核心特性

- **自动执行**：服务启动时自动检测并执行迁移
- **版本控制**：基于文件名的版本排序
- **幂等执行**：支持重复执行不报错
- **可控开关**：通过配置开启/关闭迁移
- **脚本分类**：表结构和基础数据分离

---

## 二、目录结构

```
server/migrations/
├── table_admin_system.sql      # 管理员系统表结构
├── table_captcha_base.sql      # 验证码基础表
├── table_content.sql           # 内容管理表结构
├── table_storage.sql           # 存储模块表结构
├── table_sys_configs.sql       # 系统配置表结构
├── table_sys_dict.sql          # 字典表结构
├── data_admin_system.sql       # 管理员系统基础数据
├── data_captcha_base.sql       # 验证码基础数据
├── data_content.sql            # 内容管理基础数据
├── data_storage.sql            # 存储模块基础数据
├── data_sys_configs.sql        # 系统配置基础数据
└── data_sys_dict.sql           # 字典基础数据

server/internal/pkg/migration/
└── migrator.go                 # 迁移执行器
```

---

## 三、架构设计

### 3.1 迁移执行器

```go
// Migrator 迁移执行器
type Migrator struct {
    db      *gorm.DB
    enabled bool
    source  string // 迁移脚本目录
}

// Migrate 执行所有迁移
func (m *Migrator) Migrate() error {
    if !m.enabled {
        log.Println("Migration is disabled")
        return nil
    }
    
    // 1. 创建迁移记录表
    if err := m.createMigrationTable(); err != nil {
        return err
    }
    
    // 2. 读取所有迁移文件
    files, err := m.readMigrationFiles()
    if err != nil {
        return err
    }
    
    // 3. 按版本排序
    sort.Slice(files, func(i, j int) bool {
        return files[i].Version < files[j].Version
    })
    
    // 4. 执行未执行的迁移
    for _, file := range files {
        executed, err := m.isExecuted(file.Version)
        if err != nil {
            return err
        }
        
        if executed {
            log.Printf("Migration %s already executed, skipping", file.Version)
            continue
        }
        
        if err := m.execute(file); err != nil {
            return fmt.Errorf("execute migration %s failed: %w", file.Version, err)
        }
        
        if err := m.record(file); err != nil {
            return fmt.Errorf("record migration %s failed: %w", file.Version, err)
        }
    }
    
    return nil
}
```

### 3.2 迁移记录表

```sql
CREATE TABLE IF NOT EXISTS schema_migrations (
    id SERIAL PRIMARY KEY,
    version VARCHAR(64) NOT NULL UNIQUE,  -- 迁移版本（文件名）
    executed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    execution_time INTEGER               -- 执行耗时（毫秒）
);
```

---

## 四、配置说明

### 4.1 配置文件（config.toml）

```toml
[migration]
# 是否启用自动迁移
enabled = true

# 迁移脚本目录（相对路径）
source = "./migrations"
```

### 4.2 环境变量覆盖

```bash
# 生产环境建议关闭自动迁移
export MIGRATION_ENABLED=false
```

---

## 五、迁移脚本规范

### 5.1 文件命名规则

```
{类型}_{模块}_{描述}.sql

类型：
- table: 表结构定义
- data: 基础数据
- alter: 结构变更
- index: 索引创建

示例：
- table_admin_system.sql      # 管理员系统表结构
- data_admin_system.sql       # 管理员系统基础数据
- alter_article_add_tags.sql  # 文章表新增tags字段
```

### 5.2 脚本内容规范

```sql
-- table_admin_system.sql

-- 管理员表
CREATE TABLE IF NOT EXISTS sys_admins (
    id SERIAL PRIMARY KEY,
    username VARCHAR(64) NOT NULL UNIQUE,
    password VARCHAR(256) NOT NULL,
    nickname VARCHAR(128),
    avatar VARCHAR(512),
    email VARCHAR(128),
    phone VARCHAR(32),
    status SMALLINT DEFAULT 1,
    last_login_at BIGINT,
    created_at BIGINT DEFAULT EXTRACT(EPOCH FROM NOW()),
    updated_at BIGINT DEFAULT EXTRACT(EPOCH FROM NOW()),
    deleted_at BIGINT
);

-- 角色表
CREATE TABLE IF NOT EXISTS sys_roles (
    id SERIAL PRIMARY KEY,
    code VARCHAR(64) NOT NULL UNIQUE,
    name VARCHAR(128) NOT NULL,
    description VARCHAR(256),
    status SMALLINT DEFAULT 1,
    created_at BIGINT DEFAULT EXTRACT(EPOCH FROM NOW()),
    updated_at BIGINT DEFAULT EXTRACT(EPOCH FROM NOW()),
    deleted_at BIGINT
);

-- 索引
CREATE INDEX IF NOT EXISTS idx_sys_admins_status ON sys_admins(status);
CREATE INDEX IF NOT EXISTS idx_sys_roles_status ON sys_roles(status);
```

### 5.3 幂等性保证

```sql
-- 使用IF NOT EXISTS确保幂等

-- 创建表
CREATE TABLE IF NOT EXISTS table_name (...);

-- 添加列
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'articles' AND column_name = 'tags'
    ) THEN
        ALTER TABLE articles ADD COLUMN tags VARCHAR(512);
    END IF;
END $$;

-- 创建索引
CREATE INDEX IF NOT EXISTS idx_name ON table_name(column);
```

---

## 六、使用示例

### 6.1 启动期自动执行

```go
// internal/app/init.go

func InitDB(config *config.Config) (*gorm.DB, error) {
    // 1. 连接数据库
    db, err := gorm.Open(postgres.Open(config.Database.DSN), &gorm.Config{})
    if err != nil {
        return nil, err
    }
    
    // 2. 执行迁移
    migrator := migration.NewMigrator(db, config.Migration.Enabled, config.Migration.Source)
    if err := migrator.Migrate(); err != nil {
        return nil, fmt.Errorf("migration failed: %w", err)
    }
    
    return db, nil
}
```

### 6.2 手动执行迁移

```go
// cmd/migrate/main.go

package main

import (
    "log"
    "server/internal/config"
    "server/internal/pkg/migration"
    "gorm.io/driver/postgres"
    "gorm.io/gorm"
)

func main() {
    cfg := config.Load("config.toml")
    
    db, err := gorm.Open(postgres.Open(cfg.Database.DSN), &gorm.Config{})
    if err != nil {
        log.Fatal(err)
    }
    
    migrator := migration.NewMigrator(db, true, "./migrations")
    if err := migrator.Migrate(); err != nil {
        log.Fatal(err)
    }
    
    log.Println("Migration completed successfully")
}
```

---

## 七、二次开发示例

### 7.1 新增业务模块迁移

```sql
-- migrations/table_order.sql

-- 订单表
CREATE TABLE IF NOT EXISTS orders (
    id SERIAL PRIMARY KEY,
    order_no VARCHAR(64) NOT NULL UNIQUE,
    user_id INTEGER NOT NULL,
    total_amount DECIMAL(10, 2) NOT NULL,
    status SMALLINT DEFAULT 1,
    created_at BIGINT DEFAULT EXTRACT(EPOCH FROM NOW()),
    updated_at BIGINT DEFAULT EXTRACT(EPOCH FROM NOW())
);

-- 订单项表
CREATE TABLE IF NOT EXISTS order_items (
    id SERIAL PRIMARY KEY,
    order_id INTEGER NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    product_id INTEGER NOT NULL,
    product_name VARCHAR(256) NOT NULL,
    price DECIMAL(10, 2) NOT NULL,
    quantity INTEGER NOT NULL,
    created_at BIGINT DEFAULT EXTRACT(EPOCH FROM NOW())
);

-- 索引
CREATE INDEX IF NOT EXISTS idx_orders_user_id ON orders(user_id);
CREATE INDEX IF NOT EXISTS idx_orders_status ON orders(status);
CREATE INDEX IF NOT EXISTS idx_order_items_order_id ON order_items(order_id);
```

```sql
-- migrations/data_order.sql

-- 插入订单状态字典
INSERT INTO sys_dict_type (code, name, description, status) VALUES
('order_status', '订单状态', '订单的各种状态', 1);

INSERT INTO sys_dict_data (type_code, label, value, sort, status) VALUES
('order_status', '待支付', 'pending', 1, 1),
('order_status', '已支付', 'paid', 2, 1),
('order_status', '已发货', 'shipped', 3, 1),
('order_status', '已完成', 'completed', 4, 1),
('order_status', '已取消', 'cancelled', 5, 1);
```

### 7.2 表结构变更

```sql
-- migrations/alter_article_add_tags.sql

-- 文章表新增tags字段
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'articles' AND column_name = 'tags'
    ) THEN
        ALTER TABLE articles ADD COLUMN tags VARCHAR(512);
        COMMENT ON COLUMN articles.tags IS '文章标签，逗号分隔';
    END IF;
END $$;

-- 文章表新增source字段
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'articles' AND column_name = 'source'
    ) THEN
        ALTER TABLE articles ADD COLUMN source VARCHAR(256);
        ALTER TABLE articles ADD COLUMN source_url VARCHAR(512);
    END IF;
END $$;
```

### 7.3 创建索引

```sql
-- migrations/index_article_search.sql

-- 为文章搜索创建索引
CREATE INDEX IF NOT EXISTS idx_articles_title_search 
ON articles USING gin(to_tsvector('chinese', title));

CREATE INDEX IF NOT EXISTS idx_articles_content_search 
ON articles USING gin(to_tsvector('chinese', content));

-- 复合索引
CREATE INDEX IF NOT EXISTS idx_articles_status_publish_time 
ON articles(status, publish_time DESC);
```

---

## 八、版本管理策略

### 8.1 版本号规则

```
格式：YYYYMMDD_NNN

示例：
- 20240115_001  # 2024年1月15日的第1个迁移
- 20240115_002  # 2024年1月15日的第2个迁移
- 20240120_001  # 2024年1月20日的第1个迁移
```

### 8.2 迁移回滚

```go
// internal/pkg/migration/migrator.go

// Rollback 回滚指定版本的迁移
func (m *Migrator) Rollback(version string) error {
    // 1. 检查迁移是否已执行
    executed, err := m.isExecuted(version)
    if err != nil {
        return err
    }
    if !executed {
        return fmt.Errorf("migration %s not executed", version)
    }
    
    // 2. 读取回滚脚本
    rollbackSQL, err := m.readRollbackScript(version)
    if err != nil {
        return err
    }
    
    // 3. 执行回滚
    if err := m.db.Exec(rollbackSQL).Error; err != nil {
        return err
    }
    
    // 4. 删除迁移记录
    return m.db.Exec("DELETE FROM schema_migrations WHERE version = ?", version).Error
}
```

---

## 九、最佳实践

1. **开发环境**：开启自动迁移，方便快速迭代
2. **生产环境**：关闭自动迁移，手动执行并备份
3. **脚本测试**：迁移脚本在开发环境充分测试后再提交
4. **数据备份**：生产环境执行前务必备份数据库
5. **事务包裹**：每个迁移脚本应在事务中执行
6. **版本锁定**：发布后不再修改已执行的迁移脚本

---

## 十、相关文档

- [Server架构设计](./server-architecture.md)
- [PostgreSQL文档](https://www.postgresql.org/docs/)
