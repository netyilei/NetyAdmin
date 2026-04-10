# Server（Go）目录结构与分层

## 顶层结构（关键文件）

```text
server/
├── cmd/
│   └── server/
│       └── main.go                # 进程入口（加载 config.toml -> InitDB -> Bootstrap -> Run）
├── config.toml                    # 运行配置（TOML）
├── go.mod
├── go.sum
├── migrations/                    # SQL 迁移脚本目录
└── internal/                      # 私有业务代码（不对外暴露）
```

## internal/ 的主要分区

```text
internal/
├── app/                           # 应用启动与依赖装配（Bootstrap / Run / InitDB）
├── config/                        # 配置结构与加载（TOML）
├── domain/                        # 领域模型（DTO / Entity / VO）
│   ├── dto/                       # 入参/出参（请求模型、分页等）
│   ├── entity/                    # 持久化实体（GORM Model）
│   └── vo/                        # 面向前端的 View Object
├── handler/                       # HTTP Handler（控制器层）
│   └── v1/                        # /admin/v1 下的业务 Handler
├── job/                           # 内置任务（迁移、定时发布、日志清理等）
├── middleware/                    # Gin 中间件（JWT、RBAC、RequestID、日志、超时、恢复）
├── pkg/                           # 可复用基础设施包（cache/redis/jwt/task/storage/migration 等）
├── repository/                    # 数据访问层（按业务域拆分）
├── router/                        # 路由注册（/admin/v1 分组）
└── service/                       # 业务服务层（按业务域拆分）
```

## 分层调用链（当前代码口径）

当前实现基本遵循：

```text
router -> handler -> service -> repository -> entity
```

- **handler**：负责参数绑定、鉴权上下文读取、调用 service、输出统一响应
- **service**：承载业务规则、聚合多个 repository、做缓存/配置联动
- **repository**：承载 CRUD 与查询拼装（GORM）
- **domain/entity**：数据库实体与表结构映射
- **domain/dto / domain/vo**：前后端契约模型与展示模型

## 目录内的“模块划分”方式

后端模块以业务域拆分为多个子包（而非把所有 handler/service/repo 混在一起）：

- `system/*`：RBAC、系统配置、字典、任务
- `content/*`：分类、文章、Banner
- `storage/*`：对象存储配置、上传凭证、上传记录
- `log/*`：操作日志、错误日志
