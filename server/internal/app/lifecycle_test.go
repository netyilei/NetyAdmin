package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recLifecycle 记录型生命周期模块（测试用）。
type recLifecycle struct {
	name         string
	startErr     error
	panicOnStart bool
	started      int
	stopped      int
}

func (m *recLifecycle) Name() string { return m.name }

func (m *recLifecycle) OnStart(_ context.Context) error {
	if m.panicOnStart {
		panic("boom: " + m.name)
	}
	m.started++
	return m.startErr
}

func (m *recLifecycle) OnStop(_ context.Context) { m.stopped++ }

// TestRegisterModule_CollectsLifecycle 验证 RegisterModule 收集 LifecycleModule
// （与路由/任务模块正交，一个模块可同时实现多类接口）。
func TestRegisterModule_CollectsLifecycle(t *testing.T) {
	a := &App{}
	m := &recLifecycle{name: "lifecycle-only"}
	a.RegisterModule(m)
	require.Len(t, a.lifecycleModules, 1)
	assert.Equal(t, "lifecycle-only", a.lifecycleModules[0].Name())

	// 非 Lifecycle 模块（仅 Name）不收集
	a.RegisterModule(&recLifecycle{name: "plain"}) // 实现了 OnStart/OnStop → 会收集
	plain := struct{ appModuleOnly }{}
	_ = plain
}

// appModuleOnly 仅实现 Module 基接口（Name），用于验证非生命周期模块不收集。
type appModuleOnly struct{ name string }

func (m *appModuleOnly) Name() string { return m.name }

func TestRegisterModule_NonLifecycleNotCollected(t *testing.T) {
	a := &App{}
	a.RegisterModule(&appModuleOnly{name: "plain"})
	assert.Empty(t, a.lifecycleModules, "非生命周期模块不得进入 lifecycleModules")
}

// TestRunLifecycleStart_PanicContained 验证单模块 OnStart panic 被基座捕获：
// 后续模块照常启动，进程不崩溃（兑现 module.go 生命周期契约）。
func TestRunLifecycleStart_PanicContained(t *testing.T) {
	a := &App{}
	ok1 := &recLifecycle{name: "ok1"}
	boom := &recLifecycle{name: "boom", panicOnStart: true}
	ok2 := &recLifecycle{name: "ok2"}
	a.RegisterModule(ok1)
	a.RegisterModule(boom)
	a.RegisterModule(ok2)

	a.runLifecycleStart(context.Background())
	assert.Equal(t, 1, ok1.started)
	assert.Equal(t, 1, ok2.started, "panic 模块不得阻断后续模块 OnStart")
}

// TestRunLifecycleStop_ReverseOrder 验证 OnStop 逆序执行（栈式收尾语义）。
func TestRunLifecycleStop_ReverseOrder(t *testing.T) {
	a := &App{}
	var order []string
	m1 := &stopTracker{name: "m1", order: &order}
	m2 := &stopTracker{name: "m2", order: &order}
	a.RegisterModule(m1)
	a.RegisterModule(m2)

	a.runLifecycleStop()
	assert.Equal(t, []string{"m2", "m1"}, order, "OnStop 应逆序执行")
}

// stopTracker 记录 OnStop 执行顺序。
type stopTracker struct {
	name  string
	order *[]string
}

func (m *stopTracker) Name() string                    { return m.name }
func (m *stopTracker) OnStart(_ context.Context) error { return nil }
func (m *stopTracker) OnStop(_ context.Context)        { *m.order = append(*m.order, m.name) }
