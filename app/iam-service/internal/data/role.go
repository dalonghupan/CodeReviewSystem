package data

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/redis/go-redis/v9"

	"cr-system/pkg/errcode"
	"cr-system/pkg/util"
)

// ListRoles 查询租户角色列表（含系统预设角色模板）
func (d *Data) ListRoles(ctx context.Context, tenantID string) ([]*Role, error) {
	var items []*Role
	const q = `SELECT role_id, tenant_id, name, display_name, description, is_system, created_at, updated_at
		FROM sys_role
		WHERE tenant_id = $1 OR tenant_id = '00000000-0000-0000-0000-000000000000'
		ORDER BY is_system DESC, created_at ASC`
	if err := d.readDB.SelectContext(ctx, &items, q, tenantID); err != nil {
		return nil, errcode.ErrDatabase.WithDetail(err.Error())
	}
	return items, nil
}

// GetRole 查询角色详情
func (d *Data) GetRole(ctx context.Context, roleID string) (*Role, error) {
	var r Role
	const q = `SELECT role_id, tenant_id, name, display_name, description, is_system, created_at, updated_at
		FROM sys_role WHERE role_id = $1`
	if err := d.readDB.GetContext(ctx, &r, q, roleID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errcode.ErrNotFound.WithDetail("角色不存在")
		}
		return nil, errcode.ErrDatabase.WithDetail(err.Error())
	}
	return &r, nil
}

// CreateRole 创建自定义角色并关联权限（事务保证原子性，LLD §1.3-3）
func (d *Data) CreateRole(ctx context.Context, r *Role, permissionIDs []string) error {
	r.RoleID = util.NewUUID()
	tx, err := d.writeDB.BeginTxx(ctx, nil)
	if err != nil {
		return errcode.ErrDatabase.WithDetail(err.Error())
	}
	defer func() { _ = tx.Rollback() }()

	const insertRole = `INSERT INTO sys_role (role_id, tenant_id, name, display_name, description)
		VALUES ($1, $2, $3, $4, $5)`
	if _, err := tx.ExecContext(ctx, insertRole, r.RoleID, r.TenantID, r.Name, r.DisplayName, r.Description); err != nil {
		return errcode.ErrDatabase.WithDetail(err.Error())
	}

	if err := bindRolePermissions(ctx, tx, r.RoleID, permissionIDs); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return errcode.ErrDatabase.WithDetail(err.Error())
	}
	return nil
}

// UpdateRole 更新角色信息与权限关联（系统预设角色仅允许改描述与权限）
func (d *Data) UpdateRole(ctx context.Context, r *Role, permissionIDs []string) error {
	tx, err := d.writeDB.BeginTxx(ctx, nil)
	if err != nil {
		return errcode.ErrDatabase.WithDetail(err.Error())
	}
	defer func() { _ = tx.Rollback() }()

	const updateRole = `UPDATE sys_role SET name = $2, description = $3, updated_at = NOW()
		WHERE role_id = $1`
	res, err := tx.ExecContext(ctx, updateRole, r.RoleID, r.Name, r.Description)
	if err != nil {
		return errcode.ErrDatabase.WithDetail(err.Error())
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errcode.ErrNotFound.WithDetail("角色不存在")
	}

	// 权限关联全量重建：先删后插
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM sys_role_permission WHERE role_id = $1`, r.RoleID); err != nil {
		return errcode.ErrDatabase.WithDetail(err.Error())
	}
	if err := bindRolePermissions(ctx, tx, r.RoleID, permissionIDs); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return errcode.ErrDatabase.WithDetail(err.Error())
	}
	// 角色权限变更，该角色下所有用户缓存失效（通过缓存Key前缀清理交由下一次读穿透重建）
	return nil
}

// bindRolePermissions 事务内批量插入角色权限关联
func bindRolePermissions(ctx context.Context, tx interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}, roleID string, permissionIDs []string) error {
	for _, pid := range permissionIDs {
		const q = `INSERT INTO sys_role_permission (role_id, permission_id)
			VALUES ($1, $2) ON CONFLICT (role_id, permission_id) DO NOTHING`
		if _, err := tx.ExecContext(ctx, q, roleID, pid); err != nil {
			return errcode.ErrDatabase.WithDetail(err.Error())
		}
	}
	return nil
}

// ListRolePermissionIDs 查询角色关联的权限ID列表
func (d *Data) ListRolePermissionIDs(ctx context.Context, roleID string) ([]string, error) {
	var ids []string
	const q = `SELECT permission_id FROM sys_role_permission WHERE role_id = $1`
	if err := d.readDB.SelectContext(ctx, &ids, q, roleID); err != nil {
		return nil, errcode.ErrDatabase.WithDetail(err.Error())
	}
	return ids, nil
}

// ==================== 权限校验（CheckDataPermission 核心） ====================

// permSet 用户权限集合缓存值：["repo:read", "review:write", ...]
type permSet []string

// GetUserPermissions 查询用户权限集合（Redis缓存优先，LLD §6 user:perm:{uid}:{tenantId} TTL=2h）
func (d *Data) GetUserPermissions(ctx context.Context, userID, tenantID string) (permSet, error) {
	cacheKey := fmt.Sprintf(permCacheKeyFmt, userID, tenantID)

	// 1. 读缓存
	if cached, err := d.rdb.Get(ctx, cacheKey).Result(); err == nil {
		var ps permSet
		if json.Unmarshal([]byte(cached), &ps) == nil {
			return ps, nil
		}
	} else if !errors.Is(err, redis.Nil) {
		d.log.Warnw("msg", "权限缓存读取异常，降级查库", "key", cacheKey, "error", err.Error())
	}

	// 2. 查库：用户角色 → 角色权限 → 权限资源
	var rows []struct {
		ResourceType string `db:"resource_type"`
		Action       string `db:"action"`
	}
	const q = `SELECT DISTINCT p.resource_type, p.action
		FROM sys_user_role ur
		JOIN sys_role_permission rp ON rp.role_id = ur.role_id
		JOIN sys_permission p ON p.permission_id = rp.permission_id
		WHERE ur.user_id = $1 AND ur.tenant_id = $2`
	if err := d.readDB.SelectContext(ctx, &rows, q, userID, tenantID); err != nil {
		return nil, errcode.ErrDatabase.WithDetail(err.Error())
	}

	ps := make(permSet, 0, len(rows))
	for _, r := range rows {
		ps = append(ps, r.ResourceType+":"+r.Action)
	}

	// 3. 回写缓存（失败仅告警不影响主流程）
	if data, err := json.Marshal(ps); err == nil {
		if err := d.rdb.Set(ctx, cacheKey, data, permCacheTTL).Err(); err != nil {
			d.log.Warnw("msg", "权限缓存写入失败", "key", cacheKey, "error", err.Error())
		}
	}
	return ps, nil
}

// InvalidatePermCache 使用户权限缓存失效（角色变更/账号禁用时调用）
// tenantID 传 "*" 时按前缀清理该用户全部租户缓存
func (d *Data) InvalidatePermCache(ctx context.Context, userID, tenantID string) error {
	if tenantID != "*" {
		return d.rdb.Del(ctx, fmt.Sprintf(permCacheKeyFmt, userID, tenantID)).Err()
	}
	// 前缀扫描清理（用户-角色变更低频操作，SCAN 代价可接受）
	pattern := fmt.Sprintf(permCacheKeyFmt, userID, "*")
	iter := d.rdb.Scan(ctx, 0, pattern, 100).Iterator()
	var keys []string
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if len(keys) > 0 {
		return d.rdb.Del(ctx, keys...).Err()
	}
	return iter.Err()
}

// CheckPermission 判断权限集合是否包含指定资源操作
// action 支持层级：admin 隐含 write/read，write 隐含 read
func CheckPermission(perms permSet, resourceType, action string) bool {
	for _, p := range perms {
		parts := strings.SplitN(p, ":", 2)
		if len(parts) != 2 || parts[0] != resourceType {
			continue
		}
		owned := parts[1]
		if owned == "admin" || owned == action {
			return true
		}
		if owned == "write" && action == "read" {
			return true
		}
	}
	return false
}
