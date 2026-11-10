package recovery

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestGlobalRecovery_Repanics 锁定退出契约：GlobalRecovery 只做「打印 + 重新 panic」，
// 绝不吞掉 panic——历史实现打印后正常返回使 main exit 0，启动期 panic 表现为
// 「成功退出」，supervisor（systemd/K8s）不重启、监控无失败信号。
// 本测试若失败（外层 recover 到 nil），说明 panic 被吞，退出契约被破坏。
func TestGlobalRecovery_Repanics(t *testing.T) {
	defer func() {
		r := recover()
		assert.NotNil(t, r, "GlobalRecovery 不得吞掉 panic：必须重新 panic 使进程非零退出")
		if s, ok := r.(string); ok {
			assert.Equal(t, "boom", s, "重抛的必须是原始 panic 值")
		}
	}()
	defer GlobalRecovery()
	panic("boom")
}

// TestGlobalRecovery_NoPanicNoop 无 panic 时必须是 no-op（不产生虚假崩溃）。
func TestGlobalRecovery_NoPanicNoop(t *testing.T) {
	assert.NotPanics(t, func() {
		defer GlobalRecovery()
	})
}
