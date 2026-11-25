// tracer.go：pgx 查询追踪（observability.md §3 DB 行）。
// na_db_query_duration_seconds（连接级 SQL：Query/QueryRow/Exec 均经此路径）+
// na_db_errors_total[op]（op ∈ query|begin|commit|rollback）。SQL 文本不进
// 标签（基数红线）。metrics == nil 时不装 tracer（零开销）。
package database

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/netyilei/netyadmin/internal/kernel/observability"
)

type tracerStartKey struct{}

// metricsTracer 实现 pgx.QueryTracer（pgx/v5 唯一连接级 tracer：Query/QueryRow/
// Exec 共用；Batch/CopyFrom/Prepare/Connect 不在本行范围，随消费方接入）。
type metricsTracer struct {
	m *observability.Metrics
}

func (t metricsTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryStartData) context.Context {
	return context.WithValue(ctx, tracerStartKey{}, time.Now())
}

func (t metricsTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	if t.m == nil {
		return
	}
	start, ok := ctx.Value(tracerStartKey{}).(time.Time)
	if !ok {
		return
	}
	t.m.DBQueryDuration.Observe(time.Since(start).Seconds())
	if data.Err != nil {
		t.m.DBErrors.WithLabelValues("query").Inc()
	}
}

// compile-time：接口签名漂移在编译期暴露。
var _ pgx.QueryTracer = metricsTracer{}
