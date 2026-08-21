// Package data git-adapter 数据访问层
// 封装 PostgreSQL（授权/仓库/黑名单/敏感文件配置）、Redis（Diff缓存、限流计数、授权Token缓存）
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

	"cr-system/app/git-adapter/internal/conf"
	"cr-system/pkg/util"
)

// 缓存Key与TTL规范（LLD §6）
const (
	// Diff解析数据：diff:{mrId}:{commitHash}:{filePath}，TTL 7天
	// （LLD §6 定义为 diff:{mrId}:{commitHash}，因 GetDiffData 接口按文件粒度查询，追加文件维度）
	diffCacheKeyFmt = "diff:%s:%s:%s"
	diffCacheTTL    = 7 * 24 * time.Hour

	// 仓库基础信息：repo:info:{repoId}，TTL 12小时
	repoCacheKeyFmt = "repo:info:%s"
	repoCacheTTL    = 12 * time.Hour

	// Git接口限流计数：git_limit:{platform}:{sourceIp}，按小时过期
	rateLimitKeyFmt = "git_limit:%s:%s"
	rateLimitTTL    = time.Hour

	// OAuth state 临时会话缓存（CSRF防护）
	oauthStateKeyFmt = "oauth_state:%s"
	oauthStateTTL    = 10 * time.Minute
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
