package recovery

import (
	"fmt"
	"runtime/debug"
)

func GlobalRecovery() {
	if r := recover(); r != nil {
		stack := debug.Stack()
		fmt.Printf("[CRITICAL] 全局 panic 恢复: %v\n", r)
		fmt.Printf("[CRITICAL] 堆栈信息:\n%s\n", string(stack))
	}
}

func SafeGo(fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				stack := debug.Stack()
				fmt.Printf("[GOROUTINE] panic 恢复: %v\n", r)
				fmt.Printf("[GOROUTINE] 堆栈信息:\n%s\n", string(stack))
			}
		}()
		fn()
	}()
}

func SafeGoWithRecovery(fn func(), onPanic func(any)) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				stack := debug.Stack()
				fmt.Printf("[GOROUTINE] panic 恢复: %v\n", r)
				fmt.Printf("[GOROUTINE] 堆栈信息:\n%s\n", string(stack))
				if onPanic != nil {
					onPanic(r)
				}
			}
		}()
		fn()
	}()
}
