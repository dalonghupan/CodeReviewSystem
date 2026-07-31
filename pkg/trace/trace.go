// Package trace OpenTelemetry 链路追踪初始化
// 对应 LLD §10：Jaeger采集全链路调用轨迹，TraceID贯穿网关、微服务、MQ
package trace

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// Config 链路追踪配置
type Config struct {
	ServiceName    string  // 服务名
	ServiceVersion string  // 服务版本（与镜像版本一致）
	Endpoint       string  // Jaeger/OTLP Collector gRPC 地址，如 jaeger:4317
	SampleRatio    float64 // 采样率 0.0-1.0，生产建议 0.1-1.0
	Insecure       bool    // 是否使用明文连接（内网集群通常为 true）
}

// InitProvider 初始化全局 TracerProvider
// 返回 shutdown 函数，服务退出时调用以冲刷缓存的 span
func InitProvider(cfg Config) (func(context.Context) error, error) {
	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(cfg.Endpoint),
	}
	if cfg.Insecure {
		opts = append(opts, otlptracegrpc.WithInsecure())
	}

	ctx := context.Background()
	exporter, err := otlptracegrpc.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("创建OTLP导出器失败: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
			attribute.String("deploy.env", "k8s"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("创建资源描述失败: %w", err)
	}

	ratio := cfg.SampleRatio
	if ratio <= 0 || ratio > 1 {
		ratio = 1.0
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))),
	)

	otel.SetTracerProvider(tp)
	// 使用 W3C TraceContext 传播标准，与 APISIX 网关透传兼容
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp.Shutdown, nil
}

// TraceIDFromContext 从 OTel span context 提取 TraceID
// 用于日志字段注入、MQ 消息头透传
func TraceIDFromContext(ctx context.Context) string {
	sc := oteltrace.SpanContextFromContext(ctx)
	if sc.HasTraceID() {
		return sc.TraceID().String()
	}
	return ""
}

// SpanIDFromContext 从 OTel span context 提取 SpanID
func SpanIDFromContext(ctx context.Context) string {
	sc := oteltrace.SpanContextFromContext(ctx)
	if sc.HasSpanID() {
		return sc.SpanID().String()
	}
	return ""
}
