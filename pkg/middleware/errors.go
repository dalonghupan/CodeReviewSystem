package middleware

import (
	"context"
	"errors"

	kerrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"

	"cr-system/pkg/errcode"
)

// ErrorHandler 统一错误处理中间件（LLD §7.2）
// 将业务层返回的 errcode.Error 统一转换为 kratos errors.Error：
//   - HTTP 场景：状态码按 errcode.ToHTTPStatus 映射，body 携带业务错误码
//   - gRPC 场景：通过 kratos 标准错误元数据透传业务码
//
// 业务错误（20xxx）允许直接返回前端提示；系统/第三方错误不暴露内部细节。
func ErrorHandler() middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			reply, err := handler(ctx, req)
			if err == nil {
				return reply, nil
			}
			return reply, convertError(err)
		}
	}
}

// convertError 统一错误转换
func convertError(err error) error {
	// 已是业务错误码：包装为 kratos 标准错误
	var berr *errcode.Error
	if errors.As(err, &berr) {
		return errcodeToKratos(berr)
	}

	// 已是 kratos 错误：直接透传（框架内部产生的，如参数校验）
	var kerr *kerrors.Error
	if errors.As(err, &kerr) {
		return kerr
	}

	// 未知错误：按系统内部错误处理，不暴露细节（LLD §7.2-2）
	return errcodeToKratos(errcode.ErrInternal)
}

// errcodeToKratos 业务错误码 → kratos errors.Error
// Code 字段填充 HTTP 状态码，Reason 填充业务错误码字符串，Message 为前端可见文案
func errcodeToKratos(berr *errcode.Error) *kerrors.Error {
	httpStatus := int32(errcode.ToHTTPStatus(berr.Code))
	msg := berr.Message
	if berr.Detail != "" {
		msg = berr.Message + "：" + berr.Detail
	}
	kerr := kerrors.New(int(httpStatus), reasonOf(berr.Code), msg)
	return kerr
}

// reasonOf 业务码 → 大写下划线 Reason 字符串，如 20101 → "BIZ_20101"
func reasonOf(code int) string {
	switch {
	case code >= 30000:
		return "THIRD_PARTY_" + itoa(code)
	case code >= 20000:
		return "BIZ_" + itoa(code)
	default:
		return "SYS_" + itoa(code)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// BusinessCodeFromError 从错误中提取业务错误码（用于日志记录与告警埋点）
func BusinessCodeFromError(err error) int {
	var berr *errcode.Error
	if errors.As(err, &berr) {
		return berr.Code
	}
	return 0
}

// TransportName 获取当前请求的传输层名称（http/grpc），辅助日志
func TransportName(ctx context.Context) string {
	if tr, ok := transport.FromServerContext(ctx); ok {
		return tr.Kind().String()
	}
	return "unknown"
}
