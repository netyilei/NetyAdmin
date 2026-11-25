# 启动与运行完整手册

> 读者：第一次把 NetyAdmin 跑起来的人。本文把每一个命令、每一个参数、每一个必设变量写全——照抄即可启动，不需要读源码。
> 只想 5 分钟看到登录页：直接跳到 [§4 最快路径](#4-最快路径源码方式5-分钟)。

## 1. 你需要什么

| 依赖 | 版本 | 是否必需 | 说明 |
|------|------|:--------:|------|
| Go | ≥ 1.26 | 源码方式必需 | 只用发布二进制可跳过 |
| PostgreSQL | 18（14+ 理论可用） | **必需** | 唯一数据真相源 |
| Redis | 8（7+ 可用） | 单机可选 / 多机必需 | `topology=single` 且未配置 `redis.addr` 时自动降级为进程内实现 |
| Node + pnpm | Node ≥ 20 / pnpm ≥ 9 | 只做前端时 | 管理端源码 `admin/`；只用 API 可跳过 |
| S3 兼容对象存储 | — | 使用文件上传时 | MinIO / COS / OSS 等；不启用 storage 可跳过 |

## 2. 运行时目录契约（先理解这个，少走一半弯路）

二进制**以启动时的工作目录为根**解析相对路径。以下五个相对路径决定一切：

```
<工作目录>/
├── netyadmin              # 二进制（或 go run 时任意）
├── config.yaml            # 配置文件（-config 可改；不存在=纯默认值+环境变量）
├── keys/jwt_active.pem    # JWT 签发密钥；dev 首启自动生成；staging/prod 必须预置
├── admin/dist/            # 管理端构建产物；存在=后端同源挂载 SPA，缺失=纯 API 形态
└── logs/<日期>/           # 运行/访问/错误/审计日志（自动创建；按日期分目录）
```

> 踩坑高发：在 A 目录构建、在 B 目录启动，会出现「配置没生效」「密钥重新生成」「前端 404」。**先 `cd` 到部署目录再启动**，或全程用 `-config /abs/path/config.yaml` 加绝对路径。

## 3. 命令行参数全表

```bash
netyadmin [-mode serve|migrate|reset-password] [-migrate-action up|status]
          [-username <管理员名>] [-config config.yaml] [-addr 0.0.0.0:8080] [-version]
```

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-mode` | `serve` | `serve`=启动服务；`migrate`=数据库迁移子命令；`reset-password`=运维重置管理员密码 |
| `-migrate-action` | `up` | `-mode=migrate` 时有效：`up`=执行全部待应用迁移；`status`=只打印迁移状态 |
| `-username` | 空 | `-mode=reset-password` 时**必填**，目标管理员用户名 |
| `-config` | `config.yaml` | 配置文件路径；文件不存在时使用内置默认值（仍可用环境变量覆盖） |
| `-addr` | 空 | 覆盖 `server.http_port` 的监听地址，如 `:8080` / `0.0.0.0:8080` / `127.0.0.1:9000` |
| `-version` | — | 打印版本与构建 commit 后退出 |

## 4. 最快路径（源码方式，5 分钟）

按顺序执行，每步都给出预期输出：

```bash
# ① 建库（用你的 PG 超级用户或已授权账号执行一次）
psql -h 127.0.0.1 -U postgres -c "CREATE DATABASE netyadmin_v2"

# ② 进入仓库根目录，复制配置样例
cp deploy/config.example.yaml config.yaml

# ③ 编辑 config.yaml：把 database.user 改成你的 PG 用户（其余默认值可先用）
#    最小需要改的一行：
#      database:
#        user: postgres          # ← 你执行 ① 用的账号

# ④ 注入两个必设环境变量（密钥类只走环境变量，永不进配置文件）
export NETYADMIN_DATABASE_PASSWORD=你的PG密码
export NETYADMIN_GW_SECRET_KEY=$(openssl rand -base64 32)

# ⑤ 迁移：建表 + 创建初始管理员 + 自举网关应用
go run ./cmd/netyadmin -mode=migrate -migrate-action=up
```

第 ⑤ 步输出末尾会打印**一次性的初始管理员密码**（只打印这一次，请立即保存）：

```
==============================================
bootstrap admin created:
  username: admin
  password: Xk9mP2vQ7rT4
  (shown once; must_change_password enforced on first login)
==============================================
```

```bash
# ⑥ 启动服务（前台运行；默认监听 :8080）
go run ./cmd/netyadmin

# 预期日志末行类似：
#   level=INFO msg="http server listening" addr=[::]:8080 env=dev topology=single

# ⑦ 另开终端验证
curl -s http:/7.0.0.1:8080/health/live    # → {"status":"ok"}
curl -s http:/7.0.0.1:8080/health/ready   # → {"status":"ok","checks":{"pg":"ok","redis":"ok"}}
```

> single 拓扑未配置 Redis 时 `checks` 中**没有** `redis` 键（进程内降级，属正常）；multi 拓扑 Redis 不通则 `status":"unready"` 且 ready 返回 503。

```bash
# ⑧ 登录（API 方式；需要先取验证码）
curl -s http:/7.0.0.1:8080/admin/v1/auth/captcha
# → {"code":10000,"data":{"captchaId":"cap_xxx","image":"..."}}
#    image 是 base64 图片；存文件或直接解码查看其中 4 位字符

curl -s -X POST http:/7.0.0.1:8080/admin/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"<⑤步的密码>","captchaId":"<上一步captchaId>","captchaCode":"<图中的4位字符>"}'
# → {"code":10000,"data":{"access_token":"eyJ...","refresh_token":"eyJ...","expires_in":7200,
#                          "must_change_password":true,...}}
#    首次登录 must_change_password=true —— 先改密再继续：
#    POST /admin/v1/auth/change-password  {"old_password":"...","new_password":"..."}
#    token 过期后刷新：POST /admin/v1/auth/refresh  {"refresh_token":"..."}（一次性轮换）
```

浏览器方式：管理端产物 `admin/dist` 存在时直接打开 `http:/7.0.0.1:8080`；未构建则按 [§6 前端](#6-前端启动两种方式) 用开发模式。

## 5. 二进制部署方式

发布产物为 `netyadmin_v2.0.0_<os>_<arch>[.exe]` 单文件 + 前端包 `admin-dist_v2.0.0.tar.gz`。部署目录：

```
/opt/netyadmin/
├── netyadmin                       # 发布二进制
├── config.yaml                     # cp deploy/config.example.yaml config.yaml 后修改
├── keys/jwt_active.pem             # 预置（见下）
├── admin/
│   └── dist/                       # tar -xzf admin-dist_v2.0.0.tar.gz 解到 admin/ 下
└── logs/                           # 自动创建
```

```bash
# ① 解压前端产物到 admin/ 下（包内顶层目录即 dist）
tar -xzf admin-dist_v2.0.0.tar.gz -C /opt/netyadmin/admin/

# ② 预置 JWT 密钥（staging/prod 必须；缺失=拒绝启动）
mkdir -p /opt/netyadmin/keys && cd /opt/netyadmin
openssl genrsa -out keys/jwt_active.pem 2048 && chmod 600 keys/jwt_active.pem
# dev 环境可跳过——首次启动自动生成到 keys/ 下

# ③ 配置 + 环境变量（同 §4 的 ②③④）

# ④ 迁移 + 启动（cd 到部署目录，相对路径才正确）
cd /opt/netyadmin
./netyadmin -mode=migrate -migrate-action=up
./netyadmin -mode=serve
```

systemd 单元参考（`/etc/systemd/system/netyadmin.service`）：

```ini
[Unit]
Description=NetyAdmin
After=network.target postgresql.service redis.service

[Service]
Type=simple
User=netyadmin
WorkingDirectory=/opt/netyadmin
EnvironmentFile=/etc/netyadmin/env        # 一行一个 KEY=VALUE，含两个必设密钥
ExecStart=/opt/netyadmin/netyadmin -mode=serve
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

```bash
systemctl daemon-reload && systemctl enable --now netyadmin
journalctl -u netyadmin -f          # stdout 日志；文件日志在 /opt/netyadmin/logs/
```

## 6. 前端启动（两种方式）

**开发模式**（热更新，日常改前端用）：

```bash
cd admin
pnpm install
pnpm dev          # → http://localhost:5173，Vite 已代理 /admin/v1 与 /client/v1 到本机 8080
```

前置：后端已在 8080 启动；`config.yaml` 的 `realtime.admin_origins` 含 `http://localhost:5173`（样例默认已含）。

**生产构建**（产物给后端同源挂载）：

```bash
cd admin
pnpm build:pro    # → admin/dist/
```

随后启动后端即自动挂载：`http:/7.0.0.1:8080` 直接打开管理端。SPA 路由回退、静态资源与 API 同源，无需额外 Nginx 配置（对外建议仍由 Nginx/网关终止 TLS 再反代 8080）。

## 7. 环境变量全表

所有环境变量统一 `NETYADMIN_` 前缀；优先级：默认值 < `config.yaml` < 环境变量 < 密钥类（env-only）。**密钥类没有 yaml 入口**，只能走环境变量。

### 必设（启动即校验，缺失 fail-closed）

| 变量 | 值 | 说明 |
|------|-----|------|
| `NETYADMIN_DATABASE_PASSWORD` | PG 连接密码 | `database.dsn` 显式配置时可不设 |
| `NETYADMIN_GW_SECRET_KEY` | base64 编码的 32 字节随机串 | 应用 AppSecret 的 AES-256-GCM 加密钥；`openssl rand -base64 32` 生成；缺失/非法=拒绝启动；**丢失=所有应用密钥无法解密，须妥善备份** |

### 按需设置

| 变量 | 默认 | 何时需要 |
|------|------|----------|
| `NETYADMIN_ENV` | `dev` | 设 `prod` 时强制 HTTPS 相关基线（HSTS、WS 明文拒绝、JWT 密钥必须预置） |
| `NETYADMIN_DEPLOYMENT_TOPOLOGY` | `single` | 多副本部署设 `multi`（必须同时配 Redis） |
| `NETYADMIN_DEPLOYMENT_NODE_ID` | `node-1` | multi 拓扑**每个副本必须不同**；字符集 `[a-z0-9._-]`，≤64 |
| `NETYADMIN_REDIS_ADDR` | 空 | 如 `127.0.0.1:6379`；single 可空，multi 必填 |
| `NETYADMIN_SERVER_HTTP_PORT` | `8080` | 改监听端口（也可用 `-addr`） |
| `NETYADMIN_DATABASE_MAX_CONNS` | 0=按 CPU 自动 | 多实例部署时显式规划：进程数 × 池上限 ≤ PG `max_connections` |
| `NETYADMIN_CACHE_L2_KEY` | 空 | base64 32 字节；启用含 PII 的缓存实体时必需，缺失=该实体注册失败 |
| `NETYADMIN_STORAGE_HMAC_SALT` | 空 | 使用 storage 直传时必需；缺失=直传完成接口全拒 |
| `NETYADMIN_MESSAGE_SMTP_PASSWORD` | 空 | 启用 SMTP 邮件时必需 |
| `NETYADMIN_REALTIME_MAX_CONNS_TOTAL` | `100000` | WS 节点级连接总数上限（0=不限） |
| `NETYADMIN_REALTIME_MAX_CONNS_PER_APP` | `10000` | 单应用 WS 连接上限 |
| `NETYADMIN_REALTIME_MAX_MESSAGE_RATE_PER_APP_SEC` | `1000` | 单应用消息速率上限（条/秒） |

> 全量映射（含每个字段）见 `internal/kernel/config/config.go` 各结构体的 `env` 标签；本文列的是实际会改到的部分。

## 8. 配置文件详解

从 `deploy/config.example.yaml` 复制。四个必改/常改位置：

```yaml
server:
  env: dev                 # staging/prod：HSTS + WS 强制 HTTPS + JWT 密钥必须预置
  http_port: 8080
  # cors_origins:          # 浏览器跨域白名单；默认空=不启用；通配符 * 启动拒绝
  #   - https://admin.example.com
  # trusted_proxies:       # 反向代理 CIDR 白名单（X-Forwarded-For 信任面）
  #   - 127.0.0.1       #   反代/LB 之后部署必须登记，否则审计/限流/IPAC 全按代理 IP；空=不信任代理头

deployment:
  topology: single         # multi=多副本（必须 Redis）
  role: all                # all | core | admin（接入面拆分部署时用）
  node_id: node-1          # multi 每副本不同

database:
  host: 127.0.0.1
  port: 5432
  name: netyadmin_v2
  user: netyadmin          # ← 必改：你的 PG 用户
  sslmode: disable

redis:
  addr: 127.0.0.1:6379     # single 可不装（降级进程内）；multi 必填
  db: 0
  prefix: na
```

其余段（`log` / `audit` / `storage` / `message` / `users` / `realtime` / `observability`）样例中已含默认值（`observability.alerts` 为观测页阈值覆盖，默认集见 [可观测契约 §6](../rules/observability.md#6-告警最小集与-slomd-联动)）。三个规则：

1. **模块段必须存在**（`identity: {}` / `rbac: {}` / `configcenter: {}` / `gw: {}` 空段即可）——删掉会导致启动报 `config section ... not present`。
2. 启动日志出现 `config warning: 未知配置段：xxx` 属正常——这些是模块消费的段，不是内核配置结构，不影响运行。
3. **反代部署必须登记 `server.trusted_proxies`**：不登记时不解析 `X-Forwarded-For`（`ClientIP` = 直连地址），审计记录、登录限流、网关 IPAC/配额都会按代理 IP 判定。

## 9. 运行形态

| 形态 | 用法 | 要点 |
|------|------|------|
| 单机 all-in-one | `topology: single` + `role: all` | 默认形态；无 Redis 自动进程内降级 |
| 接入面拆分 | 两进程：`role: core` / `role: admin`，共享同库 | core=平台服务+RPC 宿主+运维面；admin=HTTP 门。跨进程调用须在 `binding[].base_urls` 配目标地址，接收方 `rpc.allowed_callers` 登记调用方服务名（**空=全拒**） |
| 多副本水平扩展 | `topology: multi` + 同配置多副本 | 必须同一 Redis；每副本 `node_id` 不同；会话/锁/限流/配额全外置，任意副本可服务任意请求 |

形态选型与升级策略见 [deploy/README](../../deploy/README.md)；多机伸缩细节见 [多机部署与水平扩展](../rules/multi-node-scaling.md)。

## 10. 日常运维命令

```bash
# 查看迁移状态（当前版本、待应用数量）
./netyadmin -mode=migrate -migrate-action=status

# 升级：先迁移后启新版本（滚动发布、向后兼容窗口）
./netyadmin -mode=migrate -migrate-action=up

# 忘记管理员密码：重置（输出一次性新密码；TokenVersion 自增，所有旧会话立即失效）
./netyadmin -mode=reset-password -username=admin

# 版本信息
./netyadmin -version
```

| 端点 | 用途 |
|------|------|
| `GET /health/live` | 进程存活（容器/守护重启判定） |
| `GET /health/ready` | 依赖就绪（PG/Redis；摘流判定） |
| `GET /metrics` | Prometheus 指标 |
| `GET /admin/v1/system/diagnostics` | 运行诊断快照（需管理员 token） |
| `GET /debug/pprof/*` | 性能剖析；默认关闭，`config.yaml` 设 `server.ops_pprof: true` 后开放 |

日志位置：`logs/<YYYY-MM-DD>/<module>_<type>.log`，类型 `app/access/error/security/audit`；级别与开关支持运行期动态调整（管理端系统设置），debug 默认关闭。

## 11. 排错手册

| 现象 | 原因与处理 |
|------|-----------|
| `config: read file ... no such file` | 工作目录下没有 `config.yaml`。`cp deploy/config.example.yaml config.yaml`，或 `-config` 指绝对路径 |
| `NETYADMIN_GW_SECRET_KEY` 相关启动失败 | 变量缺失或不是 base64 32 字节。重新 `openssl rand -base64 32` 并写入环境变量。注意：换新密钥会导致已存应用 AppSecret 无法解密 |
| `keys: keys/jwt_active.pem missing` | `server.env=staging/prod` 时必须预置密钥文件（`openssl genrsa -out keys/jwt_active.pem 2048`）；dev 自动生成 |
| `config section ... not present` | `config.yaml` 缺模块段。补回 `identity: {}` 等空段 |
| `deployment.topology=multi requires redis` | multi 必须配置 `redis.addr` |
| `pg ping` 失败 / ready 不绿 | 检查 PG 是否运行、账号密码、库是否存在；`psql -h 127.0.0.1 -U <user> -d netyadmin_v2 -c 'select 1'` 手工验证 |
| `docker`? 不需要 | 本项目不使用容器，直接跑二进制；PG/Redis 用系统安装或本机服务 |
| 登录 503 `captcha not wired` | 验证码组件未装配（异常部署）；检查 Redis 可用性或 single 降级是否生效 |
| 登录 400 `captcha invalid` | 验证码一次性且 3 分钟过期：每次登录前重新取 `/admin/v1/auth/captcha` |
| 登录 429 | 登录限流（10 次/分钟/IP）；等待或换 IP |
| 接口 401 | token 过期或被吊销：用 `refresh_token` 调 `/admin/v1/auth/refresh`，或重新登录 |
| 接口 403 | RBAC 权限码未授权：在管理端「角色管理」给当前角色授对应权限 |
| 前端打开空白 / 404 | `admin/dist` 不存在或未在二进制同级目录：`cd admin && pnpm build:pro`，或开发模式 `pnpm dev`（5173） |
| 前端 401 后自动登出 | refresh token 也过期（默认 7 天）——重新登录 |
| WS 连接立即断开 | 对照 `docs/rules/ws-protocol.md` 关闭码表：4001=ticket/JWT 过期、4006=非法帧、1013=过载/限流/连接数超限 |

更多模块级问题见 [下游文档入口](README.md) 的排障速查表。

---

适用版本 v2.0.0
