package data

import (
	"context"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// ==================== Diff 结构化缓存（LLD §2 步骤9 / §6-3） ====================

// CacheDiff 写入结构化 Diff 缓存，TTL 7天
func (d *Data) CacheDiff(ctx context.Context, mrID, commitHash, filePath string, data []byte) error {
	return d.rdb.Set(ctx, fmt.Sprintf(diffCacheKeyFmt, mrID, commitHash, filePath), data, diffCacheTTL).Err()
}

// GetCachedDiff 读取结构化 Diff 缓存（未命中返回 nil, nil）
func (d *Data) GetCachedDiff(ctx context.Context, mrID, commitHash, filePath string) ([]byte, error) {
	val, err := d.rdb.Get(ctx, fmt.Sprintf(diffCacheKeyFmt, mrID, commitHash, filePath)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return val, nil
}

// DeleteCachedDiff 删除缓存（MR更新后旧版本失效）
func (d *Data) DeleteCachedDiff(ctx context.Context, mrID, commitHash, filePath string) error {
	return d.rdb.Del(ctx, fmt.Sprintf(diffCacheKeyFmt, mrID, commitHash, filePath)).Err()
}

// ==================== Git 接口限流（LLD §3.2-2 防平台封禁） ====================

// IncrGitCall 递增限流计数器，返回当前小时已调用次数
// Key: git_limit:{platform}:{sourceIp}，按小时过期
func (d *Data) IncrGitCall(ctx context.Context, platform, sourceIP string) (int64, error) {
	key := fmt.Sprintf(rateLimitKeyFmt, platform, sourceIP)
	pipe := d.rdb.Pipeline()
	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, rateLimitTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, err
	}
	return incr.Val(), nil
}

// GetGitCallCount 查询当前计数（不递增，限流预检用）
func (d *Data) GetGitCallCount(ctx context.Context, platform, sourceIP string) (int64, error) {
	val, err := d.rdb.Get(ctx, fmt.Sprintf(rateLimitKeyFmt, platform, sourceIP)).Int64()
	if errors.Is(err, redis.Nil) {
		return 0, nil
	}
	return val, err
}
