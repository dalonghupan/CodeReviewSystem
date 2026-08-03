// Package data cr-core 数据访问层
// 封装 PostgreSQL（评审单/评论分区表/流转日志/Sonar结果）、Redis（分布式锁）
// 对应 LLD §1.2：service 禁止直连数据源
package data

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"

	"cr-system/app/cr-core/internal/conf"
	"cr-system/pkg/util"
)

// 分布式锁规范（LLD §3.3-3 / §6-4）：lock:review_mr:{mrId}，30秒超时
const (
	reviewLockKeyFmt = "lock:review_mr:%s"
	reviewLockTTL    = 30 * time.Second
)

// Data 数据层聚合
type Data struct {
	writeDB *sqlx.DB
	readDB  *sqlx.DB
	rdb     *redis.Client
	log     *log.Helper
}

// NewData 初始化数据层
func NewData(cfg *conf.Data, logger log.Logger) (*Data, func(), error) {
	helper := log.NewHelper(logger)

	writeDB, err := newPostgres(cfg.Postgres.DSN, cfg.Postgres)
	if err != nil {
		return nil, nil, fmt.Errorf("连接主库失败: %w", err)
	}

	readDB := writeDB
	if cfg.Postgres.ReadDSN != "" {
		readDB, err = newPostgres(cfg.Postgres.ReadDSN, cfg.Postgres)
		if err != nil {
			_ = writeDB.Close()
			return nil, nil, fmt.Errorf("连接从库失败: %w", err)
		}
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:         cfg.Redis.Addr,
		Password:     cfg.Redis.Password,
		DB:           cfg.Redis.DB,
		DialTimeout:  util.ParseDurationOr(cfg.Redis.DialTimeout, 3*time.Second),
		ReadTimeout:  util.ParseDurationOr(cfg.Redis.ReadTimeout, 2*time.Second),
		WriteTimeout: util.ParseDurationOr(cfg.Redis.WriteTimeout, 2*time.Second),
	})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		_ = writeDB.Close()
		return nil, nil, fmt.Errorf("连接Redis失败: %w", err)
	}

	d := &Data{writeDB: writeDB, readDB: readDB, rdb: rdb, log: helper}
	cleanup := func() {
		_ = writeDB.Close()
		if readDB != writeDB {
			_ = readDB.Close()
		}
		_ = rdb.Close()
		helper.Info("数据层资源已释放")
	}
	return d, cleanup, nil
}

func newPostgres(dsn string, cfg conf.Postgres) (*sqlx.DB, error) {
	db, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(util.ParseDurationOr(cfg.ConnMaxLifetime, 30*time.Minute))
	return db, nil
}

// ==================== 分布式锁（LLD §3.3-3） ====================

// AcquireReviewLock 获取评审单创建锁：lock:review_mr:{mrId}，30秒超时
// 返回 true 表示加锁成功；防同一 MR 重复创建多张评审单
func (d *Data) AcquireReviewLock(ctx context.Context, mrID string) (bool, error) {
	return d.rdb.SetNX(ctx, fmt.Sprintf(reviewLockKeyFmt, mrID), "1", reviewLockTTL).Result()
}

// ReleaseReviewLock 释放锁
func (d *Data) ReleaseReviewLock(ctx context.Context, mrID string) {
	if err := d.rdb.Del(ctx, fmt.Sprintf(reviewLockKeyFmt, mrID)).Err(); err != nil {
		d.log.Warnw("msg", "释放评审锁失败", "mr_id", mrID, "error", err.Error())
	}
}
