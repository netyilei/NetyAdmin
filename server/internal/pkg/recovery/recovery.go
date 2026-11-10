package recovery

import (
	"fmt"
	"runtime/debug"
)

// GlobalRecovery 是进程级兜底 panic 恢复函数，仅在 cmd/server/main.go 顶层
// 通过 `defer recovery.GlobalRecovery()` 调用，作为整个进程崩溃前的最后一道防线。
//
// 与 GoSafe 的区别：
//   - GoSafe 用于异步 goroutine 的 per-goroutine panic 恢复，附带 slog + Sentry 上报；
//   - GlobalRecovery 用于主 goroutine 兜底，使用 fmt.Printf 直接打印（此时进程即将退出，
//     结构化日志与 Sentry 上报可能因 flush 时序问题而丢失，直接打印到 stderr 最稳妥）。
//
// 退出契约：recover 后打印堆栈并**重新 panic**——进程以非零码退出（runtime 默认 2）。
// 历史缺陷：打印后吞掉 panic 使 main 正常 return（exit 0），启动期 panic
// （gin 路由冲突、装配失败等）表现为「成功退出」——systemd Restart=on-failure /
// K8s OnFailure 不会重启，监控读不到任何失败信号。崩溃必须表现为崩溃。
//
// 注意：本函数不再被推荐用于 goroutine 级恢复 —— 异步 goroutine 应统一使用 GoSafe。
func GlobalRecovery() {
	if r := recover(); r != nil {
		stack := debug.Stack()
		fmt.Printf("[CRITICAL] 全局 panic 恢复: %v\n", r)
		fmt.Printf("[CRITICAL] 堆栈信息:\n%s\n", string(stack))
		// 重新 panic：非零退出，supervisor 才能识别失败并重启。
		// 不用 os.Exit(2)：panic 展开保证 main 中其余 defer 语义完整。
		panic(r)
	}
}
