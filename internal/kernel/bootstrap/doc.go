// Package bootstrap 提供进程生命周期引擎：6 段（BeforeBootstrap/AfterBootstrap/
// BeforeListen/AfterListen/BeforeShutdown/AfterShutdown）+ 优雅关闭。
//
// 错误策略（§7.2）：启动段 fail-fast 首错中止；排空段（BeforeShutdown 5s 预算）
// 单钩子失败/超时不跳过其余清理动作；钩子 panic 隔离按阶段策略。
// 钩子：优先级降序 + 注册序（On 较 §7.5 多 name 参数，诊断用； 统一注册表时收敛）。
// ServeFunc 为阻塞契约（与 http.Server.Serve 一致；监听器创建放 BeforeListen 段）。
//
// 边界：配置加载（产出）在 BeforeBootstrap 前由调用方完成，本包不 import config；
// 信号监听（signal.NotifyContext→ctx）在 cmd 接线——本包只认 ctx，纯可测。
package bootstrap
