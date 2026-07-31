// Package data iam-service 数据访问层
// 封装 PostgreSQL（sqlx+pgx，预编译语句杜绝注入）、Redis（权限缓存）
// 对应 LLD §1.2：service 禁止直连数据源，统一经 data 层
package data

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	_ "github.com/jackc/pgx/v5/stdlib" // pgx database/sql 驱动
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"

	"cr-system/app/iam-service/internal/conf"
	"cr-system/pkg/util"
)

// 权限缓存规范（LLD §6）：user:perm:{uid}:{tenantId}，TTL 2小时
const (
	permCacheKeyFmt = "user:perm:%s:%s"
	permCacheTTL    = 2 * time.Hour
)

// Data 数据层聚合：主库写、从库读、Redis缓存
type Data struct {
	writeDB *sqlx.DB        // 主库（写）
	readDB  *sqlx.DB        // 从库（读），未配置时等于 writeDB
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

	// 读写分离：配置了从库则查询走从库（HLD §4.1）
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
