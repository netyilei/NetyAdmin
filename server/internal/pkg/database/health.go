// Package database 提供数据库相关的健康检查能力。
//
// 健康检查基于 github.com/hellofresh/health-go/v5 实现：
//   - 复用应用已有的 *gorm.DB 与 *redis.Client 连接池（不新建连接、不引入额外驱动依赖）；
//   - 暴露标准 /health HTTP 端点，便于 K8s liveness/readiness 探针或负载均衡探测；
//   - 启动期主动探测一次，确保依赖可用后再对外提供服务。
//
// 这替代了原先自研的、且 Start() 从未被调用的 HealthChecker（死代码）。
package database

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hellofresh/health-go/v5"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// HealthChecker 健康检查器：聚合 DB / Redis 探活，并对外提供 HTTP handler。
type HealthChecker struct {
	health *health.Health
}

// 健康检查相关常量（避免魔法数字散落）。
const (
	// checkTimeout 单项依赖探活超时上限。
	checkTimeout = 3 * time.Second
	// startupProbeTimeout 启动期主动探活的总体超时（需覆盖所有注册检查）。
	startupProbeTimeout = 5 * time.Second
	// componentVersion 对外暴露的组件版本标识（供 /health 响应与运维识别）。
	componentVersion = "v1"
	// componentName 对外暴露的组件名称。
	componentName = "NetyAdmin"
	// checkNamePostgres / checkNameRedis 注册到 health-go 的检查项名称，
	// 同时也是 /health 响应 JSON 中 failures map 的 key。
	checkNamePostgres = "postgres"
	checkNameRedis    = "redis"
)

// HealthCheckerOption 配置项（函数选项模式，与项目既有风格一致）。
type HealthCheckerOption func(*HealthChecker)

// WithRedis 注册 Redis 健康检查（可选：仅在 Redis 启用时传入）。
func WithRedis(client *redis.Client) HealthCheckerOption {
	return func(h *HealthChecker) {
		if client == nil {
			return
		}
		_ = h.health.Register(health.Config{
			Name:    checkNameRedis,
			Timeout: checkTimeout,
			Check: func(ctx context.Context) error {
				if err := client.Ping(ctx).Err(); err != nil {
					return fmt.Errorf("redis ping failed: %w", err)
				}
				return nil
			},
		})
	}
}

// NewHealthChecker 创建健康检查器并注册 PostgreSQL 检查（必选）。
//
// 设计说明：复用 *gorm.DB 底层的 *sql.DB 连接池做 PingContext，
// 避免像 health-go 内置 postgres/pgx checker 那样每次探活新建独立连接，
// 既减少连接开销，也不需要额外引入 lib/pq 或重复构造 pgx DSN。
func NewHealthChecker(db *gorm.DB, opts ...HealthCheckerOption) (*HealthChecker, error) {
	hh, err := health.New(
		health.WithComponent(health.Component{
			Name:    componentName,
			Version: componentVersion,
		}),
		health.WithChecks(health.Config{
			Name:    checkNamePostgres,
			Timeout: checkTimeout,
			Check: func(ctx context.Context) error {
				sqlDB, err := db.DB()
				if err != nil {
					return fmt.Errorf("获取数据库底层连接失败: %w", err)
				}
				if err := sqlDB.PingContext(ctx); err != nil {
					return fmt.Errorf("postgres ping failed: %w", err)
				}
				return nil
			},
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("初始化健康检查器失败: %w", err)
	}

	h := &HealthChecker{health: hh}
	for _, opt := range opts {
		opt(h)
	}
	return h, nil
}

// Start 启动期执行一次主动探活：若关键依赖不可用，记录告警但不阻断启动
// （业务层有重试/降级容错；真正熔断由 K8s 探针 + 上游负载均衡负责）。
func (h *HealthChecker) Start() {
	ctx, cancel := context.WithTimeout(context.Background(), startupProbeTimeout)
	defer cancel()

	check := h.health.Measure(ctx)
	if check.Status != health.StatusOK && check.Status != health.StatusPartiallyAvailable {
		slog.Error("启动期探活异常", "status", check.Status, "failures", check.Failures)
		return
	}
	slog.Info("启动期探活通过", "status", check.Status)
}

// Handler 返回 Gin 中间件形式的标准 /health 端点处理器。
// 用法：engine.GET("/health", healthChecker.Handler())
func (h *HealthChecker) Handler() gin.HandlerFunc {
	hh := h.health.Handler()
	return func(c *gin.Context) {
		hh.ServeHTTP(c.Writer, c.Request)
	}
}

// HandlerHTTP 返回标准 net/http Handler，便于挂载到非 Gin 的 mux 上。
func (h *HealthChecker) HandlerHTTP() http.Handler {
	return h.health.Handler()
}

// Stop 保留接口对称（health-go 为无状态探活，无需释放资源）。
// 仅为兼容 App.Run() 既有关闭流程，避免外层调用点改动。
func (h *HealthChecker) Stop() {
	// no-op：health-go 不持有需要主动释放的后台资源
}

// IsHealthy 同步执行一次探活，返回是否完全健康。
// 供需要同步判断的场景使用（如内部诊断接口、运维脚本）。
func (h *HealthChecker) IsHealthy() bool {
	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	defer cancel()
	return h.health.Measure(ctx).Status == health.StatusOK
}
