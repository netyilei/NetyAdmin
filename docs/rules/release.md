# 版本与发布流程（release）

> 兼容规则的技术细节见 hot-update.md 与；本文件定义版本轴、发布流程与下游升级约定（对应补齐清单「版本/弃用/发布流程」P0 项）。

## 1. 版本轴（六轴并行，各自独立演进）

| 轴 | 载体 | 兼容规则 |
|----|------|---------|
| Kernel | 语义化版本（tag `v{semver}`） | 模块声明 `min_kernel_version` |
| Module API | `api_version`（Manifest） | minor 只加不删；major 允许破坏 |
| Internal RPC | `/internal/v1/rpc/...` | N-1：新服务处理旧客户端 |
| Event Schema | `SchemaVer` | 只加字段；语义变更升版本 + 双发窗口 |
| OpenAPI | 路径版本 `/admin/v1` | 破坏式变更升路径版本或 major（+ api-style §1） |
| DB Schema | 模块迁移号段 | expand-contract，禁单步破坏（migrations.md） |

## 2. 发布流程（每次对外发布）

1. CI 全绿（scripts/ci 全部门禁）+ `go test -race ./...` 通过。
2. 安全门：govulncheck + gosec + SBOM（tech-stack §8）；许可证扫描通过。
3. 基准抽测：`node scripts/bench/run.mjs`（阈值断言过线，slo.md §4 纪律）。
4. 更新 `CHANGELOG.md`：按里程碑/语义化版本记录，含**破坏性变更清单**与升级动作。
5. 打 tag `v{semver}`；tag 即契约快照（contracts/openapi 与 contracts/*.yaml 随 tag 冻结）。
6. 发布物：编译产物 + 迁移说明（goose up/down 顺序）+ 升级指南章节链接。

## 3. 弃用策略（三步，不可跳步）

公告（CHANGELOG + 诊断端点标记 deprecated）→ 双发窗口（新旧并存至少一个 minor 周期，初始基线）→ major 删除。禁止在无公告的情况下直接移除任何已登记契约（状态码/端口/事件/key/端点）。

## 4. 下游升级约定

- 下游随基座升级 = 滚动发布 + N-1 兼容窗口；多机配置热更边界见 hot-update.md（监听地址/DSN/Redis/模块启停不热更）。
- 基座升级指南必须包含：迁移执行顺序、密钥重叠操作、回滚步骤。
- 下游不得修改基座 `contracts/`；自己的登记走 `contracts/*.local.yaml`。

## 5. 分支与提交（当前单仓库、无 remote 阶段）

- 主干 `main`；功能短命分支或直接主干提交均可，但**每次提交前门禁必须绿**（node scripts/ci/run.mjs）。
- 提交说明写目的而非罗列改动（G1-14）；规则文件变更后重新 ACK（AGENTS §0.1）。
- 远程仓库建立后：禁止直接 push main（全局红线），走 PR + CI 门禁。
