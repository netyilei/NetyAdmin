# 多机热更规则（V2）

> 热更 = 配置/能力/数据/契约/代码在不中断服务的前提下更新。
> **Go 不支持安全运行时热补丁：代码热更 = 滚动发布。**
> 关联：多机规则 `multi-node-scaling.md`。

## 1. 分类

| 类型 | 机制 | 是否重启 |
|------|------|---------|
| 配置 | configcenter + 事件 | 否（部分需重启） |
| 能力/特性 | 部署期开关/build tag；轻量 flag | 部署期需滚动；轻量 flag 否 |
| 代码 | 滚动/蓝绿/金丝雀 | 是 |
| DB schema | expand-contract | 否（在线 DDL） |
| 事件/API | 版本化 + 双发 | 否 |
| 缓存/会话 | generation / 密钥轮换 | 否 |

## 2. 配置热更

- 单一事实源 configcenter；版本号；schema 校验。
- 变更 → `platform.config.changed{version}`；各节点原子替换；保留上一版回滚。
- 无效配置拒绝，保留旧值并告警。
- **不可热更**：监听地址、DB DSN、Redis 地址/拓扑、模块启停、build tag 能力 —— 必须滚动重启。
- **可热更示例**：日志级别、任务暂停态（taskcenter 组）、SMTP 发送参数（message 组——admin「邮件设置」写入，各节点重建驱动；密码仍 env-only）。
- 收敛：各节点上报 applied version；滞后超阈值告警。
- 幂等应用；无部分状态；日志/指标带 version。

## 3. 能力/特性

- 轻量 flag：运行时切换（采样、阈值、灰度），不改变资源拓扑。
- 重能力/模块：部署期决定；滚动重启。
- 关闭能力：零路由/Job/goroutine/连接。
- flag 变更审计；禁止 flag 控制连接池/监听器。

## 4. 代码发布（滚动）

- 禁止 Go 动态插件/运行时补丁。
- 蓝绿/金丝雀；`maxUnavailable`/PDB。
- readiness/liveness；drain；WS 1001；scheduler 让主；log flush。
- N-1 契约兼容：旧/新 Pod 共存。
- API 只加字段；事件 schema 只加；RPC 方法元数据。
- 回滚：上一镜像 + 上一配置；迁移必须向后兼容。

## 5. DB schema

- expand-contract：加列 → 双写/回填 → 切读 → 删旧。
- 禁止单步破坏；在线 DDL；短锁。
- 模块自带迁移；版本化；幂等。
- 回填任务限流、幂等、可恢复。

## 6. 事件/缓存/会话

- 事件：`SchemaVer`；只加；双发窗口；消费者忽略未知字段。
- 缓存：generation 失效；禁止全量 flush；key 版本双读；TTL。
- 会话：JWT 密钥轮换重叠；token version；refresh 轮换。
- 命名空间：新 key + 双读；禁止原地改格式。

## 7. 多机一致性

- 配置/事件/缓存变更广播；各节点独立应用；最终收敛。
- 不假设同步；消息带 version/epoch；幂等；忽略旧。
- 收敛滞后告警。

## 8. 禁止

- 运行时热补丁。
- 未校验/无回滚的配置变更。
- 单步破坏迁移。
- 全量清缓存。
- 破坏式 API/事件变更无版本。
- 各节点配置漂移。

## 9. Checklist

- [ ] 配置版本/校验/回滚。
- [ ] flag 轻量，不控制资源拓扑。
- [ ] 滚动 N-1 测试。
- [ ] expand-contract 迁移。
- [ ] 缓存 generation。
- [ ] 收敛指标与告警。
