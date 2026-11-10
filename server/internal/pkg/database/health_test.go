package database

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// TestNewHealthChecker_NilOptionSkipped 锁定 nil 选项容错：vararg 中传 nil 函数值
// （调用方用 nil 占位表达"无选项"的笔误）必须被跳过而非 panic。
// 缺陷背景：wire.go 曾在 Redis 关闭时向 NewHealthChecker 传入 nil 选项，
// Bootstrap 在 [redis].enabled=false 时必崩——Redis「可选」承诺被破坏（2026-11-11 修复）。
func TestNewHealthChecker_NilOptionSkipped(t *testing.T) {
	// 构造期不触碰 db（探活闭包捕获但仅在 Measure 时调用），零值 *gorm.DB 安全。
	h, err := NewHealthChecker(&gorm.DB{}, nil)
	require.NoError(t, err)
	require.NotNil(t, h)

	// 正常选项 + nil 混排同样安全
	h2, err := NewHealthChecker(&gorm.DB{}, nil, WithRedis(nil))
	require.NoError(t, err)
	assert.NotNil(t, h2)
}
