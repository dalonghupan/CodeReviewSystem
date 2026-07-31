package middleware

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"

	"cr-system/pkg/trace"
)

// ServerLogging 服务端请求日志中间件
// 输出：传输类型、操作名、参数、耗时、错误业务码、TraceID（供 Loki 检索，LLD §10-2）
func ServerLogging(logger log.Logger) middleware.Middleware {
	helper := log.NewHelper(logger)
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			var (
				kind      = "unknown"
				operation = "unknown"
			)
			if tr, ok := transport.FromServerContext(ctx); ok {
				kind = tr.Kind().String()
				operation = tr.Operation()
			}

			start := time.Now()
			reply, err := handler(ctx, req)
			latency := time.Since(start)

			fields := []interface{}{
				"kind", kind,
				"operation", operation,
				"args", fmt.Sprintf("%+v", req),
				"latency_ms", latency.Milliseconds(),
				"trace_id", trace.TraceIDFromContext(ctx),
				"uid", UserIDFromContext(ctx),
			}
			if err != nil {
				fields = append(fields,
					"error", err.Error(),
					"biz_code", BusinessCodeFromError(err),
				)
				helper.Errorw(fields...)
			} else {
				helper.Infow(fields...)
			}
			return reply, err
		}
	}
}
