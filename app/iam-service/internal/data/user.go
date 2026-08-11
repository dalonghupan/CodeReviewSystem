package data

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"cr-system/pkg/errcode"
	"cr-system/pkg/util"
)

// GetUser 查询用户详情
func (d *Data) GetUser(ctx context.Context, userID string) (*User, error) {
	var u User
	// email/phone 列可空，COALESCE 兜底避免 NULL 扫描失败
	const q = `SELECT user_id, tenant_id, username, display_name,
		COALESCE(email, '') AS email, COALESCE(phone, '') AS phone, COALESCE(avatar_url, '') AS avatar_url,
		is_active, last_login_at, created_at, updated_at
		FROM sys_user WHERE user_id = $1`
	if err := d.readDB.GetContext(ctx, &u, q, userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errcode.ErrNotFound.WithDetail("用户不存在")
		}
		return nil, errcode.ErrDatabase.WithDetail(err.Error())
	}
	return &u, nil
}

// GetUserByUsername 按租户+用户名查询（Keycloak同步 upsert 用）
func (d *Data) GetUserByUsername(ctx context.Context, tenantID, username string) (*User, error) {
	var u User
	const q = `SELECT user_id, tenant_id, username, display_name,
		COALESCE(email, '') AS email, COALESCE(phone, '') AS phone, COALESCE(avatar_url, '') AS avatar_url,
		is_active, last_login_at, created_at, updated_at
		FROM sys_user WHERE tenant_id = $1 AND username = $2`
	if err := d.readDB.GetContext(ctx, &u, q, tenantID, username); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // 不存在返回 nil,nil 便于同步逻辑判断
		}
		return nil, errcode.ErrDatabase.WithDetail(err.Error())
	}
	return &u, nil
}

// ListUsers 分页查询租户用户（支持关键词与角色筛选）
// 使用 PostgreSQL 空串短路写法，避免 SQL 字符串拼接，全部参数预编译
func (d *Data) ListUsers(ctx context.Context, tenantID, keyword, roleID string, page, pageSize uint32) ([]*User, uint32, error) {
	page, pageSize = util.NormalizePage(page, pageSize)

	// roleID 为空时 ur.role_id = $4 条件恒真（$4 = ''），即不筛选
	const filter = `
		FROM sys_user u
		LEFT JOIN sys_user_role ur ON ur.user_id = u.user_id AND ($4 = '' OR ur.role_id = $4)
		WHERE u.tenant_id = $1
		  AND (u.username ILIKE '%' || $2 || '%' OR u.display_name ILIKE '%' || $2 || '%')
		  AND ($4 = '' OR ur.role_id IS NOT NULL)`

	var total uint32
	if err := d.readDB.GetContext(ctx, &total,
		`SELECT COUNT(DISTINCT u.user_id) `+filter, tenantID, keyword, pageSize, roleID); err != nil {
		return nil, 0, errcode.ErrDatabase.WithDetail(err.Error())
	}

	var items []*User
	const listQ = `SELECT DISTINCT u.user_id, u.tenant_id, u.username, u.display_name,
		COALESCE(u.email, '') AS email, COALESCE(u.phone, '') AS phone, COALESCE(u.avatar_url, '') AS avatar_url,
		u.is_active, u.last_login_at, u.created_at, u.updated_at ` + filter + `
		ORDER BY u.created_at DESC LIMIT $3 OFFSET $5`
	if err := d.readDB.SelectContext(ctx, &items, listQ,
		tenantID, keyword, pageSize, roleID, util.PageOffset(page, pageSize)); err != nil {
		return nil, 0, errcode.ErrDatabase.WithDetail(err.Error())
	}
	return items, total, nil
}

// UpsertUser Keycloak 同步：存在则更新资料，不存在则新建
func (d *Data) UpsertUser(ctx context.Context, u *User) (isNew bool, err error) {
	exist, err := d.GetUserByUsername(ctx, u.TenantID, u.Username)
	if err != nil {
		return false, err
	}
	if exist == nil {
		u.UserID = util.NewUUID()
		const q = `INSERT INTO sys_user (user_id, tenant_id, username, display_name, email, avatar_url)
			VALUES ($1, $2, $3, $4, $5, $6)`
		if _, err := d.writeDB.ExecContext(ctx, q, u.UserID, u.TenantID, u.Username, u.DisplayName, u.Email, u.AvatarURL); err != nil {
			return false, errcode.ErrDatabase.WithDetail(err.Error())
		}
		return true, nil
	}
	u.UserID = exist.UserID
	const q = `UPDATE sys_user SET display_name = $2, email = $3, avatar_url = $4, updated_at = NOW()
		WHERE user_id = $1`
	if _, err := d.writeDB.ExecContext(ctx, q, u.UserID, u.DisplayName, u.Email, u.AvatarURL); err != nil {
		return false, errcode.ErrDatabase.WithDetail(err.Error())
	}
	return false, nil
}

// DisableUser 禁用账号（离职人员，LLD §3.1-4）
func (d *Data) DisableUser(ctx context.Context, userID string) error {
	const q = `UPDATE sys_user SET is_active = FALSE, updated_at = NOW() WHERE user_id = $1`
	if _, err := d.writeDB.ExecContext(ctx, q, userID); err != nil {
		return errcode.ErrDatabase.WithDetail(err.Error())
	}
	// 权限缓存同步失效
	_ = d.InvalidatePermCache(ctx, userID, "*")
	return nil
}

// ListUserRoleNames 查询用户角色名列表
func (d *Data) ListUserRoleNames(ctx context.Context, userID string) ([]string, error) {
	var names []string
	const q = `SELECT r.name FROM sys_user_role ur
		JOIN sys_role r ON r.role_id = ur.role_id
		WHERE ur.user_id = $1`
	if err := d.readDB.SelectContext(ctx, &names, q, userID); err != nil {
		return nil, errcode.ErrDatabase.WithDetail(err.Error())
	}
	return names, nil
}

// AssignRole 分配角色（幂等：唯一索引去重）
func (d *Data) AssignRole(ctx context.Context, userID, roleID, tenantID string) error {
	const q = `INSERT INTO sys_user_role (user_id, role_id, tenant_id)
		VALUES ($1, $2, $3) ON CONFLICT (user_id, role_id) DO NOTHING`
	if _, err := d.writeDB.ExecContext(ctx, q, userID, roleID, tenantID); err != nil {
		return errcode.ErrDatabase.WithDetail(err.Error())
	}
	_ = d.InvalidatePermCache(ctx, userID, tenantID)
	return nil
}

// RemoveRole 移除角色
func (d *Data) RemoveRole(ctx context.Context, userID, roleID string) error {
	const q = `DELETE FROM sys_user_role WHERE user_id = $1 AND role_id = $2`
	if _, err := d.writeDB.ExecContext(ctx, q, userID, roleID); err != nil {
		return errcode.ErrDatabase.WithDetail(err.Error())
	}
	_ = d.InvalidatePermCache(ctx, userID, "*")
	return nil
}

// UpdateLastLogin 更新最后登录时间
func (d *Data) UpdateLastLogin(ctx context.Context, userID string, at time.Time) error {
	const q = `UPDATE sys_user SET last_login_at = $2 WHERE user_id = $1`
	_, err := d.writeDB.ExecContext(ctx, q, userID, at)
	return err
}
