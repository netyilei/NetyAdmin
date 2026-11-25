# NetyAdmin V2 下游开发规则（AI 提示词包）

> 读者：基于 NetyAdmin V2 基座开发自己项目的团队 / AI 编码助手。
> 用法：把本文档 + 你的项目参数（§0.1）一起交给 AI；AI 先产项目级 `AGENTS.md` 与模块骨架，再写业务。
> 原则：**内核不动、模块自治、契约先行、数据私有、能力可选、多机默认、热更可回滚、日志分流、反过度设计。**

---

## 0. 给 AI 的使用说明

按顺序执行，不要跳步：

1. 读本文档 + 基座 [`docs/rules/README.md`](../rules/README.md) + `contracts/`（状态码/命名空间/能力/Profile/状态策略/迁移号段）。
2. 向用户确认 §0.1 参数；缺失的参数先问，不要猜。
3. 生成项目级 `AGENTS.md`（模板见 §6），只写项目参数与加严项，不复制基座规则正文。
4. 从 `deploy/config.example.yaml` 复制出项目 `config.yaml`：填模块段（含模块开关）、`deployment`（拓扑/角色/节点）、`binding`（RPC 本地/远程）、日志与审计保留。
5. 生成模块清单 + Manifest + 骨架（`api/domain/application/adapters`）。
6. 先写契约（Port/Event/Job/Hook/Migration/Config/权限种子），再写业务。
7. 最后补测试、CI、日志/审计、观测、文档。
8. 交付前跑 §8 检查清单。

### 0.1 必须先确认的项目参数

| 参数 | 取值 / 示例 | 说明 |
|------|-------------|------|
| project id | `myapp` | 记忆与模块前缀标识 |
| 业务模块 | `order, payment, inventory` | 你要新增的模块 |
| Profile 参考 | `lean / standard / full / custom` | 只是模块集合建议（见 `contracts/profiles.yaml`），不产生运行时文件 |
| 模块开关 | `users.enabled` / `message.enabled` / `storage.enabled` / `realtime.enabled` | `false`=零注册零开销；迁移恒登记（表已建、模块关=无路由/Job/goroutine）；菜单按 `rbac_menus.module` 过滤（避免死菜单） |
| 能力 | `gateway.observe, gateway.quota` | 只引用 `contracts/capabilities.yaml` 已登记 ID；新增能力先登记 |
| 部署拓扑 | `single / multi` | `multi` 必须 Redis，缺失=拒绝启动；本地回退仅 `single` |
| 部署角色 | `all / core / admin` | 只过滤接入面挂载；模块仍全量装配、共享单库 |
| 节点标识 | `deployment.node_id` / `deployment.service_name` | multi 每副本 `node_id` 唯一；`service_name`=内部 RPC 调用方标识 |
| RPC 白名单 | `rpc.allowed_callers: [svc-a]` | 空=全拒（接收方 fail-closed）；调用方服务名按实际登记 |
| DB / Redis / OSS | host / 库名 / addr | 只填参数，不写密钥 |
| 密钥（env-only） | `NETYADMIN_DATABASE_PASSWORD` / `NETYADMIN_GW_SECRET_KEY` / `NETYADMIN_CACHE_L2_KEY` / `NETYADMIN_STORAGE_HMAC_SALT` / `NETYADMIN_MESSAGE_SMTP_PASSWORD` | 不入代码/不入库；`GW_SECRET_KEY` 缺失=拒绝启动 |
| 运行环境 | `server.env: dev / staging / prod` | `prod`：HSTS 下发、WS 强制 HTTPS 校验、JWT 密钥文件必需 |
| CSP | `server.csp`（默认空） | 空=发 `Content-Security-Policy-Report-Only` 宽策略；非空=发 enforce |
| realtime 限制 | `realtime.*`（心跳/空闲/队列/连接/频道/帧/速率/Origin/HTTPS） | 默认值即规范基线；9 个数值项必须 >0（显式非正值=拒绝启动）；`max_conns_total=0`=显式不限 |
| SLO | p99 / QPS / 错误率 | 每模块或每接入面 |
| 日志目录 | `./logs` | 运行/错误/访问日志写文件；审计入 DB |
| 审计保留 | 180 天 | `operation_logs` 保留期 |
| 目标环境 | 物理机 / 虚拟机 / K8s | 本机安装 PG/Redis，不使用 docker（部署基线） |

---

## 1. 基座边界（不可越过）

- **Kernel / 平台服务 / 接入面**是基座代码；下游**不改**。
- 下游只写 **业务模块**（`internal/modules/<your-domain>`）与 **部署配置**（`config.yaml` + 环境变量）。
- 模块之间只通过 `api/` Port 或事件通信；**禁止** import 其他模块 `internal/repository/entity`。
- 基座缺失扩展点时，先上报并提设计决策；不要直接改内核。
- 下游模块与基座模块同构：Manifest + Port + Event + Job + Hook + Migration + Config。

### 1.1 下游扩展文件（不修改基座）

- `contracts/namespaces.local.yaml`：下游模块前缀 / keys 登记（活样例：note 模块）。
- `contracts/migrations.local.yaml`：下游迁移号段（活样例：note 模块，项目 2）。
- `contracts/status-codes.local.yaml`：下游 `4xxxx-99999` 状态码（下游自建）。
- `contracts/capabilities.local.yaml`：下游能力 / Profile（下游自建）。
- 基座 `contracts/` 只读；下游登记只落在 `*.local.yaml`。
- 基座的 `scripts/ci` 是**基座号段口径**的参考门禁：下游在自己的仓库里保留同一组检查项，并把登记源/号段扫描口径改为自己的 `*.local.yaml` 与号段（工具层适配，不改运行时）。

### 1.2 范围决策

- **多租户：非目标**；需要时由你的项目自行扩展。
- **C 端用户：示例/参考模块**；基座核心是 admin-auth/RBAC，用户体系按你的业务替换。
- **插件市场：规划内、当前不做**（基座按「模块 = 可分发单元」预留扩展点）；**社区：后置**。
- **CLI：后置**；MVP 用模板/脚本。
- 开源许可证：Apache 2.0。

---

## 2. 十条硬规则

1. **依赖向内**：`kernel ← platform/business ← edges`；禁止反向。
2. **数据私有**：模块独占表；禁止跨模块 JOIN / 事务 / 外键；跨模块只存 ID。
3. **契约单一事实源**：状态码、keys、rpc、channels、events 只允许 owner 模块 `api/` 定义；禁止字面量。**跨模块消费平台服务**（message/storage/audit/configcenter）= `module.ResolveAs` 到对方 `api/` 接口（只能 import 对方 `api/` 子树；禁止 import 对方 application/domain，禁止复制方法签名/反射绕过）。
4. **分层**：`domain` 不 import 框架/`errorx`/`keys`；`application` 不 import `adapters`。
5. **DB 是唯一真相源**；缓存不是；缓存失效在事务 commit 之后。
6. **多机默认**：模块默认无状态；状态外置；本地回退仅 `topology=single`。
7. **热更无热补丁**：代码变更 = 滚动/蓝绿/金丝雀 + N-1 兼容；DB 走 expand-contract。
8. **日志分流**：admin 操作日志 → DB `operation_logs`；运行/错误/访问日志 → 磁盘文件；**禁止 `error_logs` 表**。
9. **能力选择走登记**：能力只引用 `contracts/capabilities.yaml` 已登记 ID；运行时用模块开关与部署角色控制装配；未启用 = 无路由/Job/goroutine/连接；禁止布尔开关矩阵。
10. **反过度设计**：无具名使用者/场景不做；新抽象/配置/依赖/模块/服务须写理由与删除条件。

---

## 3. 如何思考（决策框架）

### 3.1 新功能

先问五问：谁在用？何时用？不用会怎样？最简方案？何时删除？

- 是业务模块还是平台能力？平台能力先查基座是否已有。
- 能否用现有 Port/Event/Job/Hook 完成？不能才新增。
- 新增依赖前答：许可证、维护活跃度、社区、安全、体积。
- 未测量的性能优化不做；先有 SLO/基准。

### 3.2 新模块

- 需要 **2 个真实用例**，否则先放现有模块。
- 声明 `scaling.mode`：`stateless | leader | sharded | stateful`。
- 数据边界：表、迁移、索引、保留策略。
- 依赖方向：只依赖其他模块 `api/`；不依赖接入面。
- SLO：每副本 QPS、p99、内存、连接数。

### 3.3 跨模块一致性

- 单模块多表：UnitOfWork 单事务。
- 跨模块：事件 + outbox / saga / 快照；**禁止分布式事务**。
- 消费幂等：事件 ID / 业务唯一键；重试有上限；失败进 DLQ。
- 跨模块查询：批量 Port + 缓存，禁止 N+1。

### 3.4 多机与伸缩

- 默认无状态；会话/上传/锁/计数/临时文件外置。
- `leader` 任务选主 + 租约；`sharded` 定义分片键与重平衡；`stateful` 定义连接归属与排空。
- 禁止 sticky（WS 可选）；禁止共享本地文件。
- 读接口幂等才可重试；写接口用幂等键。
- 容量：连接池 = 副本数 × pool；压测按峰值；扩容触发写进 SLO。

### 3.5 热更与自愈

- 配置：configcenter + 版本 + 校验 + 回滚；监听地址/DSN/Redis 地址/模块启停**不热更**。
- 能力：轻量 flag 运行期；重能力部署期。
- DB：expand-contract；禁止单步破坏。
- 事件/缓存/会话：`SchemaVer` / generation / 密钥重叠轮换。
- 多机收敛：广播 + version + 幂等 + 滞后告警。
- 快照自愈：订阅 handler 进程内退避重试（首发 + 3 次，100ms→300ms→900ms），穷尽后记 Error 并交周期对账；每节点每 5 分钟盲重载一次（1 分钟超时、2 次重试）——陈旧上界 5 分钟。

### 3.6 性能

- 先测量再优化：Profile / 压测 / 慢查询 / 慢 Redis。
- 预算：local < 1ms；RPC p99 < 10ms；DB p99 < 20ms；API p99 < 2000ms。
- 未测量不加缓存/索引/分片/队列。
- 大列表用游标分页；禁止无界扫描。

### 3.7 安全

- 认证/授权/限流/IPAC/审计默认启用；敏感操作二次授权；admin 登录带一次性验证码。
- 安全响应头基线恒发：`X-Content-Type-Options: nosniff` / `X-Frame-Options: DENY` / `Referrer-Policy: strict-origin-when-cross-origin`；CSP 默认 Report-Only（显式配置转 enforce）；HSTS 仅 `env=prod` 下发。
- CORS 只允许精确 Origin 白名单，禁止 `*` + 凭证组合。
- PII 加密 + 盲索引 + 脱敏；密钥不入代码；日志/导出禁明文。
- 外部调用：超时/重试/熔断/限流/签名/幂等。
- 服务间：服务 JWT + 端口级 AuthZ + 接收方 `allowed_callers` 白名单（空=全拒）；不信任内网。

### 3.8 日志与观测

- 运行/错误/访问日志 → stdout/文件/采集器；**不写业务 DB**。
- admin 操作日志 → DB `operation_logs`（审计）；不建 `error_logs`。
- 字段：`ts/level/module/type/msg/request_id/trace_id/node_id/source`；PII 脱敏。
- 每模块定义 SLO + 指标 + 看板/告警；trace 跨 HTTP/RPC/Bus/Job。

---

## 4. 如何落地（执行清单）

### 4.1 项目初始化

- 参考 Profile（lean/standard/full/custom）确定模块集合，但落地以 `config.yaml` 为准。
- 复制 `deploy/config.example.yaml` → `config.yaml`：模块段必须存在（空段 `{}` 即满足 `config_section` 校验）；未知段只告警不阻断。
- 配置 `deployment`（topology/role/node_id/service_name）、`binding`（RPC 本地/远程）、日志目录、审计保留、DB/Redis/OSS、SLO。
- 密钥只走环境变量；`prod` 环境缺 JWT 密钥文件=拒绝启动。
- 生成项目 `AGENTS.md`（§6），填项目参数与加严项。

### 4.2 新模块

```bash
node scripts/scaffold/new-module.mjs <name> --kind=business --owner=<your-team>
# 生成 internal/modules/<name>/：module.yaml、module.go（迁移/Init/AttachDB 骨架）、
# api/{keys,rpc,events}、domain/application/adapters、migrations/；生成物可直接过门禁
```

生成后必做（门禁会兜底检查）：

1. 登记 `contracts/namespaces.local.yaml`（下游）或基座 `contracts/namespaces.yaml`（仅基座内模块）——样例见 `contracts/namespaces.local.yaml`（note 模块）。
2. 登记迁移号段 `contracts/migrations.local.yaml`（项目首位数字 + 模块号）；基座内模块登记 `migrations.yaml`。
3. module.yaml 填 `depends_on/ports/events/jobs/tables/scaling.mode`；软删/大表按 [`migrations.md`](../rules/migrations.md) 声明。
4. 装配登记（三处，缺一迁移或路由不生效）：
   - `cmd/netyadmin/assembly.go` 的 `registerModules`：构造模块实例并 `reg("internal/modules/<name>", mod)`；
   - `cmd/netyadmin/migrate.go` 的 `moduleMigrations`：加一行 `{Module: "<name>", FS: <name>.Migrations()}`；
   - `cmd/netyadmin/main.go` 的 `adminapi-routes`（或对应接入面钩子）：经 `module.ResolveAs[*<name>app.Service](registry.Services, "<name>.Service")` 取端口后注册路由。
5. 跑门禁确认绿（门禁清单见 §8；参考实现随基座源码仓库维护方内部维护，下游按清单自建 CI）；contracts 变更后重新跑门禁确认。

> 参考样例：`note`（骨架登记样例，无业务实现）、`users`（业务模块 + 权限种子 + Port 消费的完整示范）；
> 生产级参考：`configcenter`（事件/hot reload/周期对账）、
> `audit`（DB 落库 + Sink 异步批量 + 模块 Job 经 scheduler）、`gw`（规则快照 + Bus 订阅热更 + HMAC 验签与真转发）。

**拆卸**：业务模块（`kind: business`）两步拆掉，项目照常编译运行：

```bash
rm -r internal/modules/<name>          # ① 删模块目录
# ② 删 cmd/netyadmin 装配清单中对应的三行（assembly / migrate / 路由注册）
```

前提由门禁强制：edges 不 import 业务模块（业务路由由组合根/RouteRegistrar 注册，跨模块调用经 Registry Resolve）——所以删目录不会牵连其他代码。
核心功能（平台服务/接入面）不拆，用模块开关：`config.yaml` 中对应段 `enabled: false`。

结构（与基座模块同构）：

```
internal/modules/<name>/
├── api/             # 端口、DTO、事件、错误码引用；keys/rpc/events 唯一封装
├── domain/          # 实体、值对象、聚合、仓储接口、领域错误
├── application/     # 用例、事务边界、编排
├── adapters/        # 仓储、RPC、外部系统
├── module.go        # 装配入口（Migrations/Init/AttachDB 惯例）
├── module.yaml
└── migrations/
```

Manifest 必填/常用字段：`name/kind/version/api_version/contract_version/min_kernel_version/owner/data_classification/slo/depends_on/ports/events/jobs/tables/config_section/scaling/deployment`。
Port 方法元数据：`idempotent/timeout/retryable/max_retries/idempotency_key`（`retryable: true` 必须 `idempotent: true` 或 `idempotency_key: required`）。
Hook 不入 Manifest——在 `module.go` 的 Init 中经 `Registry.Hooks`（生命周期段）或 transport 请求级 Hook 注册。
生命周期六段：`before_bootstrap / after_bootstrap / before_listen / after_listen / before_shutdown / after_shutdown`。
请求六段：`request_received`（引擎全局链、鉴权前）→ `request_authenticated`（组级 stage、AuthN 之后，Principal 已注入）→ `before_handler`（可短路）→ `after_handler`（log-only）→ `response_sent` / `request_error`（异步；后两段禁写业务 DB）。

### 4.3 接入面

- `adminapi` / `clientapi` / `gateway` 只做路由与中间件；业务在模块。
- 管理端功能必须带权限种子（菜单/API/按钮/角色），随迁移交付；`rbac_apis.act` 与路由权限码同值。
- 路由：`/admin/v1/<module>/...`、`/client/v1/<module>/...`、`/gateway/v1/...`；统一信封 + 分页 + 5 位状态码。
- WS 三面（admin/client/gateway）：
  - ticket 单次消费（Redis GETDEL 原子）；未配置 Redis=无 ticket 面，WS 拒绝（single 也需 Redis 才能启用 WS），逻辑 60s；过期桶 2×TTL——桶内过期返回 `18502`，超桶/已用/未发/跨面统一 `18501`；
  - 频道 `user:/admin:/app:/topic:`，订阅与下行两级 ACL；禁止字符串拼接；
  - `realtime` 配置面 = `enabled` + 心跳/空闲/发送队列/单身份连接/单 App 连接/节点总连接/单连接频道/单帧大小/admin Origin 白名单/单连接速率/单 App 速率/RequireHTTPS；`prod` 强制 `require_https`；`max_conns_total=0`=显式不限节点连接总数；
  - 慢消费者队列满断开（1013），禁止无限缓冲。
- 外部网关：
  - 判定管线：时间窗 → App 快照 → 状态 → IPAC → HMAC 验签 → nonce 防重放 → 配额 → scope→API 白名单 → 真转发；
  - 真转发 = 进程内反代到目标业务路由（客户端签名面是完整 `/gateway/v1/...` path；转发前重写为内部 path）；自环 `/gateway/*` 拒绝；
  - 失败码 18xxx 细分（认证 181xx / 授权 182xx / 限流 183xx / API 登记 184xx / WS 185xx）；`18501`=ticket 无效或已用，`18502`=ticket 过期；
  - 配额语义：`RateLimitEnabled=true` 时 `Rate=0` = 显式不限流；关闭配额用 `RateLimitEnabled=false`；
  - 未启用治理面不得开放入口；存储不可用=503 fail-closed。
- 内部 RPC：`/internal/v1/rpc` 同 engine 路径隔离；接收方按 `rpc.allowed_callers` 白名单（空=全拒）；调用方身份=服务 JWT。

### 4.4 能力选择

- 只引用 `contracts/capabilities.yaml` 已登记能力；新增能力先登记。
- 大能力做成独立模块（`cdn`、`gateway-observe`），不加布尔开关。
- 运行时由模块开关与部署角色决定装配；未启用模块：无路由/Job/goroutine/连接。
- `hot_local` 缓存只给白名单实体；安全/会话/协调/PII 永不 L1。

### 4.5 数据与状态

- 模块表前缀/归属；迁移自带；expand-contract。
- 缓存面名 `kernel.Cache`（Registry Expose；下游 `module.ResolveAs[*cache.Store]` 消费并注册自己的 entity）。
- 缓存只声明 entity class：`hot_local`（L1+L2，≤64KB，白名单）/ `shared`（L2，默认）/ `durable`（DB）；禁止手工选层。
- L1 常开；L2 多机必开；generation 失效；loader 写回校验 generation。
- `hot_local` 自动降级：命中率 <0.7（每 512 次 miss 评估）或滚动 10 分钟 QPS <100（观测满窗）→ 降为 `shared`；不自动升级；`EffectiveClass` 可观测。
- PII entity 写 L2 前 AES-256-GCM 加密（`classification: pii`）；密钥 env-only `NETYADMIN_CACHE_L2_KEY`，缺失=实体注册失败（fail-closed）；轮换后旧密文视为 miss 自然回填。
- 幂等：API 幂等键 + 事件消费去重 + 任务幂等。

### 4.6 日志/审计

- 运行日志：`logs/<YYYY-MM-DD>/<module>_<type>.log`；类型 `app/access/error/security/audit`；`debug` 是级别，默认关。
- 错误日志：文本文件（主）+ 采集器可选；**不写 DB**。
- admin 操作日志：DB `operation_logs`；字段/保留/导出按合规。
- 禁止 `error_logs` 表；禁止运行/错误/访问日志写业务 DB。

### 4.7 测试 / CI

- 单元：用例 + 内存适配器（不启动 DB/Redis）。
- 集成：本机 PG/Redis 直连；`TEST_DATABASE_DSN` 指向测试专用库；迁移全量跑通。
- 全量测试串行：`go test -race -p 1 ./...`（迁移版本水位共享，多包并行互删）。
- 契约：local/remote 双适配器；新旧版本 N-1。
- 架构：import 图、禁止跨模块 SQL、domain 禁框架、Manifest 校验。
- 性能：基准 + 压测；按你启用的模块组合（Profile 参考）做压测矩阵。
- 混沌：Redis/DB/leader 故障（按触发条件）。
- 门禁清单（下游按此自建 CI；参考实现随基座源码仓库维护方内部维护）：gofmt / go build / go vet / 生成物一致性 / 状态码 / 迁移号段 / 权限码对偶 / 命名空间 / Manifest / import 图 / 裸字面量 / Redis key / 契约镜像 / 缓存准入 / 依赖预算 / 事件目录 / OpenAPI。
- 发布仓库本身不承诺 CI：清单面向**你自己的工程**——把同组检查接进你的 CI；号段与 `*.local.yaml` 扫描口径按你的项目调整。

---

## 5. 禁止事项（AI 必须拒绝）

- 修改 kernel / 平台服务 / 接入面内部代码。
- 跨模块 import `internal/repository/entity`；跨模块 JOIN / 事务 / 外键。
- 手写状态码 / Redis key / RPC 路由 / WS 频道 / 事件 topic。
- 运行/错误/访问日志写 DB；建 `error_logs` 表。
- 安全/会话/协调/PII 数据进 L1；多机本地回退。
- 布尔开关矩阵；未测量加缓存/索引/分片/队列。
- 运行时热补丁；单步破坏迁移；全量清缓存。
- 密码/密钥入代码；明文 PII；无界 goroutine/队列。
- 把 `Rate=0` 当“关闭配额”（它是不限流；关闭用 `RateLimitEnabled=false`）。
- 微服务 / Service Mesh / 多地域双活 / Event Sourcing / CQRS / GraphQL / 动态插件 / 内置搜索 / 数仓（V2.0 非目标）。

---

## 6. 项目级 AGENTS.md 模板

```md
# AGENTS.md — <Project>

## 0. 基座
- 基座：NetyAdmin V2 <version>
- 必读：基座 docs/rules/README.md + contracts/
- 本项目规则只加严基座规则，不复制基座正文。

## 1. 项目
- project id: <id>
- 业务模块：<list>
- 模块开关：users=<on/off> message=<on/off> storage=<on/off> realtime=<on/off>
- 能力：<contracts/capabilities.yaml IDs>
- 拓扑：<single|multi>；角色：<all|core|admin>；node_id：<id>（multi 每副本唯一）
- 范围：无多租户；C 端用户按本项目模型替换基座示例。

## 2. 数据
- DB：<host/db>（密钥走环境变量）
- Redis：<host>（multi 必开 L2）
- 模块表归属：<module -> tables>
- 审计表：operation_logs；保留 <N> 天；无 error_logs。
- 缓存 entity：<name -> class>；PII 实体加密密钥 env-only。

## 3. 接口
- admin：/admin/v1/<module>/...
- client：/client/v1/<module>/...
- gateway：/gateway/v1/...
- WS：ticket + 频道 ACL + 常量唯一；realtime 限制按默认基线。
- 权限种子：rbac_apis 与路由权限码同值，随迁移交付。

## 4. SLO / 容量
- 每副本 QPS：<n>
- p99：<read/write>
- 内存 / 连接：<n>
- 扩容触发：CPU>70% / p99>SLO / 队列深度。

## 5. 日志
- 运行/错误/访问：logs/<date>/<module>_<type>.log
- admin 操作日志：DB operation_logs
- PII 脱敏；禁止运行/错误日志入 DB。

## 6. 多机 / 热更
- scaling.mode；无状态默认；本地回退仅 single。
- 滚动 N-1；expand-contract；配置版本化；快照 5 分钟对账。

## 7. 项目加严
- <项目特定规则；只加严，不放宽>

## 8. 禁止
- 修改基座；跨模块 internal；跨模块 JOIN/TX；手写状态码/keys；
- 运行/错误日志入 DB；error_logs 表；布尔开关矩阵；未测量优化。
```

---

## 7. AI 提示词模板（直接复制）

```text
你是 NetyAdmin V2 基座上的下游项目开发助手。
输入：
- 基座规则：本文档 + 基座 docs/rules/README.md + contracts/
- 项目参数：<按 §0.1 填>

任务：
1) 生成项目 AGENTS.md（用 §6 模板；只写项目参数与加严项）。
2) 从 deploy/config.example.yaml 生成 config.yaml：模块段/开关、deployment、binding、日志/审计。
3) 生成模块清单、Manifest、骨架（api/domain/application/adapters）。
4) 先契约后代码：Port/Event/Job/Hook/Migration/Config/权限种子。
5) 完成三处装配登记（assembly / migrate / 路由钩子）。
6) 输出变更清单 + §8 检查清单。

约束：
- 不修改基座；不实现未列出的模块/能力；不新增依赖（除用户批准）。
- 每个新功能先答：谁在用/何时用/不用会怎样/最简方案/何时删除。
- 缺失参数先问，不要猜。
```

---

## 8. 交付检查清单

> 每项都写明“怎么验”。门禁命令是参考实现；你的工程接 CI 后按同一检查项落地。

- [ ] 项目 `AGENTS.md` 已生成，引用基座规则，只加严。
- [ ] `config.yaml` 的模块段/开关、`deployment`（topology/role/node_id/service_name）、`binding`、日志/审计完整；密钥仅环境变量。
- [ ] 新模块 Manifest 必填字段完整；`scaling.mode` 显式；`ports` 方法元数据可重试语义自洽（验：`node scripts/ci/check-manifests.mjs`）。
- [ ] 迁移：5 位号段/段归属、每表 schema+seed、up/down 成对、schema 无 INSERT、seed 无 DDL（验：`node scripts/ci/check-migrations.mjs`，下游改用自己的号段扫描口径）。
- [ ] 迁移源与模块实例已登记：assembly / migrate / 路由钩子三处齐（验：`go run ./cmd/netyadmin -mode=migrate -migrate-action=status` 能看到模块源；启动日志有模块装配）。
- [ ] 管理端权限种子随迁移交付；路由权限码 ↔ `rbac_apis` 双向一致（验：`node scripts/ci/check-rbac-perms.mjs`，下游改用自己的 seed 号段）。
- [ ] 数据边界：无跨模块 JOIN/TX/FK；表归属清晰（验：`check-migrations` + import 图）。
- [ ] 缓存 entity class 声明；`hot_local` 白名单 + generation；PII 加密密钥缺失时 fail-closed（验：`check-cache-admission`）。
- [ ] 日志：运行/错误/访问写文件；admin 操作日志写 DB `operation_logs`；无 `error_logs`。
- [ ] 多机：`scaling.mode`、无状态、readiness/drain、幂等；`node_id` 每副本唯一；RPC `allowed_callers` 已登记（空=全拒）。
- [ ] 热更：配置版本、N-1、expand-contract、generation；快照消费具备订阅侧重试 + 5 分钟对账。
- [ ] 安全：认证/授权/限流/审计/PII/密钥；安全响应头基线（CSP 默认 Report-Only、HSTS 仅 prod）；CORS 精确白名单。
- [ ] 网关：scope→API 白名单、HMAC、nonce、配额、`Rate=0` 语义、18xxx 细分码（含 18501502）。
- [ ] WS：三面 ticket 单次 60s；频道 ACL 两级；`realtime` 限制全部走配置（含单 App/节点总连接与速率）；`prod` 强制 HTTPS。
- [ ] 观测：日志/指标/trace/request_id/SLO。
- [ ] 测试：单元/集成/契约/架构；`go test -race -p 1 ./...`；测试库隔离。
- [ ] 反过度设计：无未使用抽象/配置/依赖；有删除条件。
- [ ] 门禁：字面量扫描、import 图、迁移校验、Manifest、权限码对偶、缓存准入全部通过。

---

## 9. 基座文档索引（只读）

- [`docs/guide/README.md`](README.md)：下游文档入口（按任务导航 + 排障速查）
- [`docs/guide/downstream-development.md`](downstream-development.md)：建模块/用基座能力/HTTP/网关/WS 操作手册
- [`docs/guide/tutorial-first-module.md`](tutorial-first-module.md)：从零做一个完整业务模块的分步教程
- [`docs/guide/gateway-canonical.md`](gateway-canonical.md)：网关 HMAC 签名契约（任意语言实现）
- [`docs/rules/README.md`](../rules/README.md)：全部规则单入口（状态码/命名/WS/能力/状态缓存/多机/热更/反过度设计/日志/迁移/API 风格/SLO/可靠性/安全/可观测/数据生命周期/版本发布/AI 纪律/测试）
- [`contracts/`](../../contracts/)：status-codes / namespaces / capabilities / profiles / state-policies / migrations / openapi
- 设计约束与非目标：本文档 §1 基座边界 + README「适用项目类型」
- 脚手架与工具：[`scripts/scaffold/`](../../scripts/scaffold/)（模块生成）、[`scripts/gen/`](../../scripts/gen/)（契约生成）

> 下游只读基座文档；项目自己的规则/决策放在自己的仓库，不修改基座。

适用版本 v2.0.0
