// Package data job-scheduler 数据访问层
// PostgreSQL（job_execution_log / mq_reconciliation 表）
package data

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"

	"cr-system/app/job-scheduler/internal/conf"
	"cr-system/pkg/util"
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

// ReadDB 返回读库（从库优先）
func (d *Data) ReadDB() *sqlx.DB {
	return d.readDB
}

// WriteDB 返回写库
func (d *Data) WriteDB() *sqlx.DB {
	return d.writeDB
}

// RDB 返回 Redis 客户端
func (d *Data) RDB() *redis.Client {
	return d.rdb
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
