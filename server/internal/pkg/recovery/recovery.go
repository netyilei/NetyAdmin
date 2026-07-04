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
// 注意：本函数不再被推荐用于 goroutine 级恢复 —— 异步 goroutine 应统一使用 GoSafe。
func GlobalRecovery() {
	if r := recover(); r != nil {
		stack := debug.Stack()
		fmt.Printf("[CRITICAL] 全局 panic 恢复: %v\n", r)
		fmt.Printf("[CRITICAL] 堆栈信息:\n%s\n", string(stack))
	}
}
