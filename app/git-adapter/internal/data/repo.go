package data

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"

	"cr-system/pkg/errcode"
	"cr-system/pkg/util"
)

// BindRepository 绑定仓库（幂等：平台+平台仓库ID+租户 唯一索引）
func (d *Data) BindRepository(ctx context.Context, r *GitRepository) error {
	r.RepoID = util.NewUUID()
	const q = `INSERT INTO git_repository (repo_id, tenant_id, auth_id, platform,
		platform_repo_id, full_name, description, default_branch, clone_url, web_url, is_private)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (platform, platform_repo_id, tenant_id)
		DO UPDATE SET is_bound = TRUE, auth_id = EXCLUDED.auth_id, updated_at = NOW()
		RETURNING repo_id`
	if err := d.writeDB.QueryRowContext(ctx, q, r.RepoID, r.TenantID, r.AuthID, r.Platform,
		r.PlatformRepoID, r.FullName, r.Description, r.DefaultBranch,
		r.CloneURL, r.WebURL, r.IsPrivate).Scan(&r.RepoID); err != nil {
		return errcode.ErrDatabase.WithDetail(err.Error())
	}
	return nil
}

// GetRepository 查询仓库（带 Redis 缓存 repo:info:{repoId} TTL=12h，LLD §6）
func (d *Data) GetRepository(ctx context.Context, repoID string) (*GitRepository, error) {
	cacheKey := fmt.Sprintf(repoCacheKeyFmt, repoID)
	if cached, err := d.rdb.Get(ctx, cacheKey).Result(); err == nil {
		var r GitRepository
		if json.Unmarshal([]byte(cached), &r) == nil {
			return &r, nil
		}
	} else if !errors.Is(err, redis.Nil) {
		d.log.Warnw("msg", "仓库缓存读取异常，降级查库", "key", cacheKey, "error", err.Error())
	}

	var r GitRepository
	const q = `SELECT repo_id, tenant_id, auth_id, platform, platform_repo_id, full_name,
		description, default_branch, clone_url, web_url, is_private, is_bound,
		sync_status, last_synced_at, created_at, updated_at
		FROM git_repository WHERE repo_id = $1 AND is_bound = TRUE`
	if err := d.readDB.GetContext(ctx, &r, q, repoID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errcode.ErrRepoNotFound
		}
		return nil, errcode.ErrDatabase.WithDetail(err.Error())
	}

	if data, err := json.Marshal(&r); err == nil {
		_ = d.rdb.Set(ctx, cacheKey, data, repoCacheTTL).Err()
	}
	return &r, nil
}

// ListRepositories 分页查询租户绑定仓库
func (d *Data) ListRepositories(ctx context.Context, tenantID, platform, keyword string, page, pageSize uint32) ([]*GitRepository, uint32, error) {
	page, pageSize = util.NormalizePage(page, pageSize)

	const filter = `FROM git_repository
		WHERE tenant_id = $1 AND is_bound = TRUE
		  AND ($2 = '' OR platform = $2)
		  AND full_name ILIKE '%' || $3 || '%'`

	var total uint32
	if err := d.readDB.GetContext(ctx, &total, `SELECT COUNT(*) `+filter, tenantID, platform, keyword); err != nil {
		return nil, 0, errcode.ErrDatabase.WithDetail(err.Error())
	}

	var items []*GitRepository
	const q = `SELECT repo_id, tenant_id, auth_id, platform, platform_repo_id, full_name,
		description, default_branch, clone_url, web_url, is_private, is_bound,
		sync_status, last_synced_at, created_at, updated_at ` + filter + `
		ORDER BY created_at DESC LIMIT $4 OFFSET $5`
	if err := d.readDB.SelectContext(ctx, &items, q, tenantID, platform, keyword,
		pageSize, util.PageOffset(page, pageSize)); err != nil {
		return nil, 0, errcode.ErrDatabase.WithDetail(err.Error())
	}
	return items, total, nil
}

// UnbindRepository 解绑仓库（软删除保留记录，DDL is_bound 注释）
func (d *Data) UnbindRepository(ctx context.Context, repoID string) error {
	const q = `UPDATE git_repository SET is_bound = FALSE, updated_at = NOW() WHERE repo_id = $1`
	res, err := d.writeDB.ExecContext(ctx, q, repoID)
	if err != nil {
		return errcode.ErrDatabase.WithDetail(err.Error())
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errcode.ErrRepoNotFound
	}
	_ = d.rdb.Del(ctx, fmt.Sprintf(repoCacheKeyFmt, repoID)).Err()
	return nil
}

// UpdateRepoSyncStatus 更新仓库同步状态
func (d *Data) UpdateRepoSyncStatus(ctx context.Context, repoID, status string) error {
	const q = `UPDATE git_repository SET sync_status = $2,
		last_synced_at = CASE WHEN $2 = 'synced' THEN NOW() ELSE last_synced_at END,
		updated_at = NOW()
		WHERE repo_id = $1`
	if _, err := d.writeDB.ExecContext(ctx, q, repoID, status); err != nil {
		return errcode.ErrDatabase.WithDetail(err.Error())
	}
	_ = d.rdb.Del(ctx, fmt.Sprintf(repoCacheKeyFmt, repoID)).Err()
	return nil
}

// ListReposByAuth 查询授权关联的绑定仓库（撤销授权时联动解绑）
func (d *Data) ListReposByAuth(ctx context.Context, authID string) ([]*GitRepository, error) {
	var items []*GitRepository
	const q = `SELECT repo_id, tenant_id, auth_id, platform, platform_repo_id, full_name,
		description, default_branch, clone_url, web_url, is_private, is_bound,
		sync_status, last_synced_at, created_at, updated_at
		FROM git_repository WHERE auth_id = $1 AND is_bound = TRUE`
	if err := d.readDB.SelectContext(ctx, &items, q, authID); err != nil {
		return nil, errcode.ErrDatabase.WithDetail(err.Error())
	}
	return items, nil
}

// ListBoundRepos 查询全部绑定仓库（增量同步定时任务用）
func (d *Data) ListBoundRepos(ctx context.Context) ([]*GitRepository, error) {
	var items []*GitRepository
	const q = `SELECT repo_id, tenant_id, auth_id, platform, platform_repo_id, full_name,
		description, default_branch, clone_url, web_url, is_private, is_bound,
		sync_status, last_synced_at, created_at, updated_at
		FROM git_repository WHERE is_bound = TRUE`
	if err := d.readDB.SelectContext(ctx, &items, q); err != nil {
		return nil, errcode.ErrDatabase.WithDetail(err.Error())
	}
	return items, nil
}

// ==================== 黑白名单（LLD §3.2-4 黑名单拦截） ====================

// SetBlacklist 全量覆盖租户黑名单（事务：先删后插）
func (d *Data) SetBlacklist(ctx context.Context, tenantID string, patterns []string, createdBy string) error {
	tx, err := d.writeDB.BeginTxx(ctx, nil)
	if err != nil {
		return errcode.ErrDatabase.WithDetail(err.Error())
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM repo_blacklist WHERE tenant_id = $1`, tenantID); err != nil {
		return errcode.ErrDatabase.WithDetail(err.Error())
	}
	for _, p := range patterns {
		const q = `INSERT INTO repo_blacklist (tenant_id, repo_pattern, created_by)
			VALUES ($1, $2, $3) ON CONFLICT (tenant_id, repo_pattern) DO NOTHING`
		if _, err := tx.ExecContext(ctx, q, tenantID, p, createdBy); err != nil {
			return errcode.ErrDatabase.WithDetail(err.Error())
		}
	}
	if err := tx.Commit(); err != nil {
		return errcode.ErrDatabase.WithDetail(err.Error())
	}
	return nil
}

// GetBlacklist 查询租户黑名单模式列表
func (d *Data) GetBlacklist(ctx context.Context, tenantID string) ([]string, error) {
	var patterns []string
	const q = `SELECT repo_pattern FROM repo_blacklist WHERE tenant_id = $1 ORDER BY created_at ASC`
	if err := d.readDB.SelectContext(ctx, &patterns, q, tenantID); err != nil {
		return nil, errcode.ErrDatabase.WithDetail(err.Error())
	}
	return patterns, nil
}

// ==================== 敏感文件配置 ====================

// SetSensitiveExtensions 全量覆盖租户敏感文件后缀（含系统默认）
func (d *Data) SetSensitiveExtensions(ctx context.Context, tenantID string, extensions []string, createdBy string) error {
	tx, err := d.writeDB.BeginTxx(ctx, nil)
	if err != nil {
		return errcode.ErrDatabase.WithDetail(err.Error())
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM sensitive_file_config WHERE tenant_id = $1`, tenantID); err != nil {
		return errcode.ErrDatabase.WithDetail(err.Error())
	}
	for _, ext := range extensions {
		const q = `INSERT INTO sensitive_file_config (tenant_id, file_extension, created_by)
			VALUES ($1, $2, $3) ON CONFLICT (tenant_id, file_extension) DO NOTHING`
		if _, err := tx.ExecContext(ctx, q, tenantID, ext, createdBy); err != nil {
			return errcode.ErrDatabase.WithDetail(err.Error())
		}
	}
	if err := tx.Commit(); err != nil {
		return errcode.ErrDatabase.WithDetail(err.Error())
	}
	return nil
}

// GetSensitiveExtensions 查询敏感文件后缀（租户配置 + 系统默认合并）
func (d *Data) GetSensitiveExtensions(ctx context.Context, tenantID string) ([]string, error) {
	var exts []string
	const q = `SELECT DISTINCT file_extension FROM sensitive_file_config
		WHERE tenant_id = $1 OR tenant_id = '00000000-0000-0000-0000-000000000000'
		ORDER BY file_extension`
	if err := d.readDB.SelectContext(ctx, &exts, q, tenantID); err != nil {
		return nil, errcode.ErrDatabase.WithDetail(err.Error())
	}
	return exts, nil
}
