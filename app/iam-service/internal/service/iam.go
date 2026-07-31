// Package service iam-service 业务逻辑层（LLD §1.2-2）
// 实现 IAMService 全部 RPC：权限校验、租户/用户/角色管理、Keycloak同步
package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "cr-system/api/crsystem/v1"
	"cr-system/app/iam-service/internal/bizadapter"
	"cr-system/app/iam-service/internal/data"
	"cr-system/pkg/errcode"
	"cr-system/pkg/util"
)

// IAMService 业务服务
type IAMService struct {
	v1.UnimplementedIAMServiceServer

	data *data.Data
	kc   *bizadapter.KeycloakClient
	log  *log.Helper
}

// NewIAMService 构造服务
func NewIAMService(d *data.Data, kc *bizadapter.KeycloakClient, logger log.Logger) *IAMService {
	return &IAMService{
		data: d,
		kc:   kc,
		log:  log.NewHelper(logger),
	}
}

// ==================== 权限校验（gRPC内部调用，全服务依赖） ====================

// CheckDataPermission 数据权限校验（LLD §3.1-2）
// 校验链：租户启用 → 用户启用 → RBAC权限匹配（缓存优先）
func (s *IAMService) CheckDataPermission(ctx context.Context, req *v1.CheckDataPermissionReq) (*v1.CheckDataPermissionResp, error) {
	if req.TenantId == "" || req.UserId == "" || req.ResourceType == "" || req.Action == "" {
		return nil, errcode.ErrParamInvalid
	}

	// 1. 租户状态
	active, err := s.data.IsTenantActive(ctx, req.TenantId)
	if err != nil {
		return nil, err
	}
	if !active {
		return &v1.CheckDataPermissionResp{Allowed: false, DenyReason: "租户已禁用"}, nil
	}

	// 2. 用户状态
	user, err := s.data.GetUser(ctx, req.UserId)
	if err != nil {
		return nil, err
	}
	if !user.IsActive {
		return &v1.CheckDataPermissionResp{Allowed: false, DenyReason: "账号已禁用"}, nil
	}
	if user.TenantID != req.TenantId {
		return &v1.CheckDataPermissionResp{Allowed: false, DenyReason: "跨租户访问被禁止"}, nil
	}

	// 3. RBAC 权限匹配（Redis缓存，TTL 2h）
	perms, err := s.data.GetUserPermissions(ctx, req.UserId, req.TenantId)
	if err != nil {
		return nil, err
	}
	if !data.CheckPermission(perms, req.ResourceType, req.Action) {
		return &v1.CheckDataPermissionResp{
			Allowed:    false,
			DenyReason: "缺少 " + req.ResourceType + ":" + req.Action + " 权限",
		}, nil
	}

	return &v1.CheckDataPermissionResp{Allowed: true}, nil
}

// ==================== 租户管理 ====================

// CreateTenant 创建租户
func (s *IAMService) CreateTenant(ctx context.Context, req *v1.CreateTenantReq) (*v1.TenantInfo, error) {
	if req.Name == "" {
		return nil, errcode.ErrParamInvalid.WithDetail("租户名称不能为空")
	}
	t := &data.Tenant{
		Name:         req.Name,
		Description:  req.Description,
		ContactEmail: req.ContactEmail,
	}
	if err := s.data.CreateTenant(ctx, t); err != nil {
		return nil, err
	}
	s.log.Infow("msg", "租户创建成功", "tenant_id", t.TenantID, "name", t.Name)
	return s.GetTenant(ctx, &v1.GetTenantReq{TenantId: t.TenantID})
}

// GetTenant 租户详情
func (s *IAMService) GetTenant(ctx context.Context, req *v1.GetTenantReq) (*v1.TenantInfo, error) {
	t, err := s.data.GetTenant(ctx, req.TenantId)
	if err != nil {
		return nil, err
	}
	return tenantToProto(t), nil
}

// ListTenants 租户分页列表
func (s *IAMService) ListTenants(ctx context.Context, req *v1.ListTenantsReq) (*v1.ListTenantsResp, error) {
	page, pageSize := paginationOf(req.Pagination)
	items, total, err := s.data.ListTenants(ctx, req.Keyword, page, pageSize)
	if err != nil {
		return nil, err
	}
	resp := &v1.ListTenantsResp{
		Pagination: paginationResp(page, pageSize, total),
	}
	for _, t := range items {
		resp.Items = append(resp.Items, tenantToProto(t))
	}
	return resp, nil
}

// UpdateTenant 更新租户
func (s *IAMService) UpdateTenant(ctx context.Context, req *v1.UpdateTenantReq) (*v1.TenantInfo, error) {
	t := &data.Tenant{
		TenantID:     req.TenantId,
		Name:         req.Name,
		Description:  req.Description,
		ContactEmail: req.ContactEmail,
		IsActive:     req.IsActive,
	}
	if err := s.data.UpdateTenant(ctx, t); err != nil {
		return nil, err
	}
	return s.GetTenant(ctx, &v1.GetTenantReq{TenantId: req.TenantId})
}

// ==================== 用户管理 ====================

// GetUser 用户详情（含角色名列表）
func (s *IAMService) GetUser(ctx context.Context, req *v1.GetUserReq) (*v1.UserInfo, error) {
	u, err := s.data.GetUser(ctx, req.UserId)
	if err != nil {
		return nil, err
	}
	roles, err := s.data.ListUserRoleNames(ctx, u.UserID)
	if err != nil {
		return nil, err
	}
	return userToProto(u, roles), nil
}

// ListUsers 租户用户分页列表
func (s *IAMService) ListUsers(ctx context.Context, req *v1.ListUsersReq) (*v1.ListUsersResp, error) {
	page, pageSize := paginationOf(req.Pagination)
	items, total, err := s.data.ListUsers(ctx, req.TenantId, req.Keyword, req.RoleId, page, pageSize)
	if err != nil {
		return nil, err
	}
	resp := &v1.ListUsersResp{
		Pagination: paginationResp(page, pageSize, total),
	}
	for _, u := range items {
		roles, err := s.data.ListUserRoleNames(ctx, u.UserID)
		if err != nil {
			return nil, err
		}
		resp.Items = append(resp.Items, userToProto(u, roles))
	}
	return resp, nil
}

// AssignRole 分配角色
func (s *IAMService) AssignRole(ctx context.Context, req *v1.AssignRoleReq) (*v1.OperateResult, error) {
	if req.UserId == "" || req.RoleId == "" || req.TenantId == "" {
		return nil, errcode.ErrParamInvalid
	}
	if _, err := s.data.GetRole(ctx, req.RoleId); err != nil {
		return nil, err
	}
	if err := s.data.AssignRole(ctx, req.UserId, req.RoleId, req.TenantId); err != nil {
		return nil, err
	}
	return &v1.OperateResult{Success: true, Message: "角色分配成功"}, nil
}

// RemoveRole 移除角色
func (s *IAMService) RemoveRole(ctx context.Context, req *v1.RemoveRoleReq) (*v1.OperateResult, error) {
	if err := s.data.RemoveRole(ctx, req.UserId, req.RoleId); err != nil {
		return nil, err
	}
	return &v1.OperateResult{Success: true, Message: "角色已移除"}, nil
}

// ==================== 角色管理 ====================

// ListRoles 角色列表
func (s *IAMService) ListRoles(ctx context.Context, req *v1.ListRolesReq) (*v1.ListRolesResp, error) {
	items, err := s.data.ListRoles(ctx, req.TenantId)
	if err != nil {
		return nil, err
	}
	resp := &v1.ListRolesResp{}
	for _, r := range items {
		permIDs, err := s.data.ListRolePermissionIDs(ctx, r.RoleID)
		if err != nil {
			return nil, err
		}
		resp.Items = append(resp.Items, roleToProto(r, permIDs))
	}
	return resp, nil
}

// CreateRole 创建自定义角色
func (s *IAMService) CreateRole(ctx context.Context, req *v1.CreateRoleReq) (*v1.RoleInfo, error) {
	if req.TenantId == "" || req.Name == "" {
		return nil, errcode.ErrParamInvalid.WithDetail("租户ID与角色名不能为空")
	}
	r := &data.Role{
		TenantID:    req.TenantId,
		Name:        req.Name,
		DisplayName: req.Name,
		Description: req.Description,
	}
	if err := s.data.CreateRole(ctx, r, req.PermissionIds); err != nil {
		return nil, err
	}
	return roleToProto(r, req.PermissionIds), nil
}

// UpdateRole 更新角色
func (s *IAMService) UpdateRole(ctx context.Context, req *v1.UpdateRoleReq) (*v1.RoleInfo, error) {
	r := &data.Role{
		RoleID:      req.RoleId,
		Name:        req.Name,
		Description: req.Description,
	}
	if err := s.data.UpdateRole(ctx, r, req.PermissionIds); err != nil {
		return nil, err
	}
	return roleToProto(r, req.PermissionIds), nil
}

// ==================== Keycloak 同步 ====================

// SyncUsersFromKeycloak 手动触发账号同步（LLD §3.1-4：新增账号入库、离职账号禁用）
func (s *IAMService) SyncUsersFromKeycloak(ctx context.Context, req *v1.SyncUsersReq) (*v1.SyncUsersResp, error) {
	kcUsers, err := s.kc.ListUsers(ctx)
	if err != nil {
		return nil, err
	}

	tenantID := req.TenantId
	if tenantID == "" {
		return nil, errcode.ErrParamInvalid.WithDetail("当前版本需指定租户ID执行同步")
	}

	var synced uint32
	for _, ku := range kcUsers {
		u := &data.User{
			TenantID:    tenantID,
			Username:    ku.Username,
			DisplayName: ku.DisplayName(),
			Email:       ku.Email,
		}
		isNew, err := s.data.UpsertUser(ctx, u)
		if err != nil {
			s.log.Errorw("msg", "同步用户失败", "username", ku.Username, "error", err.Error())
			continue // 单用户失败不中断整体同步
		}
		if isNew {
			synced++
		}
		// Keycloak 端已禁用的账号同步禁用本系统权限（离职人员，LLD §3.1-4）
		if !ku.Enabled {
			if exist, _ := s.data.GetUserByUsername(ctx, tenantID, ku.Username); exist != nil && exist.IsActive {
				_ = s.data.DisableUser(ctx, exist.UserID)
			}
		}
	}

	s.log.Infow("msg", "Keycloak同步完成", "tenant_id", tenantID, "synced", synced, "total", len(kcUsers))
	return &v1.SyncUsersResp{
		SyncedCount: synced,
		Message:     "同步完成",
	}, nil
}

// ==================== 转换辅助 ====================

func tenantToProto(t *data.Tenant) *v1.TenantInfo {
	return &v1.TenantInfo{
		TenantId:     t.TenantID,
		Name:         t.Name,
		Description:  t.Description,
		ContactEmail: t.ContactEmail,
		IsActive:     t.IsActive,
		CreatedAt:    timestamppb.New(t.CreatedAt),
		UpdatedAt:    timestamppb.New(t.UpdatedAt),
	}
}

func userToProto(u *data.User, roles []string) *v1.UserInfo {
	info := &v1.UserInfo{
		UserId:      u.UserID,
		TenantId:    u.TenantID,
		Username:    u.Username,
		DisplayName: u.DisplayName,
		Email:       u.Email,
		AvatarUrl:   u.AvatarURL,
		RoleNames:   roles,
		IsActive:    u.IsActive,
		CreatedAt:   timestamppb.New(u.CreatedAt),
	}
	if u.LastLoginAt != nil {
		info.LastLoginAt = timestamppb.New(*u.LastLoginAt)
	}
	return info
}

func roleToProto(r *data.Role, permIDs []string) *v1.RoleInfo {
	return &v1.RoleInfo{
		RoleId:        r.RoleID,
		TenantId:      r.TenantID,
		Name:          r.Name,
		Description:   r.Description,
		PermissionIds: permIDs,
		IsSystem:      r.IsSystem,
		CreatedAt:     timestamppb.New(r.CreatedAt),
	}
}

func paginationOf(p *v1.Pagination) (uint32, uint32) {
	if p == nil {
		return util.NormalizePage(0, 0)
	}
	return util.NormalizePage(p.Page, p.PageSize)
}

func paginationResp(page, pageSize, total uint32) *v1.PaginationResp {
	return &v1.PaginationResp{
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: util.TotalPages(total, pageSize),
	}
}

// 确保实现接口（编译期检查）
var _ v1.IAMServiceServer = (*IAMService)(nil)
