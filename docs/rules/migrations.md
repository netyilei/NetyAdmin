# 数据库迁移规范（V2）

> 版本：v1.0 · 2026-11-11
> 单一事实源：`contracts/migrations.yaml`（号段/模块分配）
> 关联：§3、`docs/rules/state-and-cache.md`、`docs/rules/multi-node-scaling.md`

## 1. 五条铁律

1. **不迁移 V1 迁移文件**：V2 从空库重写，从 `11001` 开始；V1 的 61 个文件不复用。
2. **禁止补丁式**：V2.0 首次发布前，表变更直接改原编号文件；不新增 `_fix/_alter/_v2/_patch`。
3. **发布后不可改**：已应用迁移禁止编辑；新增编号 + expand-contract；runner checksum 拦截。
4. **表结构与种子分离**：每表一个 schema 文件 + 一个 seed 文件；DDL 与 INSERT 不混。
5. **其他内容独立文件**：跨表索引/约束/视图/函数/触发器 → `12000-12499`；测试种子 → `13xxx`。

> 发布边界：V2.0 首次生产发布前 = 可改原文件；发布后 = 只能新增迁移。

## 2. 编号段

| 范围 | 用途 | 示例 |
|------|------|------|
| `11000-11999` | 表结构（每表一个；基座表序号 001-499） | `11001_identity_users_schema.up.sql` |
| `12000-12499` | 跨表结构补充（索引/约束/视图/函数/触发器） | `12001_rbac_role_menu_indexes.up.sql` |
| `12500-12999` | 项目预置种子（每表一个；`12500+NNN`） | `12501_identity_users_seed.up.sql` |
| `13000-13999` | 测试种子（仅测试环境；`13000+NNN`） | `13001_test_identity_users_seed.up.sql` |
| `14000-19999` | 基座保留（数据修复/回填等，需 ADR） | — |
| `20000-99999` | 下游用户 | 见 §6 |

**配对规则**：表序号 `NNN`（001-499）——schema=`11000+NNN`，项目种子=`12500+NNN`，测试种子=`13000+NNN`；`12xxx` 独立编号。

## 3. 文件名

```
NNNNN_<module>_<table>_<kind>.up.sql
NNNNN_<module>_<table>_<kind>.down.sql
```

- `kind`：`schema | seed | indexes | constraints | views | functions | triggers | test_seed | backfill`
- 禁止：`_fix`、`_alter`、`_v2`、`_patch`、`_new`。
- 一个 schema 文件只建一张表；seed 文件只做数据；索引/约束独立文件。

示例：

```
11001_identity_users_schema.up.sql
12501_identity_users_seed.up.sql
12001_rbac_role_menu_indexes.up.sql
13001_test_identity_users_seed.up.sql
```

## 4. 表结构 / 种子分离

### schema 文件

- `CREATE TABLE`、列、主键、非空、基础 CHECK、注释。
- 不写 `INSERT`；不写业务种子。
- 二级索引/外键/跨表约束放 `12xxx`（`indexes/constraints`）。
- 新表从所属模块段取下一个号；更新 `contracts/migrations.yaml`。

### seed 文件

- 只写 `INSERT`；幂等（`ON CONFLICT DO NOTHING` 或稳定唯一键）。
- down 只删除本 seed 写入的行（按稳定 ID/code），不 `TRUNCATE` 整表。
- 不写 DDL；不含 PII/密钥；测试数据放 `13xxx`。

### 其他文件

| kind | 用途 |
|------|------|
| `indexes` | 二级索引、复合索引、部分索引 |
| `constraints` | 外键、CHECK、唯一约束（跨表/补充） |
| `views` | 视图 |
| `functions` | 函数/存储过程 |
| `triggers` | 触发器 |
| `backfill` | 数据回填（幂等、可恢复、限流） |
| `test_seed` | 仅测试环境；生产禁止 |

## 5. 模块表号段（初始）

| 模块 | 表序号 `NNN` | 说明 |
|------|-------------|------|
| identity | 001-049 | users/admins/tokens/oauth/verification |
| rbac | 050-079 | roles/menus/apis/buttons/relations |
| configcenter | 080-099 | sys_configs |
| audit | 100-119 | operation_logs |
| ipac | 120-139 | ipac rules/app policies |
| storage | 140-179 | storage config/upload records |
| message | 180-219 | templates/records/internal messages |
| gateway | 250-289 | apps/scopes/apis/logs |
| scheduler | 290-309 | run records/job logs |
| realtime | 310-329 | 如需持久化连接/频道元数据 |
| bus | 330-339 | kernel/bus 设施表（outbox/dlq 等） |
| core | 340-399 | 其余内核/平台设施表；按归属细分 |
| reserved | 400-499 | 基座预留 |
| reserved_ext | 500-999 | 扩展需 ADR |

- 新表在所属模块段取下一个号；更新 `contracts/migrations.yaml`。
- 超段上限先申请扩展；禁止越段占用。
- 跨表索引/约束放 `12xxx`，文件名写清目标表。

## 6. 下游规则

- `1xxxx` 基座保留，下游禁用；下游只用 `2xxxx-9xxxx`。
- 每个项目占一个首位：`2xxxx`、`3xxxx` … `9xxxx`。
- 项目内按模块 `M=1..9` 分块（`P0xxx` 为项目公共）：
  - `P0-P9` schema
  - `P0-P9` indexes/constraints
  - `P0-P9` seed
  - `P0-P9` test seed
  - `P0-P9` reserved
- 示例：项目 2、模块 1 → schema `21000-21299`、索引 `21400-21599`、seed `21600-21799`、test `21800-21899`。
- 在 `contracts/migrations.local.yaml` 登记项目/模块/段；**不得修改基座迁移与 contracts**。
- 同样遵守 schema/seed 分离与"发布后 expand-contract"。

## 7. Runner 与校验

- 业务模块迁移放 `internal/modules/<module>/migrations/`；kernel 设施迁移放对应包内 `internal/kernel/<pkg>/migrations/`（模块名取登记段，如 bus）。
- runner 只扫描已启用模块，按 5 位全局排序执行。
- 记录表：goose `goose_db_version(version_id)` + 自建 `migration_checksums(module, version, sha256)`（双表——模块归属与 checksum 分表存储）。
- checksum 不一致 → 失败；禁止修改已应用文件。开发期改原编号文件（§2）后存量库恢复 runbook：`DELETE FROM goose_db_version WHERE version_id=<受影响号段>` + `DELETE FROM migration_checksums WHERE module='<模块>'` → 重跑 migrate（表结构需 DROP 重建时先 DROP CASCADE）。
- 测试种子只在 `--with-test-seed` 执行；生产禁止。
- down：schema/seed/test_seed 必须提供；不可逆变更在 down 中显式 `RAISE`/返回错误并注释。

## 8. CI 门禁

- [ ] 5 位前缀、段归属正确。
- [ ] 文件名符合 `NNNNN_module_table_kind`。
- [ ] schema 文件无 INSERT；seed 文件无 DDL。
- [ ] 无 `_fix/_alter/_v2/_patch/_new` 后缀。
- [ ] 编号唯一、无重复。
- [ ] 每张表有 schema；有种子则 seed 同 `NNN`。
- [ ] 12xxx 索引/约束写清目标表。
- [ ] 测试种子不与生产种子混。
- [ ] down 文件存在。
- [ ] 模块已登记 `contracts/migrations.yaml`。
- [ ] 已应用迁移 checksum 未变。

## 9. 反模式

- ❌ 把 V1 迁移文件复制进 V2。
- ❌ 用 `_fix`/`_alter` 追加补丁迁移（开发期应改原文件）。
- ❌ schema 与 seed 混在一个文件。
- ❌ 一个 schema 文件建多张表。
- ❌ seed down 用 `TRUNCATE`/`DELETE` 不带条件。
- ❌ 下游占用 `1xxxx` 或修改基座迁移。
- ❌ 修改已应用迁移（checksum 会失败）。
- ❌ 测试种子在生产执行。

## 10. V1 → V2 数据迁移

- 独立工具 `cmd/migrate-v1-v2`，不是 SQL 迁移文件。
- 保留 `operation_logs`；丢弃 dict；不建 `error_logs`；open platform 表映射 `gateway_*`。
- dry-run、备份、行数/校验和/抽样、回滚演练。

## 11. 表与列规范

| 项 | 规则 |
|----|------|
| 表名 | `<module>_<entity>`，单数、snake_case；registry 唯一 |
| 主键 | `id`：ULID string（默认）或 bigint，同模块统一 |
| 必备列 | `created_at` / `updated_at`（timestamptz，UTC） |
| 操作人 | 业务写表必带 `created_by` / `updated_by`（ULID，可空=system） |
| 软删 | 按表声明（Manifest `soft_delete`）；默认硬删 + 审计；软删带 `deleted_at` + 部分唯一索引 |
| 乐观锁 | 并发编辑聚合根必带 `version int`，UPDATE 带 `WHERE version=?` |
| 状态机 | 多状态实体：状态列枚举 + 合法迁移表（测试穷举）；禁字符串散判断 |
| 枚举 | smallint + 静态枚举/字典；禁 varchar 魔法值 |
| 金额/数量 | bigint 最小单位；禁 float |
| JSONB | 仅快照/扩展属性；禁作关联查询依据 |
| 索引 | `idx_<table>_<cols>`；唯一 `uk_<table>_<cols>`；模块内外键 `fk_<table>_<ref>` |
| 大表 | 设计期分区/归档策略写入 Manifest |
| 事务 | TM 默认超时 5s；长事务显式声明并说明 |

- 业务表查询列必须有索引（EXPLAIN 验证，G2）；列表默认 offset + 上限，>10 万行用 cursor。
- 二级索引放 `12xxx`；schema 文件只含主键/非空/基础 CHECK。
