// Package hooks 实现统一钩子注册表：lifecycle/request/ws/job 四类 19 阶段
// （元数据表 hooks.go stageMetas：错误策略 fail-fast 首错中止 vs aggregate
// 聚合继续、默认超时 2s·排空段 5s、async 阶段标记）。
// Registry：On（未登记阶段拒绝，防拼写静默）/Unregister/RunStage（同步链，
// async 阶段强制转异步）/RunStageAsync（WithoutCancel detach，fire-and-forget）/
// Hooks 诊断面。优先级降序+注册序；per-hook 超时无"不限时"语义；panic 隔离。
// 分工：Hook 同步可短路可改变结果；Bus 异步分发不关心结果。
// bootstrap.App 转发本表驱动 lifecycle 段；消费端：TR-05 request hooks。
package hooks
