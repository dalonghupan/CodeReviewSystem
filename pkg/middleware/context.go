// Package middleware 全局通用中间件：错误编码、JWT鉴权、请求日志
// 对应 LLD §1.3 / §7 异常处理策略 / §9 安全细则
package middleware

import "context"

// context 键定义（包内私有，通过 WithXxx/FromXxx 访问）
type userIDKey struct{}
type tenantIDKey struct{}
type userRolesKey struct{}

// WithUserID 将当前登录用户ID写入 context
func WithUserID(ctx context.Context, uid string) context.Context {
	return context.WithValue(ctx, userIDKey{}, uid)
}

// UserIDFromContext 获取当前登录用户ID（未登录返回空串）
func UserIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(userIDKey{}).(string); ok {
		return v
	}
	return ""
}

// WithTenantID 将当前租户ID写入 context
func WithTenantID(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, tenantIDKey{}, tenantID)
}

// TenantIDFromContext 获取当前租户ID
func TenantIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(tenantIDKey{}).(string); ok {
		return v
	}
	return ""
}

// WithUserRoles 将当前用户角色列表写入 context
func WithUserRoles(ctx context.Context, roles []string) context.Context {
	return context.WithValue(ctx, userRolesKey{}, roles)
}

// UserRolesFromContext 获取当前用户角色列表
func UserRolesFromContext(ctx context.Context) []string {
	if v, ok := ctx.Value(userRolesKey{}).([]string); ok {
		return v
	}
	return nil
}
