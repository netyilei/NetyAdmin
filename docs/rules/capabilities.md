# 能力选择与可选集成规范（V2）

> 版本：v1.0 · 2026-11-11
> 目标：项目/模块创建时可选择集成哪些能力；不用的能力零运行时开销；依赖显式、fail-fast、可诊断；下游可扩展。
> 单一事实源：`contracts/capabilities.yaml`（能力目录）与 `contracts/profiles.yaml`（预设组合）。

---

## 1. 为什么不用"布尔开关"

如果一个模块里塞 `enable_gateway`、`enable_cdn`、`enable_monitor` 等布尔开关，会迅速变成 flag 泥潭：依赖关系不清、组合爆炸、关闭不彻底、性能收益无法验证。

V2 用**能力（Capability）**建模：

- 能力 = 可独立选择、可验证、有 owner、有端口、有成本声明的功能单元。
- 大能力做成独立模块（如 `cdn`、`gateway-observe`）；小能力作为模块内的 capability（如 `gateway.ws`）。
- 选择能力 = 启用对应模块/特性 + 解析依赖 + 绑定端口；关闭 = 不启动、不注册、不链接重适配器。

---

## 2. 概念

| 概念 | 定义 |
|------|------|
| Module | 可独立部署的功能单元（Manifest 声明） |
| Capability | 可选择的能力（ID、provider、requires、ports、cost、default） |
| Profile | 能力/模块的预设组合（lean/standard/full/custom） |
| Feature | 模块内与模块同生命周期的子能力（仅少量使用） |
| Binding | 端口绑定：`local`（同进程）或 `remote`（内部 RPC） |
| Required | 必需依赖：缺失则启动失败 |
| Optional | 可选依赖：缺失则使用 no-op 适配器 |

---

## 3. 选择层级

| 层级 | 机制 | 示例 |
|------|------|------|
| 项目级 | profile + modules enabled/disabled | `profile: lean` |
| 模块级 | `depends_on` / `optional_depends_on` | storage 可选依赖 cdn |
| 能力级 | `capabilities` 列表 | `gateway.observe`、`cdn` |
| 特性级 | 模块 `features`（少量） | `gateway.features.ws=false` |

> 优先用"能力"表达选择；`features` 只用于同一生命周期内的轻量子功能，避免把大能力塞进布尔开关。

---

## 4. 能力目录

`contracts/capabilities.yaml` 字段：

```yaml
capabilities:
  gateway.observe:
    provider: gateway
    kind: observability          # platform | observability | transport | integration
    requires: [gateway]          # 模块或能力 ID
    ports: [gateway.AccessStats] # 提供哪些端口
    default: true
    cost: { goroutines: 0, connections: 0, memory_mb: 0 }
    build_tags: []               # 需要编译期排除时使用
    since: "2.0.0"
```

要求：

- 每个能力必须有 provider、requires、ports、default、cost。
- 能力 ID 全局唯一；`kind` 决定它在架构中的位置。
- `cost` 用于 `netyadmin capabilities plan` 估算资源；0 表示无额外常驻开销。
- `build_tags` 用于重适配器（Kafka/CDN SDK/某些云 SDK）的编译期排除。

---

## 5. 预设 Profile

`contracts/profiles.yaml`：

```yaml
profiles:
  lean:
    modules: [identity, rbac, configcenter, audit, adminapi]
    capabilities: []
  standard:
    modules: [identity, rbac, configcenter, audit, ipac, storage, message, gw, scheduler, adminapi, clientapi, ops]
    capabilities: [gateway.control, gateway.observe, gw.stats]
  full:
    modules: [identity, rbac, configcenter, audit, ipac, storage, message, users, realtime, gw, scheduler, adminapi, clientapi, ops]
    capabilities: [gateway.control, gateway.observe, gateway.ws, cdn, storage.cdn, gw.stats]
  custom:
    modules: []
    capabilities: []
```

- Profile 只是默认值；`deployment.yaml` 可覆盖。
- 下游可定义自己的 profile，但必须引用已登记的能力 ID。

---

## 6. 创建时选择（Scaffold；CLI 后置）

```bash
# 未来 CLI 示例；MVP 用模板/脚本
# 新建项目
netyadmin new myservice --profile lean \
  --with gateway.control,gateway.observe,cdn

# 新建模块
netyadmin module new order \
  --requires identity,storage \
  --optional cdn,gateway.observe
```

生成：

- `deployment.yaml`（modules/capabilities/bindings）
- 模块骨架（api/domain/application/adapters）
- Manifest（含 `capabilities` 与 `optional_depends_on`）
- 迁移目录、契约测试、no-op 适配器（可选依赖）
- 文档片段（能力清单、资源估算）

---

## 7. 运行时解析

启动顺序：

1. 加载 profile + deployment 覆盖。
2. 校验：所有 enabled 模块/能力存在；required 依赖满足；版本兼容。
3. 拓扑排序初始化；`local` 绑定实现，`remote` 绑定 RPC 客户端。
4. 可选依赖：provider 未启用 → 绑定 no-op 适配器（并记录诊断）。
5. 启动失败：输出完整依赖链（缺哪个模块/能力/端口/版本）。

诊断端点输出：

```json
{
  "profile": "lean",
  "modules": [{"name": "identity", "mode": "local"}],
  "capabilities": [{"id": "gateway.observe", "enabled": true, "provider": "gateway"}],
  "noop_ports": ["cdn.Purge", "cdn.SignURL"]
}
```

规则：

- **Required 缺失必须 fail-fast**，不得静默 no-op。
- Optional 缺失用 no-op，但必须在诊断中显示，且首次调用记录一次 WARN。
- 关闭的模块不得注册路由/Job/Hook/监听/goroutine/连接。
- 关闭的能力不得初始化其适配器与客户端。

---

## 8. 禁用语义矩阵

| 层级 | 机制 | 行为 | 适用 |
|------|------|------|------|
| 编译期 | build tag | 代码不编译；API 不存在 | 重适配器（Kafka/CDN SDK/云厂商 SDK） |
| 部署期 | 模块/能力未启用 | 不初始化、不注册、不监听 | 绝大多数能力 |
| 展示面 | 模块未启用 | 菜单按 `rbac_menus.module` 过滤（RouteTree 服务端过滤；空=常显） | 状态/观测/消息/存储/用户页面 |
| 运行期 | feature flag | 仅切换轻量行为，不释放已初始化资源 | 灰度、降级、实验 |

规则：

- 重资源（连接池、监听、后台 goroutine）必须在部署期决定；运行期 flag 不得用于开关它们。
- 运行期 flag 变更为轻量路径（如采样率、消息大小限制），可热更新。
- 三种层级不能混用语义；文档必须标明能力属于哪一层。

---

## 9. 性能保证

- 禁用模块/能力：0 goroutine、0 连接、0 监听、0 定时器。
- 重适配器：build tag 排除，默认二进制不链接。
- 可选客户端：懒初始化；首次调用才建连接。
- 启动自检：诊断端点列出 active/noop/disabled；CI 断言 lean profile 的 goroutine/连接基线。

---

## 10. 测试

- **Profile 矩阵**：CI 至少构建并跑 smoke：`lean`、`standard`、`full`。
- **能力组合**：关键能力做 pairwise 组合测试（如 gateway.control × cdn）。
- **no-op 适配器**：可选依赖缺失时，业务路径返回定义好的降级结果。
- **诊断测试**：启用/禁用/远程绑定三种状态的输出。
- **性能基线**：lean profile 启动后无多余 goroutine/连接。

---

## 11. 治理

- 能力必须登记在 `contracts/capabilities.yaml`；未登记不得在代码中使用。
- 每个能力有 owner、since、deprecation 策略。
- Manifest 只能引用已登记的能力 ID；CI 校验。
- 能力变更（语义/默认值/成本）走契约评审。
- 文档从目录生成，避免手写漂移。

---

## 12. 示例：网关与 CDN

### 网关

| 能力 | 说明 | 默认 |
|------|------|------|
| `gateway.control` | App/Scope/API/配额治理 | true（启用 gateway 时） |
| `gateway.observe` | 访问日志、指标、追踪 | true |
| `gateway.ws` | WebSocket 入口 | true（启用 realtime 时） |
| `gateway.quota` | 配额/限流策略 | true |

- 业务模块需要网关用量统计：`optional_depends_on: [gateway.observe]`；未启用时返回空统计。
- 不启用 `gateway.control` 时，`gateway` 入口不应启用（依赖 fail-fast）。

### CDN

| 能力 | 说明 | 默认 |
|------|------|------|
| `cdn` | Purge/Preload/SignURL/InvalidateTag | false |
| `storage.cdn` | storage 上传后自动 purge | false |
| `content.cdn` | 文章发布后 purge 关联 URL | false |

- `cdn` 是独立可选模块；storage/content 通过 `optional_depends_on` 集成。
- 未启用时：返回源站 URL，不执行 purge；诊断显示 noop 端口。
- 重适配器（Cloudflare/阿里云/腾讯云 SDK）用 build tag 排除。

---

## 13. 反模式

- ❌ 一个模块里 20 个布尔开关。
- ❌ 把 required 依赖做成 optional 的 no-op（会导致"看似可用、实际不工作"）。
- ❌ 运行期 flag 控制连接池/监听器。
- ❌ 能力没有 owner/ports/cost/登记。
- ❌ 关闭模块后仍有 goroutine/路由/定时器残留。
- ❌ 下游私自定义能力 ID 而不登记。
