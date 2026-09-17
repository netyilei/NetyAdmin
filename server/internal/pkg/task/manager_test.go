package task

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"NetyAdmin/internal/config"
)

// fakeTask 最小 Task 实现，用于引擎行为测试。
type fakeTask struct {
	name string
	spec string
}

func (f *fakeTask) Name() string        { return f.name }
func (f *fakeTask) DisplayName() string { return f.name }
func (f *fakeTask) Run(_ context.Context) error {
	return nil
}
func (f *fakeTask) Execute(_ context.Context, _ json.RawMessage) error { return nil }
func (f *fakeTask) DefaultMetadata() TaskMetadata {
	return TaskMetadata{Name: f.name, DisplayName: f.name, Type: TypeInterval, Spec: f.spec, Enabled: true}
}

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	m := NewManager(&config.TaskConfig{}, &config.RedisConfig{Enabled: false}, nil)
	return m
}

// TestStartTaskRejectsDuplicateInterval 验证防重入守卫：
// interval 任务运行中再次 StartTask 必须被拒（守卫查 intervals 注册表而非 IsRunning）。
func TestStartTaskRejectsDuplicateInterval(t *testing.T) {
	m := newTestManager(t)
	task := &fakeTask{name: "test_dup", spec: "1h"}
	m.Register(task)

	if err := m.StartTask(context.Background(), "test_dup"); err != nil {
		t.Fatalf("first StartTask: %v", err)
	}
	if err := m.StartTask(context.Background(), "test_dup"); err == nil {
		t.Fatal("second StartTask should be rejected while interval task is running")
	}
	if err := m.StopTask("test_dup"); err != nil {
		t.Fatalf("StopTask: %v", err)
	}
	// 停止后可重新启动
	if err := m.StartTask(context.Background(), "test_dup"); err != nil {
		t.Fatalf("StartTask after stop: %v", err)
	}
	m.StopTask("test_dup")
}

// TestIntervalNaturalExitFreesRegistry 回归测试：interval 任务自然退出
// （如 spec 无效）后必须清理 intervals 注册表，否则 StartTask 永久报"正在运行中"。
func TestIntervalNaturalExitFreesRegistry(t *testing.T) {
	m := newTestManager(t)
	// spec 非法 → runIntervalTask 立即退出
	task := &fakeTask{name: "test_bad_spec", spec: "not-a-duration"}
	m.Register(task)

	if err := m.StartTask(context.Background(), "test_bad_spec"); err != nil {
		t.Fatalf("StartTask with invalid spec: %v", err)
	}
	// 等 goroutine 退出 + defer 清理执行
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if err := m.StartTask(context.Background(), "test_bad_spec"); err == nil {
			return // 注册表已清理，重启成功 —— 修复生效
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("interval task registry entry not freed after natural exit; StartTask permanently blocked")
}
