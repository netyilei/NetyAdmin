// tracing.go：OTel SDK 接线（docs/rules/observability.md §4）。
// W3C traceparent 传播链 HTTP → Port → RPC → Bus → Job（RPC 随 TR-02）；
// 采样 ParentBased(TraceIDRatioBased(ratio))——上游已采样则跟随，否则按率。
// 双向桥接：自定义 trace_id（ctx / Headers.trace_id）与 OTel span 互为
// 备援——任一侧未接线时仍以 trace_id 头续链，不静默断链（§4）。
package observability

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// TraceIDHeader 是自定义 trace 头的唯一命名（bus.HeaderTraceID 别名引用本值；
// 与 W3C traceparent 并存：前者供日志/审计人读，后者供 OTel 传播）。
const TraceIDHeader = "trace_id"

// TracingOptions 是追踪装配参数（config.observability.tracing 段映射， 注入）。
type TracingOptions struct {
	Enabled bool // 总开关；true 而 Endpoint 空 = 配置非法（fail-closed 在 config 校验）
	// Endpoint 是 OTLP/HTTP collector 地址（如 localhost:4318；不用
	// docker——本地无 collector 时 enabled=false 零开销）。
	Endpoint string
	// SampleRatio 采样率 [0,1]（开发 1.0；生产基线 0.1 可配，observability.md §4）。
	SampleRatio float64
	// ServiceName 是 resource service.name（节点标识）。
	ServiceName string
	// SpanExporter 供测试/自定义导出注入；nil = 按 Endpoint 建 OTLP/HTTP exporter。
	SpanExporter sdktrace.SpanExporter
}

// Tracing 是已装配的 TracerProvider 句柄（nil-safe：未接线时 Enabled=false，
// 各消费方零开销直通）。
type Tracing struct {
	tp      *sdktrace.TracerProvider
	enabled bool
}

// SetupTracing 组装全局 TracerProvider 与 W3C 传播器（进程级单次调用，cmd 注入）。
// enabled=false 或无 exporter 时返回 no-op 句柄。
func SetupTracing(opts TracingOptions) (*Tracing, error) {
	if !opts.Enabled {
		otel.SetTextMapPropagator(propagation.TraceContext{}) // 传播器常开（透传上游 traceparent 无副作用）
		return &Tracing{}, nil
	}
	if opts.SampleRatio < 0 || opts.SampleRatio > 1 {
		return nil, fmt.Errorf("observability: sample_ratio %v out of range [0,1]", opts.SampleRatio)
	}
	exporter := opts.SpanExporter
	if exporter == nil {
		if opts.Endpoint == "" {
			return nil, errors.New("observability: tracing enabled but endpoint empty")
		}
		exp, err := otlptracehttp.New(context.Background(),
			otlptracehttp.WithEndpointURL(opts.Endpoint), otlptracehttp.WithInsecure())
		if err != nil {
			return nil, fmt.Errorf("observability: otlp exporter: %w", err)
		}
		exporter = exp
	}
	name := opts.ServiceName
	if name == "" {
		name = "netyadmin"
	}
	res := resource.NewSchemaless(semconv.ServiceNameKey.String(name))
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(opts.SampleRatio))),
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(exporter),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	return &Tracing{tp: tp, enabled: true}, nil
}

// Enabled 返回接线状态。
func (t *Tracing) Enabled() bool { return t != nil && t.enabled }

// Tracer 返回具名 tracer（未接线返回 no-op tracer——API 恒可用）。
func (t *Tracing) Tracer(name string) trace.Tracer {
	if t == nil || !t.enabled {
		return noop.NewTracerProvider().Tracer(name)
	}
	return t.tp.Tracer(name)
}

// Shutdown flush 未导出 span（bootstrap BeforeShutdown 段调用）。
func (t *Tracing) Shutdown(ctx context.Context) error {
	if t == nil || !t.enabled {
		return nil
	}
	return t.tp.Shutdown(ctx)
}

// Flush 强制导出缓冲 span（测试断言前的显式同步点；生产排空优先用 Shutdown）。
func (t *Tracing) Flush(ctx context.Context) error {
	if t == nil || !t.enabled {
		return nil
	}
	return t.tp.ForceFlush(ctx)
}

// StartSpan 是消费方的统一入口（nil-safe；span 名低基数约定 route/job/topic 模板）。
func (t *Tracing) StartSpan(ctx context.Context, tracer, name string, kind trace.SpanKind, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	return t.Tracer(tracer).Start(ctx, name, trace.WithSpanKind(kind), trace.WithAttributes(attrs...))
}

// —— 传播与桥接（非 HTTP 边界：Bus Headers / Job ctx） ——

type mapCarrier map[string]string

func (c mapCarrier) Get(key string) string { return c[key] }

func (c mapCarrier) Set(key, value string) { c[key] = value }

func (c mapCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}

// InjectTraceHeaders 将当前 trace 上下文以 W3C traceparent 写入 headers（就地）。
// 无活动 span 时桥接自定义 trace_id（合成远端 span id，不置采样位——采样决策
// 归消费方本地 sampler），保证未接线一侧的生产者也能续链。
func InjectTraceHeaders(ctx context.Context, headers map[string]string) {
	if !trace.SpanContextFromContext(ctx).IsValid() {
		if id := TraceIDFrom(ctx); id != "" {
			if sc := bridgeSpanContext(id); sc.IsValid() {
				ctx = trace.ContextWithSpanContext(ctx, sc)
			}
		}
	}
	otel.GetTextMapPropagator().Inject(ctx, mapCarrier(headers))
}

// ExtractTraceContext 从 headers 提取 trace 上下文注入 ctx；无 traceparent 时
// 回退桥接自定义 trace_id 头（对齐 Inject 的备援语义）。
func ExtractTraceContext(ctx context.Context, headers map[string]string) context.Context {
	ctx = otel.GetTextMapPropagator().Extract(ctx, mapCarrier(headers))
	if !trace.SpanContextFromContext(ctx).IsValid() {
		if id := headers[TraceIDHeader]; id != "" {
			if sc := bridgeSpanContext(id); sc.IsValid() {
				ctx = trace.ContextWithSpanContext(ctx, sc)
			}
		}
	}
	return ctx
}

// bridgeSpanContext 以自定义 trace_id（32 hex）合成远端 SpanContext。
func bridgeSpanContext(traceIDHex string) trace.SpanContext {
	var tid trace.TraceID
	if err := decodeHexFixed(tid[:], traceIDHex); err != nil {
		return trace.SpanContext{}
	}
	var sid trace.SpanID
	if _, err := rand.Read(sid[:]); err != nil {
		return trace.SpanContext{}
	}
	return trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: tid, SpanID: sid, Remote: true,
		// TraceFlags 零值：桥接上下文不声明已采样，采样决策归消费方本地 sampler。
	})
}

func decodeHexFixed(dst []byte, s string) error {
	if hex.EncodedLen(len(dst)) != len(s) {
		return errors.New("observability: trace id length mismatch")
	}
	_, err := hex.Decode(dst, []byte(s))
	return err
}
