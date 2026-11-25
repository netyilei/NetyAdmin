// Package database 提供连接管理、UnitOfWork（OnCommit/OnRollback）、模块迁移执行器与只读副本。
//
// 连接：默认单 PG（pgxpool）；只读副本可选（读流量显式路由，不做自动 SQL 解析）。
// 事务：WithinTx——ctx 传播（嵌套共享同一事务）；commit 成功后执行 OnCommit 回调
// （缓存失效等「commit 之后」的动作挂这里，G2）；rollback 后执行 OnRollback。
// 红线：禁跨模块 JOIN/事务（模块各自表/事务边界在模块内）；禁 GORM AutoMigrate；
// 复杂/批量查询用 pgx 原生，GORM CRUD 在各模块 adapters。
// 迁移：goose/v3 引擎 + 自研 runner 外壳（5 位号段/schema-seed 分离/checksum 拦截，
// docs/rules/migrations.md；CI check-migrations 的运行时对偶）。
package database

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/netyilei/netyadmin/internal/kernel/config"
	"github.com/netyilei/netyadmin/internal/kernel/observability"
)

// Pool 是主连接池（读写）。
type Pool struct {
	*pgxpool.Pool
	dsn     string
	metrics *observability.Metrics // 可空（nil=na_db_* 不计数；）
}

// Replica 是只读副本池（可选；nil 表示无副本）。
type Replica struct {
	*pgxpool.Pool
}

// Cluster 为主/副本组合：写走 Pool，读可显式走 Replica（无副本时读也走主）。
type Cluster struct {
	Write *Pool
	Read  *Replica // 可为 nil
}

// Option 调整主连接池参数（New 可变参——既有调用零改动）。
type Option func(*pgxpool.Config)

// WithMaxConns 设置主池连接上限（0=保持 pgxpool 默认 max(4,NumCPU)——
// config.Database.MaxConns；装-A3 池旋钮）。
func WithMaxConns(n int) Option {
	return func(cfg *pgxpool.Config) {
		if n > 0 {
			cfg.MaxConns = int32(n)
		}
	}
}

// New 打开主连接池（fail-fast：启动期连通性校验 ping；metrics 为
// na_db_* 指标接线，可空——非 nil 时装配 pgx QueryTracer）。
func New(ctx context.Context, dsn string, metrics *observability.Metrics, opts ...Option) (*Pool, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, errors.New("database: empty dsn")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("database: parse dsn: %w", err)
	}
	for _, opt := range opts {
		opt(cfg)
	}
	if metrics != nil {
		cfg.ConnConfig.Tracer = metricsTracer{m: metrics}
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("database: new pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("database: ping: %w", err)
	}
	return &Pool{Pool: pool, dsn: dsn, metrics: metrics}, nil
}

// NewReplica 打开只读副本池（DSN 为空返回 nil，表示未配置副本）。
func NewReplica(ctx context.Context, dsn string) (*Replica, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, nil
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("database: new replica pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("database: replica ping: %w", err)
	}
	return &Replica{Pool: pool}, nil
}

// BuildDSN 从配置段构建 PostgreSQL DSN（密码经 Secret.Value 原值注入；仅装配用，不落日志）。
func BuildDSN(db config.Database) string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		db.User, db.Password.Value(), db.Host, db.Port, db.Name, db.SSLMode,
	)
}

// Close 关闭主池（副本由调用方单独 Close）。
func (p *Pool) Close() {
	if p.Pool != nil {
		p.Pool.Close()
	}
}

// observeTx 记录事务总时长与错误（result ∈ commit|rollback——
// commit 失败亦按 rollback 归类：事务未生效）。
func (p *Pool) observeTx(_ context.Context, result string, start time.Time, err error) {
	if p.metrics == nil {
		return
	}
	p.metrics.DBTxDuration.WithLabelValues(result).Observe(time.Since(start).Seconds())
	if err != nil {
		p.metrics.DBErrors.WithLabelValues(result).Inc()
	}
}

// Close 关闭副本池。
func (r *Replica) Close() {
	if r.Pool != nil {
		r.Pool.Close()
	}
}
