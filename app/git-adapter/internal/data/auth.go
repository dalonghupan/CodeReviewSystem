package data

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"cr-system/pkg/errcode"
	"cr-system/pkg/util"
)

// CreateAuth 写入授权记录（Token 已在 service 层加密）
func (d *Data) CreateAuth(ctx context.Context, a *GitAuth) error {
	a.AuthID = util.NewUUID()
	const q = `INSERT INTO git_auth (auth_id, tenant_id, platform, platform_user_id,
		platform_username, platform_avatar, access_token, refresh_token, token_expire_time)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`
	if _, err := d.writeDB.ExecContext(ctx, q, a.AuthID, a.TenantID, a.Platform,
		a.PlatformUserID, a.PlatformUsername, a.PlatformAvatar,
		a.AccessToken, a.RefreshToken, a.TokenExpireTime); err != nil {
		return errcode.ErrDatabase.WithDetail(err.Error())
	}
	return nil
}

// GetAuth 查询授权记录
func (d *Data) GetAuth(ctx context.Context, authID string) (*GitAuth, error) {
	var a GitAuth
	const q = `SELECT auth_id, tenant_id, platform, platform_user_id, platform_username,
		platform_avatar, access_token, refresh_token, token_expire_time,
		sync_status, last_synced_at, created_at, updated_at
		FROM git_auth WHERE auth_id = $1`
	if err := d.readDB.GetContext(ctx, &a, q, authID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errcode.ErrNotFound.WithDetail("授权记录不存在")
		}
		return nil, errcode.ErrDatabase.WithDetail(err.Error())
	}
	return &a, nil
}

// ListAuths 查询租户授权列表（可选按平台筛选）
func (d *Data) ListAuths(ctx context.Context, tenantID, platform string) ([]*GitAuth, error) {
	var items []*GitAuth
	const q = `SELECT auth_id, tenant_id, platform, platform_user_id, platform_username,
		platform_avatar, access_token, refresh_token, token_expire_time,
		sync_status, last_synced_at, created_at, updated_at
		FROM git_auth
		WHERE tenant_id = $1 AND ($2 = '' OR platform = $2)
		ORDER BY created_at DESC`
	if err := d.readDB.SelectContext(ctx, &items, q, tenantID, platform); err != nil {
		return nil, errcode.ErrDatabase.WithDetail(err.Error())
	}
	return items, nil
}

// UpdateAuthToken Token 刷新后更新（定时任务，LLD §3.2-3）
func (d *Data) UpdateAuthToken(ctx context.Context, authID, encAccessToken, encRefreshToken string, expireAt time.Time) error {
	const q = `UPDATE git_auth SET access_token = $2, refresh_token = $3,
		token_expire_time = $4, updated_at = NOW()
		WHERE auth_id = $1`
	res, err := d.writeDB.ExecContext(ctx, q, authID, encAccessToken, encRefreshToken, expireAt)
	if err != nil {
		return errcode.ErrDatabase.WithDetail(err.Error())
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errcode.ErrNotFound.WithDetail("授权记录不存在")
	}
	return nil
}

// UpdateAuthSyncStatus 更新授权同步状态
func (d *Data) UpdateAuthSyncStatus(ctx context.Context, authID, status string) error {
	const q = `UPDATE git_auth SET sync_status = $2,
		last_synced_at = CASE WHEN $2 = 'synced' THEN NOW() ELSE last_synced_at END,
		updated_at = NOW()
		WHERE auth_id = $1`
	if _, err := d.writeDB.ExecContext(ctx, q, authID, status); err != nil {
		return errcode.ErrDatabase.WithDetail(err.Error())
	}
	return nil
}

// DeleteAuth 撤销授权（物理删除，关联仓库解绑由 service 层事务处理）
func (d *Data) DeleteAuth(ctx context.Context, authID string) error {
	const q = `DELETE FROM git_auth WHERE auth_id = $1`
	res, err := d.writeDB.ExecContext(ctx, q, authID)
	if err != nil {
		return errcode.ErrDatabase.WithDetail(err.Error())
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errcode.ErrNotFound.WithDetail("授权记录不存在")
	}
	return nil
}

// ListExpiringAuths 查询即将过期的授权（Token刷新定时任务用，提前1天）
func (d *Data) ListExpiringAuths(ctx context.Context, before time.Time) ([]*GitAuth, error) {
	var items []*GitAuth
	const q = `SELECT auth_id, tenant_id, platform, platform_user_id, platform_username,
		platform_avatar, access_token, refresh_token, token_expire_time,
		sync_status, last_synced_at, created_at, updated_at
		FROM git_auth
		WHERE token_expire_time < $1 AND refresh_token != ''`
	if err := d.readDB.SelectContext(ctx, &items, q, before); err != nil {
		return nil, errcode.ErrDatabase.WithDetail(err.Error())
	}
	return items, nil
}

// ==================== OAuth State 临时会话（CSRF防护，LLD §5.1-5） ====================

// SaveOAuthState 存储 OAuth state（10分钟过期）
func (d *Data) SaveOAuthState(ctx context.Context, state, tenantID, platform string) error {
	return d.rdb.Set(ctx, fmt.Sprintf(oauthStateKeyFmt, state),
		tenantID+":"+platform, oauthStateTTL).Err()
}

// ConsumeOAuthState 校验并消费 state（一次性，防重放）
func (d *Data) ConsumeOAuthState(ctx context.Context, state string) (tenantID, platform string, err error) {
	val, err := d.rdb.GetDel(ctx, fmt.Sprintf(oauthStateKeyFmt, state)).Result()
	if err != nil {
		return "", "", errcode.ErrGitAuthFailed.WithDetail("state已过期或不存在")
	}
	parts := strings.SplitN(val, ":", 2)
	if len(parts) != 2 {
		return "", "", errcode.ErrGitAuthFailed.WithDetail("state数据异常")
	}
	return parts[0], parts[1], nil
}
