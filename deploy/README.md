# 部署说明

NetyAdmin 是**单个二进制 + 配置 + 前端产物**的部署模型（不使用 docker）。本目录提供配置样例与部署形态说明。

> 从零到跑起来的完整命令序列（全部参数、环境变量、部署目录结构、systemd 单元、运维命令、排错手册）：**[启动与运行完整手册](../docs/guide/getting-started.md)**。本文只讲**部署形态选型与升级策略**——操作步骤不在此重复。

## 部署产物

| 项 | 说明 |
|----|------|
| `netyadmin` 二进制 | `go build -o netyadmin ./cmd/netyadmin`；同一二进制通过配置切换形态 |
| `config.yaml` | 从 [config.example.yaml](config.example.yaml) 复制并按环境修改（不入库） |
| `keys/jwt_active.pem` | dev 首启自动生成；**staging/prod 必须提前放置**（缺失=拒绝启动），需妥善备份与轮换 |
| `admin/dist/` | 管理端构建产物（`cd admin && pnpm build:pro`）；存在时后端同源挂载 SPA，缺失=纯 API 形态 |
| `logs/` | 运行/访问/错误/安全/审计日志目录（按日期分目录，自动轮转与保留） |
| 环境变量 | 密钥只走 env（DB 密码、`NETYADMIN_GW_SECRET_KEY`、SMTP 密码、存储 HMAC 盐、Cache L2 密钥）；全表见启动手册 §7 |

外部依赖：PostgreSQL（必需）、Redis（single 可选、multi 必需）、S3 兼容对象存储（使用 storage 时）。

## 部署形态

| 形态 | 配置 | 说明 |
|------|------|------|
| 单机 all-in-one | `topology: single` + `role: all` | 一个进程挂全部接入面；默认 8080 |
| 接入面拆分 | 多进程 `role: core` / `role: admin`，共享 PG/Redis | core=平台服务 + RPC 宿主 + 运维面；admin=HTTP 门 + RPC 宿主（无 client 门）；模块仍全量装配（共享单库） |
| 多副本水平扩展 | `topology: multi` + 同配置多副本 | 必须同一 Redis；会话/锁/ticket/配额全部外置；按模块 Manifest `scaling.mode` 声明伸缩方式；每副本 `deployment.node_id` 必须不同 |

- 拆分部署时，跨进程调用经内部 RPC：`binding[].base_urls` 显式配置目标实例，接收方 `rpc.allowed_callers` 必须登记调用方服务名——**空=全拒（fail-closed）**；`deployment.service_name` 是本进程的服务标识。
- 前端与 TLS：建议由 Nginx/网关终止 TLS 并反代 8080；`server.env=prod` 时下发 HSTS，`server.csp` 空=Report-Only 宽策略（显式配置后强制）。
- 配置热更边界：日志级别、配置中心键值等可运行期生效；**监听地址、DSN、Redis 地址、模块启停不热更**，需滚动重启。

形态是否可行取决于模块的伸缩声明（`scaling.mode: stateless|leader|sharded|stateful`）与状态外置程度，见 [多机部署与水平扩展](../docs/rules/multi-node-scaling.md)。

## 升级策略

迁移命令（`-mode=migrate -migrate-action=up|status`，advisory lock 保护，serve 永不执行迁移）见 [启动手册 §10](../docs/guide/getting-started.md#10-日常运维命令)。升级按以下纪律执行：

1. 升级前阅读 [CHANGELOG](../CHANGELOG.md) 的破坏性变更与升级动作，备份数据库。
2. 先迁移后启动新版本（滚动发布按 N-1 兼容窗口，expand-contract 为先加后用、再清理）。
3. 逐副本替换，观察 `/health/ready` 与错误日志；失败回滚到上一版本二进制（schema 变更按 expand-contract 保持向后兼容）。
4. 回滚边界：数据库迁移在发布窗口内不做 down（回滚=回退二进制，靠 expand-contract 保证旧版本可读新 schema）。

版本与发布规则见 [版本与发布流程](../docs/rules/release.md)；回滚与热更边界见 [多机热更](../docs/rules/hot-update.md)。

## 安全基线

上线前检查：`server.env=prod`（强制 HSTS / WS 明文拒绝 / JWT 密钥预置）、`server.cors_origins` 显式登记（禁通配符）、密钥全部经环境变量注入、TLS 由边缘终结。完整清单见 [安全基线](../docs/rules/security.md)。

适用版本 v2.0.0
