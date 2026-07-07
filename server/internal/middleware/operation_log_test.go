package middleware

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	logEntity "NetyAdmin/internal/domain/entity/log"
	logService "NetyAdmin/internal/service/log"
)

// mockLogBus 用于测试 OperationLogger 是否调用了 Record。
type mockLogBus struct {
	recorded []logEntity.LogEntry
}

func (m *mockLogBus) Record(_ context.Context, entry logEntity.LogEntry) error {
	m.recorded = append(m.recorded, entry)
	return nil
}
func (m *mockLogBus) RecordSync(_ context.Context, entry logEntity.LogEntry) error {
	m.recorded = append(m.recorded, entry)
	return nil
}
func (m *mockLogBus) Stop() {}

// newTestEngine 构造一个挂载 OperationLogger 的 gin 引擎用于测试。
// writeHandler 是路由的业务 handler。
func newTestEngine(t *testing.T, logBus logService.LogBusService, middlewares []gin.HandlerFunc, writeHandler gin.HandlerFunc) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(OperationLogger(logBus))
	group := r.Group("/admin/v1")
	for _, mw := range middlewares {
		group.Use(mw)
	}
	group.POST("/items", writeHandler)
	group.DELETE("/items/:id", writeHandler)
	return r
}

// TestOperationLogger_LogsNormalWrite 验证普通写操作路由会被记录。
func TestOperationLogger_LogsNormalWrite(t *testing.T) {
	bus := &mockLogBus{}
	handler := func(c *gin.Context) { c.Status(http.StatusOK) }
	engine := newTestEngine(t, bus, nil, handler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/v1/items", bytes.NewBufferString(`{"name":"x"}`))
	engine.ServeHTTP(w, req)

	assert.Len(t, bus.recorded, 1, "普通 POST 写操作应被 OperationLogger 记录一次")
	if len(bus.recorded) == 1 {
		op, ok := bus.recorded[0].(*logEntity.Operation)
		assert.True(t, ok, "记录的应是 *Operation")
		assert.Equal(t, "create", op.Action)
		assert.Equal(t, "items", op.Resource)
	}
}

// TestOperationLogger_SkipsMarkedRoute 验证挂了 SkipOperationLog marker 的路由不被记录。
// 这是替代原硬编码字符串过滤（/admin/v1/operation-logs/*）的等价行为验证。
func TestOperationLogger_SkipsMarkedRoute(t *testing.T) {
	bus := &mockLogBus{}
	handler := func(c *gin.Context) { c.Status(http.StatusOK) }
	// 在 DELETE 路由上挂 SkipOperationLog marker（模拟 operation-logs 自身删除路由）
	engine := newTestEngine(t, bus, []gin.HandlerFunc{SkipOperationLog()}, handler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/admin/v1/items/1", nil)
	engine.ServeHTTP(w, req)

	assert.Empty(t, bus.recorded, "挂了 SkipOperationLog marker 的路由不应产生操作日志")
}

// TestOperationLogger_SkipsGetAndNonAdmin 验证 GET / 非 admin 前缀的请求不被记录。
func TestOperationLogger_SkipsGetAndNonAdmin(t *testing.T) {
	bus := &mockLogBus{}
	handler := func(c *gin.Context) { c.Status(http.StatusOK) }
	engine := newTestEngine(t, bus, nil, handler)

	// GET 请求不记录（method 过滤）
	w1 := httptest.NewRecorder()
	engine.ServeHTTP(w1, httptest.NewRequest(http.MethodGet, "/admin/v1/items", nil))
	assert.Empty(t, bus.recorded, "GET 请求不应记录")

	// 非 admin 前缀不记录
	gin.SetMode(gin.TestMode)
	engine2 := gin.New()
	engine2.Use(OperationLogger(bus))
	engine2.POST("/client/v1/items", handler)
	w2 := httptest.NewRecorder()
	engine2.ServeHTTP(w2, httptest.NewRequest(http.MethodPost, "/client/v1/items", bytes.NewBufferString(`{}`)))
	assert.Empty(t, bus.recorded, "非 /admin/ 前缀请求不应记录")
}
