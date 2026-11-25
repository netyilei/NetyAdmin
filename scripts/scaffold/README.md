# scaffold（模块脚手架）

标准模块骨架生成器（CLI 后置，脚本即入口）。仅支持 `--key=value` 传参形式。

```bash
node scripts/scaffold/new-module.mjs <name> [--kind=platform-service|business|edge] [--owner=platform-team]
```

> 覆盖范围（对照wizard 设想）：当前生成**模块骨架 + Manifest + module.go
> 标准装配骨架（embed 迁移/Init 注册/AttachDB 惯例——scaffold 收拢，audit/ipac/message/
> users 手写模式收进模板）+ keys 快照测试骨架**；deployment.yaml、契约测试套件、
> 模块文档生成随后续里程碑回填，不在本脚本范围。

## 生成物（标准结构）

```
internal/modules/<name>/
├── module.yaml        # Manifest
├── module.go          # 可编译标准装配骨架：Migrations()（database.MigrationsOf 收拢）/
│                      # Init 迁移注册 / AttachDB 占位（有表模块直接填 repo 注入）
├── api/
│   ├── keys/          # key 唯一封装（namespaces.md 铁律）+ 快照测试骨架
│   ├── rpc/           # RPC 路由常量唯一封装
│   └── events/        # 事件 Topic 唯一封装（架构 §5.5）
├── domain/            # 实体/领域事件/仓储接口（依赖禁令随 doc 走）
├── application/       # 用例/事务边界
├── adapters/          # 仓储实现/外部系统适配
└── migrations/        # .gitkeep 占位（all:migrations embed 保编译；首迁移时建文件，
                       # check-migrations 校验 + migrate.go 登记对偶门禁双向强制）
```

## 生成后必做（脚本尾部会再打印）

1. `contracts/namespaces.yaml` 登记模块 prefix 与包路径；
2. `contracts/migrations.yaml` 登记表序号段；
3. `cmd/netyadmin` 登记：assembly.go registerModules 模块实例 + migrate.go
   moduleMigrations 迁移源（首个迁移文件落地后必须）；
4. module.yaml 填 depends_on/ports/events/tables/scaling.mode；
5. `node scripts/ci/run.mjs` 门禁确认绿；
6. contracts 变更后重新规则 ACK。

## 约束

- 模块名：小写字母开头的字母数字单词（`^[a-z][a-z0-9]*$`），与目录/包名一致。
- 不覆盖已存在模块；生成物可编译、可过全部门禁。
- 下游复用：约定待 guide 演练回填 docs/guide/downstream-development-rules.md。
