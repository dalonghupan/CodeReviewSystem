package data

import (
	"context"
	"database/sql"
	"errors"

	"cr-system/pkg/errcode"
	"cr-system/pkg/util"
)

// CreateTenant 创建租户
func (d *Data) CreateTenant(ctx context.Context, t *Tenant) error {
	t.TenantID = util.NewUUID()
	const q = `INSERT INTO tenant (tenant_id, name, description, contact_email)
		VALUES ($1, $2, $3, $4)`
	if _, err := d.writeDB.ExecContext(ctx, q, t.TenantID, t.Name, t.Description, t.ContactEmail); err != nil {
		return errcode.ErrDatabase.WithDetail(err.Error())
	}
	return nil
}

// GetTenant 查询租户详情
func (d *Data) GetTenant(ctx context.Context, tenantID string) (*Tenant, error) {
	var t Tenant
	// contact_email 列可空，COALESCE 兜底避免 NULL 扫描失败
	const q = `SELECT tenant_id, name, COALESCE(description, '') AS description,
		COALESCE(contact_email, '') AS contact_email, is_active, created_at, updated_at
		FROM tenant WHERE tenant_id = $1`
	if err := d.readDB.GetContext(ctx, &t, q, tenantID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errcode.ErrTenantNotFound
		}
		return nil, errcode.ErrDatabase.WithDetail(err.Error())
	}
	return &t, nil
}

// ListTenants 分页查询租户列表（支持名称模糊搜索，预编译防注入）
func (d *Data) ListTenants(ctx context.Context, keyword string, page, pageSize uint32) ([]*Tenant, uint32, error) {
	page, pageSize = util.NormalizePage(page, pageSize)

	var total uint32
	const countQ = `SELECT COUNT(*) FROM tenant WHERE name ILIKE '%' || $1 || '%'`
	if err := d.readDB.GetContext(ctx, &total, countQ, keyword); err != nil {
		return nil, 0, errcode.ErrDatabase.WithDetail(err.Error())
	}

	var items []*Tenant
	const q = `SELECT tenant_id, name, COALESCE(description, '') AS description,
		COALESCE(contact_email, '') AS contact_email, is_active, created_at, updated_at
		FROM tenant WHERE name ILIKE '%' || $1 || '%'
		ORDER BY created_at DESC LIMIT $2 OFFSET $3`
	if err := d.readDB.SelectContext(ctx, &items, q, keyword, pageSize, util.PageOffset(page, pageSize)); err != nil {
		return nil, 0, errcode.ErrDatabase.WithDetail(err.Error())
	}
	return items, total, nil
}

// UpdateTenant 更新租户信息（名称/描述/邮箱/启停状态）
func (d *Data) UpdateTenant(ctx context.Context, t *Tenant) error {
	const q = `UPDATE tenant SET name = $2, description = $3, contact_email = $4,
		is_active = $5, updated_at = NOW()
		WHERE tenant_id = $1`
	res, err := d.writeDB.ExecContext(ctx, q, t.TenantID, t.Name, t.Description, t.ContactEmail, t.IsActive)
	if err != nil {
		return errcode.ErrDatabase.WithDetail(err.Error())
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errcode.ErrTenantNotFound
	}
	return nil
}

// IsTenantActive 校验租户存在且启用（权限校验前置）
func (d *Data) IsTenantActive(ctx context.Context, tenantID string) (bool, error) {
	var active bool
	const q = `SELECT is_active FROM tenant WHERE tenant_id = $1`
	if err := d.readDB.GetContext(ctx, &active, q, tenantID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, errcode.ErrTenantNotFound
		}
		return false, errcode.ErrDatabase.WithDetail(err.Error())
	}
	return active, nil
}
