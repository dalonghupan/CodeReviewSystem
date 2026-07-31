package util

import (
	"context"
	"time"
)

// DefaultRetryDelays 默认重试间隔：LLD §7.2 第三方接口指数退避（1s、3s、5s）
var DefaultRetryDelays = []time.Duration{
	1 * time.Second,
	3 * time.Second,
	5 * time.Second,
}

// RetryWithBackoff 带退避的重试执行器
// fn 返回 nil 表示成功；超过重试次数返回最后一次错误
// ctx 取消时立即终止重试
func RetryWithBackoff(ctx context.Context, delays []time.Duration, fn func(ctx context.Context) error) error {
	var err error
	attempts := len(delays) + 1 // 首次执行 + N次重试
	for i := 0; i < attempts; i++ {
		if err = fn(ctx); err == nil {
			return nil
		}
		if i < len(delays) {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delays[i]):
			}
		}
	}
	return err
}
