// Package logger 基于 zap 的结构化日志封装
// 实现 kratos log.Logger 接口，支持从 context 自动提取 TraceID
// 对应 LLD §1.3：全链路携带TraceId，贯穿网关、微服务、MQ、日志、Jaeger
package logger

import (
	"context"
	"fmt"
	"os"

	klog "github.com/go-kratos/kratos/v2/log"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// TraceIDKey context 中 TraceID 的键（由 middleware/trace 注入）
type traceIDKey struct{}

// WithTraceID 将 TraceID 写入 context
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDKey{}, traceID)
}

// TraceIDFromContext 从 context 提取 TraceID
func TraceIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(traceIDKey{}).(string); ok {
		return v
	}
	return ""
}

// Config 日志配置
type Config struct {
	ServiceName string // 服务名，输出到每条日志的 service 字段
	Level       string // debug / info / warn / error，默认 info
	Development bool   // 开发模式：控制台彩色输出；生产模式：JSON 输出（供 Promtail 采集至 Loki）
}

// zapLogger 适配 kratos log.Logger 接口
type zapLogger struct {
	sugared *zap.SugaredLogger
}

// New 创建 kratos 兼容的日志器
func New(cfg Config) klog.Logger {
	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	var core zapcore.Core
	level := parseLevel(cfg.Level)
	if cfg.Development {
		encoderCfg = zap.NewDevelopmentEncoderConfig()
		core = zapcore.NewCore(
			zapcore.NewConsoleEncoder(encoderCfg),
			zapcore.AddSync(os.Stdout),
			level,
		)
	} else {
		core = zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderCfg),
			zapcore.AddSync(os.Stdout),
			level,
		)
	}

	zapLog := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(2)).
		With(zap.String("service", cfg.ServiceName))

	return klog.With(&zapLogger{sugared: zapLog.Sugar()},
		"ts", klog.DefaultTimestamp,
		"caller", klog.DefaultCaller,
	)
}

// Log 实现 kratos log.Logger 接口
// 自动补充 trace_id 字段：若 kv 中已携带 context 则从中提取
func (l *zapLogger) Log(level klog.Level, keyvals ...interface{}) error {
	var ctx context.Context
	fields := make([]interface{}, 0, len(keyvals)+2)
	for i := 0; i < len(keyvals); i += 2 {
		if c, ok := keyvals[i+1].(context.Context); ok && ctx == nil {
			ctx = c
			continue // context 不作为普通字段输出
		}
		fields = append(fields, keyvals[i])
		if i+1 < len(keyvals) {
			fields = append(fields, keyvals[i+1])
		}
	}
	if ctx != nil {
		if tid := TraceIDFromContext(ctx); tid != "" {
			fields = append(fields, "trace_id", tid)
		}
	}

	switch level {
	case klog.LevelDebug:
		l.sugared.Debugw("", fields...)
	case klog.LevelInfo:
		l.sugared.Infow("", fields...)
	case klog.LevelWarn:
		l.sugared.Warnw("", fields...)
	case klog.LevelError:
		l.sugared.Errorw("", fields...)
	case klog.LevelFatal:
		l.sugared.Fatalw("", fields...)
	}
	return nil
}

// WithContext 返回携带 context 的 helper，日志自动带 TraceID
func WithContext(ctx context.Context, l klog.Logger) *klog.Helper {
	return klog.NewHelper(klog.WithContext(ctx, l))
}

func parseLevel(level string) zapcore.Level {
	switch level {
	case "debug":
		return zapcore.DebugLevel
	case "warn":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}

// Sync 刷新缓冲区（服务退出前调用）
func Sync(l klog.Logger) {
	if zl, ok := l.(*zapLogger); ok {
		_ = zl.sugared.Sync()
	}
}

// 确保实现接口
var _ klog.Logger = (*zapLogger)(nil)

// String 便捷函数：构造 kratos 日志字段
func String(key, val string) interface{} {
	return fmt.Sprintf("%s=%s", key, val)
}
