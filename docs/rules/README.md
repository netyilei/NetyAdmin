# 规则与约束（V2）

本目录是 V2 的"命名与治理规则"单一入口。所有模块、下游项目、AI 协作者都必须遵守。

| 文档 | 内容 |
|------|------|
| [状态码规范](status-codes.md) | 5 位状态码分配、核心码、HTTP 映射、生成与校验 |
| [命名与命名空间规范](namespaces.md) | Redis/RPC/WS/事件/配置/指标/存储等命名，单一事实源与 CI |
| [WebSocket 协议规范](ws-protocol.md) | 三端入口、ticket、频道、信封、关闭码、限制、多节点 |
| [能力选择规范](capabilities.md) | 能力/Profile/可选集成、禁用语义、性能保证、scaffold |
| [状态子系统：缓存与降级](state-and-cache.md) | L1/L2/DB、三平面、generation 失效、降级矩阵、可观测 |
| [多机部署与水平扩展](multi-node-scaling.md) | 伸缩模式、状态外置、数据边界、容量、滚动发布 |
| [多机热更](hot-update.md) | 配置/能力/代码/schema/事件/缓存热更、N-1、收敛 |
| [反过度设计](anti-over-design.md) | 五道闸门、复杂度预算、禁止清单、触发式建设 |
| [日志规范](logging.md) | 运行/错误日志不写 DB、admin 操作日志入 DB、slog+lumberjack、路径/类型/开关 |
| [数据库迁移规范](migrations.md) | 5 位号段、schema/seed 分离、无补丁、下游 2xxxx+ |
| [AI 编码纪律](ai-coding-discipline.md) | AI 红线清单（lint/CI 强制）+ 唯一写法清单 + 增量验证 + 工作循环 |
| [测试规范](testing.md) | 分层门禁/防假绿纪律/契约与集成/CI |
| [API 风格](api-style.md) | 版本/路由/响应信封/分页/幂等/swag 注解生成/oasdiff 破坏性门禁 |
| [SLO 与性能预算](slo.md) | 全局预算表/可用性目标/测量口径/压测纪律/基准工具 |
| [可靠性基线](reliability.md) | 超时/重试/熔断/幂等/恢复约定与反模式 |
| [安全基线](security.md) | 密钥/认证授权/PII/输入上传/传输基线与红线 |
| [可观测契约](observability.md) | 记录边界/日志 schema/指标与追踪契约/健康诊断/告警最小集 |
| [数据生命周期](data.md) | 备份恢复/保留清理/软删规范/删除导出/校验 |
| [版本与发布流程](release.md) | 六版本轴/发布流程/弃用三步/下游升级/分支提交 |

## 三条总原则

1. **单一事实源**：每个命名空间只有一个 owner、一份定义文件；使用方必须引用，禁止第二份。
2. **禁止字面量**：状态码、Redis key、RPC 路由、WS 频道、事件 topic、配置 key、指标名，业务代码中不得出现原始字符串。
3. **契约先行**：新增码/前缀/频道/事件，先登记 `contracts/`，再生成常量，最后写业务。

## 登记文件

| 文件 | 用途 |
|------|------|
| [`contracts/status-codes.yaml`](../../contracts/status-codes.yaml) | 状态码单一事实源 |
| [`contracts/namespaces.yaml`](../../contracts/namespaces.yaml) | 模块前缀/owner/keys 包登记 |
| [`contracts/capabilities.yaml`](../../contracts/capabilities.yaml) | 能力目录（ID/provider/requires/ports/cost） |
| [`contracts/profiles.yaml`](../../contracts/profiles.yaml) | lean/standard/full/custom 预设 |
| [`contracts/state-policies.yaml`](../../contracts/state-policies.yaml) | 缓存/协调/会话/总线降级策略 |
| [`contracts/migrations.yaml`](../../contracts/migrations.yaml) | 迁移号段/模块分配 |

## 新增一个状态码

1. 在 `contracts/status-codes.yaml` 选一个未占用码（遵守范围与 owner）。
2. 填写 `name/surface/domain/http/retryable/message_key/owner/since`。
3. 运行生成器：`go generate ./...`（errorx 包内指令）或 `node scripts/gen/status-codes.mjs`，产出 Go/TS 常量与索引文档入库。
4. 在代码中使用生成常量；禁止手写数字。
5. CI 校验：唯一性、范围、无字面量。

## 新增一个 key / 频道 / 事件

1. 确认所属模块与 feature 段。
2. 在 owner 模块 `api/keys`（或 `api/channels`、`api/events`、`api/rpc`）中新增 builder/常量。
3. 更新 `contracts/namespaces.yaml`（若新增模块/前缀）。
4. 使用方 import owner 包；禁止复制格式。
5. 运行 key 快照测试与 CI 字面量扫描。

## CI 门禁

- 状态码：唯一、范围归属、无越界、无未登记、无代码字面量。
- OpenAPI：swag 注解生成入库（contracts/openapi/），oasdiff 破坏性变更拦截（起，spec 齐备后生效）。
- 命名空间：前缀唯一、模块已登记、无重复 builder、无裸 key 字符串。
- WS：无裸频道/消息类型/关闭码；协议常量唯一。
- 契约测试：local/remote 双适配器；新旧版本兼容。
- 迁移：5 位号段/段归属/schema-seed 分离/无补丁后缀/编号唯一/模块登记。
- 下游扩展：只在 `contracts/*.local.yaml` 登记自己的状态码/前缀/能力；禁止修改基座 `contracts/`。

## 规则门禁（强制执行）

- `.zcode/config.json` 注册 `SessionStart / UserPromptSubmit / PreToolUse / PostToolUse / Stop`。
- 首轮必须 `RULE-ACK <hash>` 并运行 `node scripts/rules/ack.mjs <hash>`；未 ACK 的工具调用被 PreToolUse 拒绝。
- 规则文件（AGENTS.md / docs/rules / docs/roadmap / contracts）变更 → hash 变化 → 必须重新 ACK。
- 写后扫描手写状态码 / Redis key / WS 频道 / 跨模块 internal；Stop 阻断硬违规。
- 脚本：`scripts/rules/`。
